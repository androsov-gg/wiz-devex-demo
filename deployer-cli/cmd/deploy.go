package cmd

import (
	"fmt"
	"os"
	"time"

	"deployer-cli/pkg/config"
	"deployer-cli/pkg/doctor"
	"deployer-cli/pkg/gcp"
	"deployer-cli/pkg/scenario"
	"deployer-cli/pkg/terraform"
	"deployer-cli/pkg/ui"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Interactively deploy a demo scenario to Google Cloud Run",
	Long:  `Guides Customer Engineers through prerequisite checks, container image building, and Terraform provisioning with full Identity-Aware Proxy (IAP) access.`,
	Run:   runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) {
	ui.Banner()

	repoRoot, err := scenario.FindRepoRoot()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to find repository root: %v", err))
		os.Exit(1)
	}

	// 1. Discover & Select Scenario
	selectedScenario, err := scenario.ResolveScenario(repoRoot, flagScenario, "Select Demo Scenario to deploy")
	if err != nil {
		ui.Error(fmt.Sprintf("Scenario selection error: %v", err))
		os.Exit(1)
	}

	// 2. Resolve Target Project for Pre-Flight Diagnostics
	targetProject := flagProject
	if targetProject == "" {
		pID, _, _, _, _ := config.ReadExistingTFVars(selectedScenario.TerraformDir)
		targetProject = pID
	}
	if targetProject == "" {
		p, _, _ := config.DetectGCPDefaults()
		targetProject = p
	}

	// 3. Doctor pre-flight check
	results, allPassed := doctor.RunDiagnostics(targetProject)
	doctor.PrintDiagnostics(results)
	if !allPassed {
		if !ui.PromptConfirm("Some pre-flight checks had warnings or failures. Do you still want to proceed?", false) {
			ui.Info("Deployment aborted. Please fix the items above and try again.")
			os.Exit(1)
		}
	}

	// 4. Resolve & Confirm Deployment Configuration
	cfg, err := config.ResolveDeploymentConfig(
		selectedScenario,
		flagProject,
		flagRegion,
		flagIAPMember,
		flagEnableScheduler,
		flagScheduleTimezone,
		flagScaleDownSched,
		flagScaleUpSched,
		true,
	)
	if err != nil {
		ui.Error(fmt.Sprintf("Configuration resolution error: %v", err))
		os.Exit(1)
	}

	ui.SectionHeader("Deployment Configuration Review")
	ui.Detail("Scenario", cfg.Scenario.Name)
	ui.Detail("GCP Project", cfg.ProjectID)
	ui.Detail("GCP Region", cfg.Region)
	ui.Detail("IAP Member", cfg.IAPMembers[0])
	ui.Detail("GCS State Bucket", cfg.StateBucket)
	if cfg.Scenario.StatePrefix != "" {
		ui.Detail("State Prefix", cfg.Scenario.StatePrefix)
	} else {
		ui.Detail("State Prefix", "(root state / default.tfstate)")
	}
	ui.Detail("Configuration Fingerprint", cfg.ImageTag)
	ui.Detail("Target Image", cfg.FullImageTag)
	if cfg.EnableScheduler {
		ui.Detail("Off-Hours Scaling", fmt.Sprintf("Enabled (Scale-down: %s, Scale-up: %s, TZ: %s)", cfg.ScaleDownSchedule, cfg.ScaleUpSchedule, cfg.ScheduleTimezone))
	} else {
		ui.Detail("Off-Hours Scaling", "Disabled (Always 1 min instance)")
	}

	if !ui.PromptConfirm("Proceed with deployment?", true) {
		ui.Info("Deployment cancelled by user.")
		os.Exit(0)
	}

	if err := config.WriteTFVars(selectedScenario.TerraformDir, cfg); err != nil {
		ui.Error(fmt.Sprintf("Failed to write terraform.auto.tfvars: %v", err))
		os.Exit(1)
	}
	ui.Success("Configured terraform.auto.tfvars")

	// 4. GCP Automation Pipeline (4 concise steps)
	totalSteps := 4

	// Step 1: Base GCP prerequisites (APIs, AR repo, Builder SA)
	ui.StepHeader(1, totalSteps, "Configuring Google Cloud Project Prerequisites")
	if err := gcp.EnsurePrerequisites(cfg.ProjectID, cfg.Region, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("Prerequisites setup failed: %v", err))
		os.Exit(1)
	}
	ui.Success("Google Cloud APIs, Artifact Registry, and Builder SA ready")

	// Step 2: Container Image (Content-Hash Fingerprint Check)
	ui.StepHeader(2, totalSteps, "Checking Container Image Fingerprint in Artifact Registry")
	needBuild := true
	if gcp.ImageExists(cfg.FullImageTag) {
		ui.Success(fmt.Sprintf("Image '%s:%s' matches current configuration fingerprint.", selectedScenario.Name, cfg.ImageTag))
		ui.Info("No configuration changes detected — skipping container build!")
		needBuild = false
	} else {
		ui.Info(fmt.Sprintf("Configuration change detected (fingerprint: %s). Rebuilding container image...", cfg.ImageTag))
	}

	if needBuild {
		if err := gcp.SubmitCloudBuild(cfg.ProjectID, cfg.Region, selectedScenario.Dir, cfg.FullImageTag, cfg.FullLatestTag, flagVerbose); err != nil {
			ui.Error(fmt.Sprintf("Cloud Build container build failed: %v", err))
			os.Exit(1)
		}
		ui.Success("Container image built and pushed successfully")
	}

	// Step 3: State Bucket
	ui.StepHeader(3, totalSteps, "Preparing Terraform Remote State Bucket")
	if err := gcp.EnsureStateBucket(cfg.ProjectID, cfg.Region, cfg.StateBucket, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("State bucket creation failed: %v", err))
		os.Exit(1)
	}
	ui.Success(fmt.Sprintf("Terraform state bucket 'gs://%s' ready", cfg.StateBucket))

	// Step 4: Terraform Init & Apply
	ui.StepHeader(4, totalSteps, "Initializing & Applying Terraform Infrastructure")
	if err := terraform.Init(selectedScenario.TerraformDir, cfg.StateBucket, selectedScenario.StatePrefix, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("Terraform initialization failed: %v", err))
		os.Exit(1)
	}
	ui.Success("Terraform initialized")

	planSummary, err := terraform.Plan(selectedScenario.TerraformDir, flagVerbose)
	if err != nil {
		ui.Error(fmt.Sprintf("Terraform plan failed: %v", err))
		os.Exit(1)
	}
	ui.Info(fmt.Sprintf("Terraform Plan Summary: %s%s%s", ui.Bold, planSummary, ui.Reset))

	if !ui.PromptConfirm("Apply this Terraform configuration to Google Cloud?", true) {
		ui.Info("Terraform apply cancelled. Infrastructure was not modified.")
		os.Exit(0)
	}

	if err := terraform.Apply(selectedScenario.TerraformDir, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("Terraform apply failed: %v", err))
		os.Exit(1)
	}

	// 5. Outputs & Readiness Verification
	outputs, err := terraform.GetOutputs(selectedScenario.TerraformDir)
	serviceURL := "https://console.cloud.google.com/run"
	if err == nil && outputs["cloud_run_service_url"] != "" {
		serviceURL = outputs["cloud_run_service_url"]
	}

	ui.Info("Verifying Cloud Run service instance health...")
	if gcp.WaitForServiceReady(cfg.ProjectID, cfg.Region, selectedScenario.Name, 45*time.Second) {
		ui.Success("Cloud Run service is healthy and ready to serve traffic")
	} else {
		ui.Warning("Cloud Run service is still starting up or taking longer than expected to report Ready.")
		ui.Info("The service will become available shortly. Check Google Cloud Run console if issues persist.")
	}

	ui.PrintDeploymentSummary(serviceURL, cfg.ProjectID, cfg.Region, selectedScenario.Name, cfg.FullImageTag)
	ui.PrintModelGardenInstructions(cfg.ProjectID)
}
