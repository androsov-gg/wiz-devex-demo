package cmd

import (
	"fmt"
	"os"

	"deployer-cli/pkg/config"
	"deployer-cli/pkg/scenario"
	"deployer-cli/pkg/ui"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or interactively edit deployment configuration for a scenario",
	Long:  `Inspects or modifies the terraform.auto.tfvars configuration for any demo scenario.`,
	Run:   runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) {
	ui.Banner()

	repoRoot, err := scenario.FindRepoRoot()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to find repository root: %v", err))
		os.Exit(1)
	}

	selectedScenario, err := scenario.ResolveScenario(repoRoot, flagScenario, "Select Demo Scenario to configure")
	if err != nil {
		ui.Error(fmt.Sprintf("Scenario selection error: %v", err))
		os.Exit(1)
	}

	existingProject, existingRegion, existingService, existingImageTag, existingIAP := config.ReadExistingTFVars(selectedScenario.TerraformDir)

	ui.SectionHeader(fmt.Sprintf("Current Configuration for %s", selectedScenario.Name))
	ui.Detail("Terraform Directory", selectedScenario.TerraformDir)
	if existingProject != "" {
		ui.Detail("Project ID", existingProject)
		ui.Detail("Region", existingRegion)
		ui.Detail("Service Name", existingService)
		if existingImageTag != "" {
			ui.Detail("Image Tag", existingImageTag)
		}
		if len(existingIAP) > 0 {
			ui.Detail("IAP Members", existingIAP[0])
		}
	} else {
		ui.Info("No existing terraform.auto.tfvars file found.")
	}

	if !ui.PromptConfirm("Do you want to edit this configuration?", false) {
		return
	}

	cfg, err := config.ResolveDeploymentConfig(selectedScenario, flagProject, flagRegion, flagIAPMember, true)
	if err != nil {
		ui.Error(fmt.Sprintf("Configuration resolution error: %v", err))
		os.Exit(1)
	}

	if err := config.WriteTFVars(selectedScenario.TerraformDir, cfg); err != nil {
		ui.Error(fmt.Sprintf("Failed to write config: %v", err))
		os.Exit(1)
	}

	ui.Success(fmt.Sprintf("Updated %s/terraform.auto.tfvars successfully.", selectedScenario.TerraformDir))
}
