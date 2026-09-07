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

// SupportedRegions defines the list of Google Cloud regions allowed for Agent Platform / Claude Sonnet 4.6 & Cloud Run setup.
var SupportedRegions = []string{
	"europe-west1",
	"us-east5",
	"asia-southeast1",
}

// DeployConfig contains the deployment settings for a demo scenario.
type DeployConfig struct {
	ProjectID         string
	Region            string
	ImageTag          string
	FullImageTag      string
	FullLatestTag     string
	StateBucket       string
	IAPMembers        []string
	Scenario          *scenario.Scenario
	EnableScheduler   bool
	ScheduleTimezone  string
	ScaleDownSchedule string
	ScaleUpSchedule   string
}

// TFVarsData contains all parsed variables from an existing terraform.auto.tfvars file.
type TFVarsData struct {
	ProjectID         string
	Region            string
	ServiceName       string
	ImageTag          string
	IAPMembers        []string
	EnableScheduler   *bool
	ScheduleTimezone  string
	ScaleDownSchedule string
	ScaleUpSchedule   string
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

// NormalizeScheduleCron converts a user time input (e.g. "08:00", "8am", "19:00", "7pm", "8", "19")
// or standard 5-part cron expression into a valid Cloud Scheduler cron expression.
func NormalizeScheduleCron(input, defaultCron string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultCron
	}

	// If it contains multiple spaces, it's already a full cron expression (e.g., "0 8 * * 1-5")
	if strings.Count(input, " ") >= 4 {
		return input
	}

	lower := strings.ToLower(input)
	isPM := strings.Contains(lower, "pm")
	isAM := strings.Contains(lower, "am")
	cleaned := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(lower, "pm", ""), "am", ""))

	var hour, min int
	if strings.Contains(cleaned, ":") {
		parts := strings.Split(cleaned, ":")
		fmt.Sscanf(parts[0], "%d", &hour)
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &min)
		}
	} else {
		fmt.Sscanf(cleaned, "%d", &hour)
		min = 0
	}

	if isPM && hour < 12 {
		hour += 12
	}
	if isAM && hour == 12 {
		hour = 0
	}

	if hour >= 0 && hour <= 23 && min >= 0 && min <= 59 {
		return fmt.Sprintf("%d %d * * 1-5", min, hour)
	}

	return input
}

