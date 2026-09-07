package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"deployer-cli/pkg/ui"

	"github.com/hashicorp/go-version"
)

// CheckResult represents the outcome of an individual diagnostic check.
type CheckResult struct {
	Name      string
	Passed    bool
	IsWarning bool
	Message   string
	Details   string
}

// RunDiagnostics runs all local and GCP pre-flight environment checks.
func RunDiagnostics(projectID string) ([]CheckResult, bool) {
	var results []CheckResult
	allPassed := true

	// 1. Check gcloud CLI
	gcloudRes, gcloudPath := checkGCloud()
	results = append(results, gcloudRes)
	if !gcloudRes.Passed && !gcloudRes.IsWarning {
		allPassed = false
	}

	// 2. Check Terraform CLI & version (>= 1.3.0)
	tfRes := checkTerraform()
	results = append(results, tfRes)
	if !tfRes.Passed && !tfRes.IsWarning {
		allPassed = false
	}

	// 3. Check git CLI
	gitRes := checkGit()
	results = append(results, gitRes)
	if !gitRes.Passed && !gitRes.IsWarning {
		allPassed = false
	}

	// 4. Check gcloud authentication & token freshness
	var gcloudAccount string
	if gcloudPath != "" {
		authRes, acc := checkGCloudAuth()
		results = append(results, authRes)
		if !authRes.Passed && !authRes.IsWarning {
			allPassed = false
		}
		gcloudAccount = acc
	}

	// 5. Check Application Default Credentials (ADC) & token freshness
	if gcloudPath != "" {
		adcResults := checkADC(gcloudAccount)
		for _, r := range adcResults {
			results = append(results, r)
			if !r.Passed && !r.IsWarning {
				allPassed = false
			}
		}
	}

	// 6. Check target GCP Project access if specified
	if projectID != "" && gcloudPath != "" {
		projRes := checkProjectAccess(projectID)
		results = append(results, projRes)
		if !projRes.Passed && !projRes.IsWarning {
			allPassed = false
		}
	}

	return results, allPassed
}

func checkGCloud() (CheckResult, string) {
	gcloudPath, err := exec.LookPath("gcloud")
	if err != nil {
		return CheckResult{
			Name:    "Google Cloud SDK (gcloud)",
			Passed:  false,
			Message: "gcloud CLI not found in PATH",
			Details: "Please install the Google Cloud SDK: https://cloud.google.com/sdk/docs/install",
		}, ""
	}

	version := getCommandOutput("gcloud", "--version")
	firstLine := "installed"
	if version != "" {
		lines := strings.Split(version, "\n")
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "Google Cloud SDK") {
				firstLine = trimmed
				break
			}
		}
	}

	return CheckResult{
		Name:    "Google Cloud SDK (gcloud)",
		Passed:  true,
		Message: fmt.Sprintf("Found at %s (%s)", gcloudPath, firstLine),
	}, gcloudPath
}

func checkTerraform() CheckResult {
	tfPath, err := exec.LookPath("terraform")
	if err != nil {
		return CheckResult{
			Name:    "Terraform CLI (>= 1.3.0)",
			Passed:  false,
			Message: "terraform CLI not found in PATH",
			Details: "Please install Terraform (>= 1.3.0): https://developer.hashicorp.com/terraform/install",
		}
	}

	versionOut := getCommandOutput("terraform", "version")
	parsedVer, firstLine := ExtractTerraformVersion(versionOut)
	if parsedVer == "" {
		return CheckResult{
			Name:    "Terraform CLI (>= 1.3.0)",
			Passed:  true,
			Message: fmt.Sprintf("Found at %s", tfPath),
		}
	}

	minVer, _ := version.NewVersion("1.3.0")
	currentVer, err := version.NewVersion(parsedVer)
	if err != nil {
		return CheckResult{
			Name:    "Terraform CLI (>= 1.3.0)",
			Passed:  true,
			Message: fmt.Sprintf("Found at %s (%s)", tfPath, firstLine),
		}
	}

	if currentVer.LessThan(minVer) {
		return CheckResult{
			Name:    "Terraform CLI (>= 1.3.0)",
			Passed:  false,
			Message: fmt.Sprintf("Found version %s at %s, but required is >= 1.3.0", parsedVer, tfPath),
			Details: "Please upgrade Terraform: https://developer.hashicorp.com/terraform/install",
		}
	}

	return CheckResult{
		Name:    "Terraform CLI (>= 1.3.0)",
		Passed:  true,
		Message: fmt.Sprintf("Found at %s (%s)", tfPath, firstLine),
	}
}

// ExtractTerraformVersion parses the version string from terraform version output.
func ExtractTerraformVersion(output string) (string, string) {
	if output == "" {
		return "", ""
	}
	firstLine := strings.Split(output, "\n")[0]
	firstLine = strings.TrimSpace(firstLine)

	fields := strings.Fields(firstLine)
	for _, f := range fields {
		if strings.HasPrefix(f, "v") && len(f) > 1 {
			verStr := strings.TrimPrefix(f, "v")
			return verStr, firstLine
		}
	}
	return "", firstLine
}

