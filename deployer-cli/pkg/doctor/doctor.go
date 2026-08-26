package doctor

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"deployer-cli/pkg/ui"
)

// CheckResult represents the outcome of an individual diagnostic check.
type CheckResult struct {
	Name    string
	Passed  bool
	Message string
	Details string
}

// RunDiagnostics runs all local and GCP pre-flight environment checks.
func RunDiagnostics(projectID string) ([]CheckResult, bool) {
	var results []CheckResult
	allPassed := true

	// 1. Check gcloud CLI
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Google Cloud SDK (gcloud)",
			Passed:  false,
			Message: "gcloud CLI not found in PATH",
			Details: "Please install the Google Cloud SDK: https://cloud.google.com/sdk/docs/install",
		})
		allPassed = false
	} else {
		version := getCommandOutput("gcloud", "--version")
		firstLine := strings.Split(version, "\n")[0]
		results = append(results, CheckResult{
			Name:    "Google Cloud SDK (gcloud)",
			Passed:  true,
			Message: fmt.Sprintf("Found at %s (%s)", gcloudPath, firstLine),
		})
	}

	// 2. Check terraform CLI
	tfPath, err := exec.LookPath("terraform")
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Terraform CLI",
			Passed:  false,
			Message: "terraform CLI not found in PATH",
			Details: "Please install Terraform (>= 1.3.0): https://developer.hashicorp.com/terraform/install",
		})
		allPassed = false
	} else {
		version := getCommandOutput("terraform", "version")
		firstLine := strings.Split(version, "\n")[0]
		results = append(results, CheckResult{
			Name:    "Terraform CLI",
			Passed:  true,
			Message: fmt.Sprintf("Found at %s (%s)", tfPath, firstLine),
		})
	}

	// 3. Check git CLI
	gitPath, err := exec.LookPath("git")
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Git CLI",
			Passed:  false,
			Message: "git CLI not found in PATH",
			Details: "Please install Git: https://git-scm.com/downloads",
		})
		allPassed = false
	} else {
		results = append(results, CheckResult{
			Name:    "Git CLI",
			Passed:  true,
			Message: fmt.Sprintf("Found at %s", gitPath),
		})
	}

	// 4. Check gcloud authentication
	if gcloudPath != "" {
		cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil || strings.TrimSpace(out.String()) == "" {
			results = append(results, CheckResult{
				Name:    "GCP Authentication",
				Passed:  false,
				Message: "No active Google Cloud credentials detected",
				Details: "Run `gcloud auth login` and `gcloud auth application-default login` to authenticate.",
			})
			allPassed = false
		} else {
			account := strings.TrimSpace(out.String())
			results = append(results, CheckResult{
				Name:    "GCP Authentication",
				Passed:  true,
				Message: fmt.Sprintf("Authenticated as %s", account),
			})
		}
	}

	// 5. Check target GCP Project access if specified
	if projectID != "" && gcloudPath != "" {
		cmd := exec.Command("gcloud", "projects", "describe", projectID, "--format=value(projectId)")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("GCP Project Access (%s)", projectID),
				Passed:  false,
				Message: fmt.Sprintf("Cannot describe project %s. Ensure you have permissions.", projectID),
			})
			allPassed = false
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("GCP Project Access (%s)", projectID),
				Passed:  true,
				Message: fmt.Sprintf("Verified access to %s", projectID),
			})
		}
	}

	return results, allPassed
}

// PrintDiagnostics displays the diagnostic results nicely in the terminal.
func PrintDiagnostics(results []CheckResult) {
	ui.SectionHeader("Doctor Pre-Flight Diagnostics")
	for _, res := range results {
		if res.Passed {
			ui.Success(fmt.Sprintf("%s: %s", res.Name, res.Message))
		} else {
			ui.Error(fmt.Sprintf("%s: %s", res.Name, res.Message))
			if res.Details != "" {
				fmt.Printf("      %s%s%s\n", ui.Dim, res.Details, ui.Reset)
			}
		}
	}
}

func getCommandOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}
