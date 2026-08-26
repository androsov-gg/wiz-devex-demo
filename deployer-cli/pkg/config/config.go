package config

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"deployer-cli/pkg/gcp"
	"deployer-cli/pkg/scenario"
	"deployer-cli/pkg/ui"
)

// SupportedRegions defines the list of Google Cloud regions allowed for Vertex AI / Claude Sonnet 4.6 & Cloud Run setup.
var SupportedRegions = []string{
	"europe-west1",
	"us-east5",
	"asia-southeast1",
}

// DeployConfig contains the deployment settings for a demo scenario.
type DeployConfig struct {
	ProjectID     string
	Region        string
	ImageTag      string
	FullImageTag  string
	FullLatestTag string
	StateBucket   string
	IAPMembers    []string
	Scenario      *scenario.Scenario
}

// ValidateRegion verifies if the provided region is in the supported list.
func ValidateRegion(region string) error {
	region = strings.TrimSpace(strings.ToLower(region))
	for _, r := range SupportedRegions {
		if r == region {
			return nil
		}
	}
	return fmt.Errorf("region '%s' is not supported. Supported regions: %s", region, strings.Join(SupportedRegions, ", "))
}

// DetectGCPDefaults attempts to read default project and account from active gcloud configuration.
func DetectGCPDefaults() (project string, account string, err error) {
	// 1. Detect project
	cmdProj := exec.Command("gcloud", "config", "get-value", "project")
	var outProj bytes.Buffer
	cmdProj.Stdout = &outProj
	if err := cmdProj.Run(); err == nil {
		p := strings.TrimSpace(outProj.String())
		if p != "" && p != "(unset)" {
			project = p
		}
	}

	// 2. Detect account
	cmdAcc := exec.Command("gcloud", "config", "get-value", "account")
	var outAcc bytes.Buffer
	cmdAcc.Stdout = &outAcc
	if err := cmdAcc.Run(); err == nil {
		a := strings.TrimSpace(outAcc.String())
		if a != "" && a != "(unset)" {
			account = a
		}
	}

	return project, account, nil
}

// InferDefaultIAPMember suggests a reasonable IAP member string given an active account.
func InferDefaultIAPMember(account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		return "domain:yourname.altostrat.com"
	}

	if strings.Contains(account, "@") {
		parts := strings.Split(account, "@")
		username := parts[0]
		domain := parts[1]

		if strings.HasSuffix(domain, "altostrat.com") {
			return fmt.Sprintf("domain:%s", domain)
		}
		if domain == "google.com" {
			return fmt.Sprintf("domain:%s.altostrat.com", username)
		}
		return fmt.Sprintf("user:%s", account)
	}

	return fmt.Sprintf("domain:%s.altostrat.com", account)
}

// ResolveDeploymentConfig resolves all deployment parameters via flags, defaults, and interactive prompts.
func ResolveDeploymentConfig(selectedScenario *scenario.Scenario, flagProj, flagReg, flagIAP string, interactive bool) (*DeployConfig, error) {
	if selectedScenario == nil {
		return nil, fmt.Errorf("cannot resolve deployment config: scenario is nil")
	}

	detectedProject, detectedAccount, _ := DetectGCPDefaults()
	existingProject, existingRegion, _, _, existingIAP := ReadExistingTFVars(selectedScenario.TerraformDir)

	// 1. Resolve Project ID (Priority: CLI flag > Existing tfvars > Detected gcloud project)
	targetProject := flagProj
	if targetProject == "" {
		defaultProj := existingProject
		if defaultProj == "" {
			defaultProj = detectedProject
		}
		if interactive {
			targetProject = ui.PromptString("Google Cloud Project ID (Argolis)", defaultProj)
		} else {
			targetProject = defaultProj
		}
	}
	if targetProject == "" {
		return nil, fmt.Errorf("GCP project ID is required")
	}

	// 2. Resolve Region (Priority: CLI flag > Existing tfvars > Default europe-west1)
	targetRegion := flagReg
	if targetRegion == "" || targetRegion == "europe-west1" {
		defaultRegion := existingRegion
		if defaultRegion == "" {
			defaultRegion = "europe-west1"
		}
		defaultIdx := 0
		for i, r := range SupportedRegions {
			if r == defaultRegion {
				defaultIdx = i
				break
			}
		}
		if interactive {
			targetRegion = ui.PromptChoice("Google Cloud Region", SupportedRegions, defaultIdx)
		} else {
			targetRegion = defaultRegion
		}
	}

	if err := ValidateRegion(targetRegion); err != nil {
		return nil, err
	}

	// 3. Resolve IAP Member (Priority: CLI flag > Existing tfvars > Inferred from gcloud account)
	targetIAP := flagIAP
	if targetIAP == "" {
		defaultIAP := ""
		if len(existingIAP) > 0 {
			defaultIAP = existingIAP[0]
		}
		if defaultIAP == "" {
			defaultIAP = InferDefaultIAPMember(detectedAccount)
		}
		if interactive {
			targetIAP = ui.PromptString("Allowed IAP Access Member (e.g. domain:username.altostrat.com)", defaultIAP)
		} else {
			targetIAP = defaultIAP
		}
	}

	// 4. Compute Content Hash Fingerprint
	contentHash, err := scenario.ComputeContentHash(selectedScenario.Dir)
	if err != nil || contentHash == "" {
		contentHash = "latest"
	}

	stateBucket := gcp.ResolveStateBucket(targetProject)
	imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/wiz-devex-demo-images/%s:%s", targetRegion, targetProject, selectedScenario.Name, contentHash)
	latestTag := fmt.Sprintf("%s-docker.pkg.dev/%s/wiz-devex-demo-images/%s:latest", targetRegion, targetProject, selectedScenario.Name)

	return &DeployConfig{
		ProjectID:     targetProject,
		Region:        targetRegion,
		ImageTag:      contentHash,
		FullImageTag:  imageTag,
		FullLatestTag: latestTag,
		StateBucket:   stateBucket,
		IAPMembers:    []string{targetIAP},
		Scenario:      selectedScenario,
	}, nil
}