// ResolveDeploymentConfig resolves all deployment parameters via flags, defaults, and interactive prompts.
func ResolveDeploymentConfig(
	selectedScenario *scenario.Scenario,
	flagProj, flagReg, flagIAP string,
	flagEnableSched, flagSchedTZ, flagScaleDown, flagScaleUp string,
	interactive bool,
) (*DeployConfig, error) {
	if selectedScenario == nil {
		return nil, fmt.Errorf("cannot resolve deployment config: scenario is nil")
	}

	detectedProject, detectedAccount, _ := DetectGCPDefaults()
	existing := ReadExistingFullTFVars(selectedScenario.TerraformDir)

	// Fast-Path: If complete configuration exists and no overriding flags were supplied, offer 1-click reuse
	hasExistingConfig := existing.ProjectID != "" && existing.Region != "" && len(existing.IAPMembers) > 0
	noOverrideFlags := flagProj == "" && (flagReg == "" || flagReg == "europe-west1") && flagIAP == "" && flagEnableSched == ""

	if interactive && hasExistingConfig && noOverrideFlags {
		ui.SectionHeader(fmt.Sprintf("Existing Configuration Found (%s)", selectedScenario.Name))
		ui.Detail("GCP Project", existing.ProjectID)
		ui.Detail("GCP Region", existing.Region)
		ui.Detail("IAP Member", existing.IAPMembers[0])
		schedDesc := "Disabled (Always 1 min instance)"
		if existing.EnableScheduler == nil || *existing.EnableScheduler {
			tz := existing.ScheduleTimezone
			if tz == "" {
				tz = "UTC"
			}
			schedDesc = fmt.Sprintf("Enabled (%s to %s, %s)", existing.ScaleUpSchedule, existing.ScaleDownSchedule, tz)
		}
		ui.Detail("Off-Hours Scaling", schedDesc)

		if ui.PromptConfirm("Reuse this existing configuration?", true) {
			enableSched := true
			if existing.EnableScheduler != nil {
				enableSched = *existing.EnableScheduler
			}
			schedTimezone := "UTC"
			if existing.ScheduleTimezone != "" {
				schedTimezone = existing.ScheduleTimezone
			}
			scaleUpSched := "0 8 * * 1-5"
			if existing.ScaleUpSchedule != "" {
				scaleUpSched = existing.ScaleUpSchedule
			}
			scaleDownSched := "0 19 * * 1-5"
			if existing.ScaleDownSchedule != "" {
				scaleDownSched = existing.ScaleDownSchedule
			}

			contentHash, err := scenario.ComputeContentHash(selectedScenario.Dir)
			if err != nil || contentHash == "" {
				contentHash = "latest"
			}

			stateBucket := gcp.ResolveStateBucket(existing.ProjectID)
			imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/wiz-devex-demo-images/%s:%s", existing.Region, existing.ProjectID, selectedScenario.Name, contentHash)
			latestTag := fmt.Sprintf("%s-docker.pkg.dev/%s/wiz-devex-demo-images/%s:latest", existing.Region, existing.ProjectID, selectedScenario.Name)

			return &DeployConfig{
				ProjectID:         existing.ProjectID,
				Region:            existing.Region,
				ImageTag:          contentHash,
				FullImageTag:      imageTag,
				FullLatestTag:     latestTag,
				StateBucket:       stateBucket,
				IAPMembers:        existing.IAPMembers,
				Scenario:          selectedScenario,
				EnableScheduler:   enableSched,
				ScheduleTimezone:  schedTimezone,
				ScaleDownSchedule: scaleDownSched,
				ScaleUpSchedule:   scaleUpSched,
			}, nil
		}
	}

	// 1. Resolve Project ID (Priority: CLI flag > Existing tfvars > Detected gcloud project)
	targetProject := flagProj
	if targetProject == "" {
		defaultProj := existing.ProjectID
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
		defaultRegion := existing.Region
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
		if len(existing.IAPMembers) > 0 {
			defaultIAP = existing.IAPMembers[0]
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

	// 4. Resolve Cloud Scheduler settings (Off-hours scaling: scale down to 0 at end of working day, back up to 1 at start of day)
	enableSched := true
	schedTimezone := "UTC"
	if existing.ScheduleTimezone != "" {
		schedTimezone = existing.ScheduleTimezone
	}
	scaleUpSched := "0 8 * * 1-5"
	if existing.ScaleUpSchedule != "" {
		scaleUpSched = existing.ScaleUpSchedule
	}
	scaleDownSched := "0 19 * * 1-5"
	if existing.ScaleDownSchedule != "" {
		scaleDownSched = existing.ScaleDownSchedule
	}

	if flagEnableSched != "" {
		enableSched = (strings.ToLower(flagEnableSched) == "true" || flagEnableSched == "1")
		if flagSchedTZ != "" {
			schedTimezone = flagSchedTZ
		}
		if flagScaleUp != "" {
			scaleUpSched = NormalizeScheduleCron(flagScaleUp, "0 8 * * 1-5")
		}
		if flagScaleDown != "" {
			scaleDownSched = NormalizeScheduleCron(flagScaleDown, "0 19 * * 1-5")
		}
	} else if interactive {
		schedOptions := []string{
			fmt.Sprintf("Standard Off-Hours Scaling (0 min instances off-hours, 1 min instance 08:00-19:00 Mon-Fri, %s)", schedTimezone),
			"Custom Hours & Timezone",
			"Disabled (Always keep 1 min instance running 24/7)",
		}
		chosen := ui.PromptChoice("Cloud Scheduler Off-Hours Cost Optimization", schedOptions, 0)
		switch chosen {
		case schedOptions[0]:
			enableSched = true
			scaleUpSched = "0 8 * * 1-5"
			scaleDownSched = "0 19 * * 1-5"
		case schedOptions[1]:
			enableSched = true
			schedTimezone = ui.PromptString("Schedule Timezone (IANA format, e.g. America/New_York, Europe/London, America/Los_Angeles, UTC)", schedTimezone)
			rawUp := ui.PromptString("Working day start time (scale up to 1 instance) [Mon-Fri] (e.g. 08:00, 8am, or cron '0 8 * * 1-5')", "08:00")
			scaleUpSched = NormalizeScheduleCron(rawUp, "0 8 * * 1-5")
			rawDown := ui.PromptString("Working day end time (scale down to 0 instances) [Mon-Fri] (e.g. 19:00, 7pm, or cron '0 19 * * 1-5')", "19:00")
			scaleDownSched = NormalizeScheduleCron(rawDown, "0 19 * * 1-5")
		case schedOptions[2]:
			enableSched = false
		}
	}

	// 5. Compute Content Hash Fingerprint
	contentHash, err := scenario.ComputeContentHash(selectedScenario.Dir)
	if err != nil || contentHash == "" {
		contentHash = "latest"
	}

	stateBucket := gcp.ResolveStateBucket(targetProject)
	imageTag := fmt.Sprintf("%s-docker.pkg.dev/%s/wiz-devex-demo-images/%s:%s", targetRegion, targetProject, selectedScenario.Name, contentHash)
	latestTag := fmt.Sprintf("%s-docker.pkg.dev/%s/wiz-devex-demo-images/%s:latest", targetRegion, targetProject, selectedScenario.Name)

	return &DeployConfig{
		ProjectID:         targetProject,
		Region:            targetRegion,
		ImageTag:          contentHash,
		FullImageTag:      imageTag,
		FullLatestTag:     latestTag,
		StateBucket:       stateBucket,
		IAPMembers:        []string{targetIAP},
		Scenario:          selectedScenario,
		EnableScheduler:   enableSched,
		ScheduleTimezone:  schedTimezone,
		ScaleDownSchedule: scaleDownSched,
		ScaleUpSchedule:   scaleUpSched,
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

	timezone := cfg.ScheduleTimezone
	if timezone == "" {
		timezone = "UTC"
	}
	scaleDown := cfg.ScaleDownSchedule
	if scaleDown == "" {
		scaleDown = "0 19 * * 1-5"
	}
	scaleUp := cfg.ScaleUpSchedule
	if scaleUp == "" {
		scaleUp = "0 8 * * 1-5"
	}

	content := fmt.Sprintf(`# Automatically generated by deployer-cli. DO NOT EDIT DIRECTLY.
project_id   = %q
region       = %q
service_name = %q
image_tag    = %q
iap_members  = [
%s
]

# Cloud Scheduler Off-Hours Scaling Settings (Cost Optimization)
enable_scheduler    = %t
schedule_timezone   = %q
scale_down_schedule = %q
scale_up_schedule   = %q
`, cfg.ProjectID, cfg.Region, cfg.Scenario.Name, imageTag, strings.Join(iapMembersFormatted, ",\n"),
		cfg.EnableScheduler, timezone, scaleDown, scaleUp)

	tfvarsPath := filepath.Join(terraformDir, "terraform.auto.tfvars")
	if err := os.WriteFile(tfvarsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", tfvarsPath, err)
	}

	return nil
}

// ReadExistingFullTFVars reads all key/values from an existing terraform.auto.tfvars file if present.
func ReadExistingFullTFVars(terraformDir string) TFVarsData {
	tfvarsPath := filepath.Join(terraformDir, "terraform.auto.tfvars")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		return TFVarsData{}
	}

	var res TFVarsData
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
				res.IAPMembers = append(res.IAPMembers, val)
			}
			continue
		}

		if strings.HasPrefix(line, "project_id") {
			res.ProjectID = extractQuotedString(line)
		} else if strings.HasPrefix(line, "region") {
			res.Region = extractQuotedString(line)
		} else if strings.HasPrefix(line, "service_name") {
			res.ServiceName = extractQuotedString(line)
		} else if strings.HasPrefix(line, "image_tag") {
			res.ImageTag = extractQuotedString(line)
		} else if strings.HasPrefix(line, "iap_members") {
			inIAP = true
		} else if strings.HasPrefix(line, "enable_scheduler") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "enable_scheduler"))
			val = strings.TrimSpace(strings.TrimPrefix(val, "="))
			val = strings.ToLower(strings.Trim(val, `"'`))
			b := (val == "true")
			res.EnableScheduler = &b
		} else if strings.HasPrefix(line, "schedule_timezone") {
			res.ScheduleTimezone = extractQuotedString(line)
		} else if strings.HasPrefix(line, "scale_down_schedule") {
			res.ScaleDownSchedule = extractQuotedString(line)
		} else if strings.HasPrefix(line, "scale_up_schedule") {
			res.ScaleUpSchedule = extractQuotedString(line)
		}
	}

	return res
}

// ReadExistingTFVars reads simple key/values from an existing terraform.auto.tfvars file if present.
func ReadExistingTFVars(terraformDir string) (projectID, region, serviceName, imageTag string, iapMembers []string) {
	data := ReadExistingFullTFVars(terraformDir)
	return data.ProjectID, data.Region, data.ServiceName, data.ImageTag, data.IAPMembers
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
