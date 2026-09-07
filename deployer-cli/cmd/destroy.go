package cmd

import (
	"fmt"
	"os"

	"deployer-cli/pkg/config"
	"deployer-cli/pkg/gcp"
	"deployer-cli/pkg/scenario"
	"deployer-cli/pkg/terraform"
	"deployer-cli/pkg/ui"

	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Interactively destroy a deployed demo scenario",
	Long:  `Tears down the Cloud Run service, IAM policy bindings, and optionally cleans up the Artifact Registry repository or builder service account.`,
	Run:   runDestroy,
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

func runDestroy(cmd *cobra.Command, args []string) {
	ui.Banner()

	repoRoot, err := scenario.FindRepoRoot()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to find repository root: %v", err))
		os.Exit(1)
	}

	selectedScenario, err := scenario.ResolveScenario(repoRoot, flagScenario, "Select Demo Scenario to destroy")
	if err != nil {
		ui.Error(fmt.Sprintf("Scenario selection error: %v", err))
		os.Exit(1)
	}

	existingProject, existingRegion, _, _, _ := config.ReadExistingTFVars(selectedScenario.TerraformDir)
	project := flagProject
	if project == "" {
		project = existingProject
	}
	if project == "" {
		p, _, _ := config.DetectGCPDefaults()
		project = p
	}

	ui.SectionHeader(fmt.Sprintf("Teardown Scenario: %s", selectedScenario.Name))
	ui.Warning(fmt.Sprintf("You are about to destroy Cloud Run infrastructure for '%s' in project '%s'.", selectedScenario.Name, project))

	if !ui.PromptConfirm(fmt.Sprintf("Are you sure you want to destroy %s?", selectedScenario.Name), false) {
		ui.Info("Teardown cancelled.")
		os.Exit(0)
	}

	// Ensure Terraform backend is initialized with remote GCS state bucket before destroy
	stateBucket := gcp.ResolveStateBucket(project)
	if err := terraform.Init(selectedScenario.TerraformDir, stateBucket, selectedScenario.StatePrefix, flagVerbose); err != nil {
		ui.Warning(fmt.Sprintf("Terraform initialization notice: %v", err))
	}

	// Run Terraform Destroy
	if err := terraform.Destroy(selectedScenario.TerraformDir, flagVerbose); err != nil {
		ui.Error(fmt.Sprintf("Terraform destroy failed: %v", err))
		os.Exit(1)
	}
	ui.Success(fmt.Sprintf("Scenario '%s' infrastructure successfully destroyed.", selectedScenario.Name))

	// Optional cleanup of pre-requisites
	region := flagRegion
	if region == "" {
		region = existingRegion
	}
	if region == "" {
		region = "europe-west1"
	}

	if project != "" {
		if ui.PromptConfirm("Do you also want to delete the Cloud Build builder service account?", false) {
			if err := gcp.DeleteBuilderServiceAccount(project); err != nil {
				ui.Warning(fmt.Sprintf("Could not delete service account: %v", err))
			} else {
				ui.Success("Builder Service Account deleted.")
			}
		}

		if ui.PromptConfirm("Do you also want to delete the Artifact Registry container images repository?", false) {
			if err := gcp.DeleteArtifactRegistry(project, region, "wiz-devex-demo-images"); err != nil {
				ui.Warning(fmt.Sprintf("Could not delete Artifact Registry repository: %v", err))
			} else {
				ui.Success("Artifact Registry repository deleted.")
			}
		}
	}
}