func checkGit() CheckResult {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return CheckResult{
			Name:    "Git CLI",
			Passed:  false,
			Message: "git CLI not found in PATH",
			Details: "Please install Git: https://git-scm.com/downloads",
		}
	}
	version := getCommandOutput("git", "--version")
	msg := fmt.Sprintf("Found at %s", gitPath)
	if version != "" {
		msg = fmt.Sprintf("Found at %s (%s)", gitPath, version)
	}
	return CheckResult{
		Name:    "Git CLI",
		Passed:  true,
		Message: msg,
	}
}

func checkGCloudAuth() (CheckResult, string) {
	cmd := exec.Command("gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || strings.TrimSpace(out.String()) == "" {
		return CheckResult{
			Name:    "gcloud CLI Authentication",
			Passed:  false,
			Message: "No active Google Cloud credentials detected",
			Details: "Run `gcloud auth login --update-adc` to authenticate both gcloud and Terraform.",
		}, ""
	}

	account := strings.TrimSpace(out.String())
	lines := strings.Split(account, "\n")
	account = strings.TrimSpace(lines[0])

	// Verify token freshness
	tokenCmd := exec.Command("gcloud", "auth", "print-access-token")
	var tokenOut bytes.Buffer
	tokenCmd.Stdout = &tokenOut
	if err := tokenCmd.Run(); err != nil || strings.TrimSpace(tokenOut.String()) == "" {
		return CheckResult{
			Name:    "gcloud CLI Authentication",
			Passed:  false,
			Message: fmt.Sprintf("Active account '%s' detected, but OAuth token is expired or invalid", account),
			Details: "Your login session may have expired. Run `gcloud auth login --update-adc` to refresh credentials.",
		}, account
	}

	return CheckResult{
		Name:    "gcloud CLI Authentication",
		Passed:  true,
		Message: fmt.Sprintf("Authenticated as %s (token active & fresh)", account),
	}, account
}

func checkADC(gcloudAccount string) []CheckResult {
	var results []CheckResult

	// 1. Check if token can be generated via ADC
	adcCmd := exec.Command("gcloud", "auth", "application-default", "print-access-token")
	var tokenOut bytes.Buffer
	adcCmd.Stdout = &tokenOut
	adcErr := adcCmd.Run()
	adcToken := strings.TrimSpace(tokenOut.String())

	if adcErr != nil || adcToken == "" {
		results = append(results, CheckResult{
			Name:    "Application Default Credentials (ADC)",
			Passed:  false,
			Message: "ADC not configured or credentials expired",
			Details: "Terraform and Go SDK require ADC. Run `gcloud auth login --update-adc` (or `gcloud auth application-default login`) to configure.",
		})
		return results
	}

	// 2. Discover ADC Account Identity if available
	adcAccount := readADCAccount()
	msg := "Credentials valid and token fresh"
	if adcAccount != "" {
		msg = fmt.Sprintf("Configured as %s (token active & fresh)", adcAccount)
	}

	results = append(results, CheckResult{
		Name:    "Application Default Credentials (ADC)",
		Passed:  true,
		Message: msg,
	})

	// 3. Check Account Parity if both identities are known
	if gcloudAccount != "" && adcAccount != "" && !strings.EqualFold(gcloudAccount, adcAccount) {
		results = append(results, CheckResult{
			Name:      "Credential Account Parity",
			Passed:    true,
			IsWarning: true,
			Message:   fmt.Sprintf("Account mismatch detected (gcloud: '%s' vs ADC: '%s')", gcloudAccount, adcAccount),
			Details:   "Terraform uses ADC while Cloud Build uses gcloud. Run `gcloud auth login --update-adc` to synchronize them.",
		})
	}

	return results
}

func readADCAccount() string {
	var adcPath string
	if envPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); envPath != "" {
		adcPath = envPath
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			adcPath = filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
		}
	}

	if adcPath == "" {
		return ""
	}

	data, err := os.ReadFile(adcPath)
	if err != nil {
		return ""
	}

	var parsed struct {
		Account     string `json:"account"`
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(data, &parsed); err == nil {
		if parsed.Account != "" {
			return parsed.Account
		}
		if parsed.ClientEmail != "" {
			return parsed.ClientEmail
		}
	}

	return ""
}

func checkProjectAccess(projectID string) CheckResult {
	cmd := exec.Command("gcloud", "projects", "describe", projectID, "--format=value(projectId)")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return CheckResult{
			Name:    fmt.Sprintf("GCP Project Access (%s)", projectID),
			Passed:  false,
			Message: fmt.Sprintf("Cannot describe project %s. Ensure you have permissions.", projectID),
			Details: fmt.Sprintf("Verify that project '%s' exists and your active account has access.", projectID),
		}
	}

	return CheckResult{
		Name:    fmt.Sprintf("GCP Project Access (%s)", projectID),
		Passed:  true,
		Message: fmt.Sprintf("Verified access to %s", projectID),
	}
}

// PrintDiagnostics displays the diagnostic results nicely in the terminal.
func PrintDiagnostics(results []CheckResult) {
	ui.SectionHeader("Doctor Pre-Flight Diagnostics")
	for _, res := range results {
		if res.Passed && !res.IsWarning {
			ui.Success(fmt.Sprintf("%s: %s", res.Name, res.Message))
		} else if res.IsWarning {
			ui.Warning(fmt.Sprintf("%s: %s", res.Name, res.Message))
			if res.Details != "" {
				fmt.Printf("      %s%s%s\n", ui.Dim, res.Details, ui.Reset)
			}
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
