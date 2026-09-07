package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[90m"
	White   = "\033[97m"
)

// Banner prints the stylish Wiz DevEx CLI banner.
func Banner() {
	fmt.Printf("%s%s", Cyan, Bold)
	fmt.Println(`
  ╔═══════════════════════════════════════════════════════════════════════╗
  ║                 🧙 WIZ DEVELOPER EXPERIENCE DEMO                      ║
  ║                   Google Cloud Argolis Deployer                       ║
  ╚═══════════════════════════════════════════════════════════════════════╝`)
	fmt.Printf("%s\n", Reset)
}

// StepHeader prints a structured step header.
func StepHeader(step, total int, message string) {
	fmt.Printf("\n%s%s[%d/%d]%s %s%s%s\n", Cyan, Bold, step, total, Reset, Bold, message, Reset)
	fmt.Println(strings.Repeat("─", 65))
}

// SectionHeader prints a prominent section title.
func SectionHeader(title string) {
	fmt.Printf("\n%s%s=== %s ===%s\n\n", Magenta, Bold, title, Reset)
}

// Success prints a success message.
func Success(message string) {
	fmt.Printf("  %s%s✓%s %s\n", Green, Bold, Reset, message)
}

// Warning prints a warning message.
func Warning(message string) {
	fmt.Printf("  %s%s⚠%s %s%s%s\n", Yellow, Bold, Reset, Yellow, message, Reset)
}

// Error prints an error message.
func Error(message string) {
	fmt.Printf("  %s%s✖%s %s%s%s\n", Red, Bold, Reset, Red, message, Reset)
}

// Info prints an informational message.
func Info(message string) {
	fmt.Printf("  %sℹ%s %s\n", Cyan, Reset, message)
}

// Detail prints key-value formatted bullet.
func Detail(key, val string) {
	fmt.Printf("  %s•%s %s%-20s%s : %s%s%s\n", Gray, Reset, Bold, key, Reset, Cyan, val, Reset)
}

// PromptString asks the user for a text input with an optional default.
func PromptString(label, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		if defaultValue != "" {
			fmt.Printf("%s?%s %s%s%s [%s%s%s]: ", Green, Reset, Bold, label, Reset, Dim, defaultValue, Reset)
		} else {
			fmt.Printf("%s?%s %s%s%s: ", Green, Reset, Bold, label, Reset)
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			return defaultValue
		}

		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			if defaultValue != "" {
				return defaultValue
			}
			continue
		}
		return trimmed
	}
}

// PromptChoice presents a numbered list of choices to the user.
func PromptChoice(label string, options []string, defaultIndex int) string {
	if len(options) == 0 {
		return ""
	}
	if len(options) == 1 {
		return options[0]
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%s?%s %s%s%s\n", Green, Reset, Bold, label, Reset)
	for i, opt := range options {
		marker := " "
		if i == defaultIndex {
			marker = "*"
		}
		fmt.Printf("  %s%d)%s %s %s\n", Cyan, i+1, Reset, marker, opt)
	}

	for {
		defaultOptNum := defaultIndex + 1
		fmt.Printf("Select choice (1-%d) [%d]: ", len(options), defaultOptNum)
		input, err := reader.ReadString('\n')
		if err != nil {
			return options[defaultIndex]
		}

		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return options[defaultIndex]
		}

		num, err := strconv.Atoi(trimmed)
		if err == nil && num >= 1 && num <= len(options) {
			return options[num-1]
		}
		fmt.Printf("%sInvalid choice. Please enter a number between 1 and %d.%s\n", Red, len(options), Reset)
	}
}

// PromptConfirm asks for a yes/no confirmation.
func PromptConfirm(prompt string, defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}

	for {
		fmt.Printf("\n%s?%s %s%s%s [%s]: ", Yellow, Reset, Bold, prompt, Reset, hint)
		input, err := reader.ReadString('\n')
		if err != nil {
			return defaultYes
		}

		trimmed := strings.ToLower(strings.TrimSpace(input))
		if trimmed == "" {
			return defaultYes
		}

		if trimmed == "y" || trimmed == "yes" {
			return true
		}
		if trimmed == "n" || trimmed == "no" {
			return false
		}
		fmt.Printf("%sPlease answer 'y' or 'n'.%s\n", Red, Reset)
	}
}

// PrintModelGardenInstructions prints the helpful Argolis Model Garden walkthrough.
func PrintModelGardenInstructions(projectID string) {
	consoleURL := fmt.Sprintf("https://console.cloud.google.com/vertex-ai/model-garden?project=%s", projectID)
	fmt.Printf("\n%s%s📋 Prerequisite: Claude Sonnet 4.6 on Agent Platform Model Garden%s\n", Yellow, Bold, Reset)
	fmt.Printf("  Argolis projects require one-time approval in Model Garden before Agent Platform can invoke Claude:\n")
	fmt.Printf("  %s1. Open URL:%s %s%s%s\n", Bold, Reset, Cyan, consoleURL, Reset)
	fmt.Printf("  %s2. Search for:%s \"Claude Sonnet 4.6\" (Anthropic)\n", Bold, Reset)
	fmt.Printf("  %s3. Click \"Enable\" / \"Accept Agreement\" with these Argolis values:%s\n", Bold, Reset)
	fmt.Printf("      • Business Name       : %sGoogle LLC%s\n", Cyan, Reset)
	fmt.Printf("      • Business Website    : %shttps://www.google.com%s\n", Cyan, Reset)
	fmt.Printf("      • HQ Location         : %sUnited States of America%s\n", Cyan, Reset)
	fmt.Printf("      • Industry            : %sTelecommunications%s\n", Cyan, Reset)
	fmt.Printf("      • Intended Users      : %sInternal employees%s\n", Cyan, Reset)
	fmt.Printf("      • Intended Use Cases  : %sCustomer demonstrations%s\n\n", Cyan, Reset)
}

// PrintDeploymentSummary prints a celebratory summary box upon deployment completion.
func PrintDeploymentSummary(url, project, region, scenario, imageTag string) {
	fmt.Printf("\n%s%s", Green, Bold)
	fmt.Println("╔═══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║               🎉 DEMO ENVIRONMENT DEPLOYED SUCCESSFULLY!              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════╝")
	fmt.Printf("%s\n", Reset)

	Detail("Scenario", scenario)
	Detail("GCP Project", project)
	Detail("GCP Region", region)
	Detail("Container Image", imageTag)
	fmt.Println()
	fmt.Printf("  %s%s🚀 Cloud Run URL (Secured by IAP):%s\n", Green, Bold, Reset)
	fmt.Printf("  %s%s%s\n\n", Cyan, Bold, url)
}
