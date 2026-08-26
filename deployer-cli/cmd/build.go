package cmd

import (
	"fmt"
	"os"

	"deployer-cli/pkg/config"
	"deployer-cli/pkg/gcp"
	"deployer-cli/pkg/scenario"
	"deployer-cli/pkg/ui"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build and push a scenario container image using Cloud Build",
	Long:  `Submits a build job to Google Cloud Build and pushes the custom code-server container image to Artifact Registry without modifying Terraform state.`,
	Run:   runBuild,
}

func init() {
	rootCmd.AddCommand(buildCmd)
}

func runBuild(cmd *cobra.Command, args []string) {
	ui.Banner()

	repoRoot, err := scenario.FindRepoRoot()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to find repository root: %v", err))
		os.Exit(1)
	}

	selectedScenario, err := scenario.ResolveScenario(repoRoot, flagScenario, "Select Demo Scenario to build")
	if err != nil {
		ui.Error(fmt.Sprintf("Scenario selection error: %v", err))
		os.Exit(1)
	}

	cfg, err := config.ResolveDeploymentConfig(selectedScenario, flagProject, flagRegion, flagIAPMember, true)
	if err != nil {
		ui.Error(fmt.Sprintf("Configuration resolution error: %v", err))
		os.Exit(1)
	}

	ui.SectionHeader(fmt.Sprintf("Building Container Image for %s", selectedScenario.Name))
	ui.Detail("Scenario", selectedScenario.Name)
	ui.Detail("Configuration Fingerprint", cfg.ImageTag)
	ui.Detail("Target Image", cfg.FullImageTag)
	ui.Detail("Latest Tag", cfg.FullLatestTag)

	if gcp.ImageExists(cfg.FullImageTag) {
		ui.Warning(fmt.Sprintf("Container image with fingerprint '%s' already exists in Artifact Registry.", cfg.ImageTag))
		if !ui.PromptConfirm("Do you want to rebuild and overwrite this container image?", false) {
			ui.Info("Build cancelled.")
			os.Exit(0)
		}
	} else {
		if !ui.PromptConfirm("Submit container build to Cloud Build?", true) {
			ui.Info("Build cancelled.")
			os.Exit(0)
		}
	}

	if err := gcp.EnsurePrerequisites(cfg.ProjectID, cfg.Region, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("Prerequisites setup failed: %v", err))
		os.Exit(1)
	}

	if err := gcp.SubmitCloudBuild(cfg.ProjectID, cfg.Region, selectedScenario.Dir, cfg.FullImageTag, cfg.FullLatestTag, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("Container build failed: %v", err))
		os.Exit(1)
	}

	ui.Success(fmt.Sprintf("Image successfully built and pushed with tags: %s and latest", cfg.ImageTag))
}