// WriteTFVars writes the terraform.auto.tfvars file inside the scenario's terraform directory.
func WriteTFVars(terraformDir string, cfg *DeployConfig) error {
	if cfg == nil || cfg.Scenario == nil {
		return fmt.Errorf("invalid configuration: missing scenario details")
	}

	if err := os.MkdirAll(terraformDir, 0755); err != nil {
		return fmt.Errorf("failed to create terraform directory: %w", err)
	}

	var iapMembersFormatted []string
	for _, m := range cfg.IAPMembers {
		trimmed := strings.TrimSpace(m)
		if trimmed != "" {
			iapMembersFormatted = append(iapMembersFormatted, fmt.Sprintf("  %q", trimmed))
		}
	}

	if len(iapMembersFormatted) == 0 {
		iapMembersFormatted = append(iapMembersFormatted, "  \"domain:yourname.altostrat.com\"")
	}

	imageTag := cfg.ImageTag
	if imageTag == "" {
		imageTag = "latest"
	}

	content := fmt.Sprintf(`# Automatically generated by deployer-cli. DO NOT EDIT DIRECTLY.
project_id   = %q
region       = %q
service_name = %q
image_tag    = %q
iap_members  = [
%s
]
`, cfg.ProjectID, cfg.Region, cfg.Scenario.Name, imageTag, strings.Join(iapMembersFormatted, ",\n"))

	tfvarsPath := filepath.Join(terraformDir, "terraform.auto.tfvars")
	if err := os.WriteFile(tfvarsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", tfvarsPath, err)
	}

	return nil
}

// ReadExistingTFVars reads simple key/values from an existing terraform.auto.tfvars file if present.
func ReadExistingTFVars(terraformDir string) (projectID, region, serviceName, imageTag string, iapMembers []string) {
	tfvarsPath := filepath.Join(terraformDir, "terraform.auto.tfvars")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		return "", "", "", "", nil
	}

	lines := strings.Split(string(data), "\n")
	inIAP := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}

		if inIAP {
			if strings.Contains(line, "]") {
				inIAP = false
				continue
			}
			val := extractQuotedString(line)
			if val != "" {
				iapMembers = append(iapMembers, val)
			}
			continue
		}

		if strings.HasPrefix(line, "project_id") {
			projectID = extractQuotedString(line)
		} else if strings.HasPrefix(line, "region") {
			region = extractQuotedString(line)
		} else if strings.HasPrefix(line, "service_name") {
			serviceName = extractQuotedString(line)
		} else if strings.HasPrefix(line, "image_tag") {
			imageTag = extractQuotedString(line)
		} else if strings.HasPrefix(line, "iap_members") {
			inIAP = true
		}
	}

	return projectID, region, serviceName, imageTag, iapMembers
}

func extractQuotedString(line string) string {
	first := strings.Index(line, "\"")
	if first == -1 {
		return ""
	}
	last := strings.LastIndex(line, "\"")
	if last <= first {
		return ""
	}
	return line[first+1 : last]
}
