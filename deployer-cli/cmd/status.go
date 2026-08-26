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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status and access URLs of deployed demo services",
	Long:  `Queries Google Cloud Run and local Terraform states to display live status, endpoints, and deployment details.`,
	Run:   runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) {
	ui.Banner()

	repoRoot, err := scenario.FindRepoRoot()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to find repository root: %v", err))
		os.Exit(1)
	}

	scenarios, err := scenario.DiscoverScenarios(repoRoot)
	if err != nil {
		ui.Error(fmt.Sprintf("Scenario discovery error: %v", err))
		os.Exit(1)
	}

	defaultProject := ""
	defaultRegion := "europe-west1"
	if len(scenarios) > 0 {
		pID, reg, _, _, _ := config.ReadExistingTFVars(scenarios[0].TerraformDir)
		defaultProject = pID
		if reg != "" {
			defaultRegion = reg
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

	region := flagRegion
	if region == "" || region == "europe-west1" {
		region = defaultRegion
	}

	ui.SectionHeader(fmt.Sprintf("Demo Platform Status (%s)", project))

	// Check local scenario terraform states
	for _, s := range scenarios {
		pID, reg, sName, imgTag, iap := config.ReadExistingTFVars(s.TerraformDir)
		if pID == "" {
			pID = project
		}
		if reg == "" {
			reg = region
		}
		if sName == "" {
			sName = s.Name
		}

		fmt.Printf("\n%s%sScenario: %s%s\n", ui.Cyan, ui.Bold, s.Name, ui.Reset)
		ui.Detail("Terraform Dir", s.TerraformDir)
		if len(iap) > 0 {
			ui.Detail("IAP Access", iap[0])
		}
		if imgTag != "" {
			ui.Detail("Config Fingerprint", imgTag)
		}

		outputs, err := terraform.GetOutputs(s.TerraformDir)
		if err == nil && len(outputs) > 0 {
			if url, ok := outputs["cloud_run_service_url"]; ok {
				ui.Detail("Service URL", url)
			}
			if img, ok := outputs["custom_container_image_tag"]; ok {
				ui.Detail("Container Image", img)
			}
			ui.Success("State: Provisioned via Terraform")
		} else {
			ui.Info("State: Not yet deployed locally or uninitialized")
		}
	}

	// Also check live Cloud Run services if project is specified
	if project != "" && region != "" {
		ui.SectionHeader(fmt.Sprintf("Active Cloud Run Services in %s (%s)", region, project))
		services, err := gcp.ListCloudRunServices(project, region)
		if err != nil {
			ui.Warning(fmt.Sprintf("Could not query Cloud Run services directly: %v", err))
		} else if len(services) == 0 {
			ui.Info("No Cloud Run services currently found in this region.")
		} else {
			for _, svc := range services {
				fmt.Printf("\n  %s•%s %s%s%s\n", ui.Green, ui.Reset, ui.Bold, svc.Name, ui.Reset)
				ui.Detail("URL", svc.URL)
				if svc.Image != "" {
					ui.Detail("Image", svc.Image)
				}
			}
		}
	}
}
