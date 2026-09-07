package cmd

import (
	"deployer-cli/pkg/config"
	"deployer-cli/pkg/doctor"
	"deployer-cli/pkg/scenario"
	"deployer-cli/pkg/ui"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check deployment prerequisites and GCP environment health",
	Long:  `Validates local dependencies (gcloud, terraform, git), Google Cloud authentication, project permissions, and displays Agent Platform Model Garden checklist.`,
	Run:   runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) {
	ui.Banner()

	defaultProject := ""
	if repoRoot, err := scenario.FindRepoRoot(); err == nil {
		if scenarios, err := scenario.DiscoverScenarios(repoRoot); err == nil && len(scenarios) > 0 {
			pID, _, _, _, _ := config.ReadExistingTFVars(scenarios[0].TerraformDir)
			defaultProject = pID
		}
	}

	if defaultProject == "" {
		p, _, _ := config.DetectGCPDefaults()
		defaultProject = p
	}

	project := flagProject
	if project == "" {
		project = ui.PromptString("Google Cloud Project ID (Argolis)", defaultProject)
	}

	results, allPassed := doctor.RunDiagnostics(project)
	doctor.PrintDiagnostics(results)

	if allPassed {
		ui.Success("All core pre-flight checks passed!")
	} else {
		ui.Warning("Some checks require your attention before deploying.")
	}

	ui.PrintModelGardenInstructions(project)
}
