package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"deployer-cli/pkg/ui"
)

// Scenario represents a self-contained demo configuration scenario.
type Scenario struct {
	Name         string
	Dir          string
	TerraformDir string
	Dockerfile   string
	CloudBuild   string
	Description  string
	StatePrefix  string
}

// FindRepoRoot attempts to find the root directory of the repository.
func FindRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check for marker directories or files
		if _, err := os.Stat(filepath.Join(dir, "configurations")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not locate repository root (configurations/ directory not found in parent paths)")
}

// DiscoverScenarios scans configurations/ for valid scenarios.
func DiscoverScenarios(repoRoot string) ([]Scenario, error) {
	configsDir := filepath.Join(repoRoot, "configurations")
	entries, err := os.ReadDir(configsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read configurations directory at %s: %w", configsDir, err)
	}

	var scenarios []Scenario
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		scenarioDir := filepath.Join(configsDir, name)
		tfDir := filepath.Join(scenarioDir, "terraform")

		// A valid scenario must have a terraform directory with at least one .tf file
		// or contain a Dockerfile / cloudbuild.yaml
		hasTF := false
		if tfEntries, err := os.ReadDir(tfDir); err == nil {
			for _, tfEntry := range tfEntries {
				if !tfEntry.IsDir() && strings.HasSuffix(tfEntry.Name(), ".tf") {
					hasTF = true
					break
				}
			}
		}

		dockerfilePath := filepath.Join(scenarioDir, "Dockerfile")
		hasDocker := fileExists(dockerfilePath)
		cloudBuildPath := filepath.Join(scenarioDir, "cloudbuild.yaml")

		if hasTF || hasDocker {
			// For backward compatibility: wiz-code-golden-demo uses root state (no prefix)
			// Future scenarios use prefix: scenarios/<name>
			statePrefix := ""
			if name != "wiz-code-golden-demo" {
				statePrefix = fmt.Sprintf("scenarios/%s", name)
			}

			description := humanizeName(name)
			// Read description if README exists in scenario dir
			readmePath := filepath.Join(scenarioDir, "README.md")
			if desc := readFirstHeading(readmePath); desc != "" {
				description = desc
			}

			scenarios = append(scenarios, Scenario{
				Name:         name,
				Dir:          scenarioDir,
				TerraformDir: tfDir,
				Dockerfile:   dockerfilePath,
				CloudBuild:   cloudBuildPath,
				Description:  description,
				StatePrefix:  statePrefix,
			})
		}
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no demo scenarios found under %s", configsDir)
	}

	return scenarios, nil
}

// GetScenario finds a specific scenario by name.
func GetScenario(repoRoot, name string) (*Scenario, error) {
	scenarios, err := DiscoverScenarios(repoRoot)
	if err != nil {
		return nil, err
	}

	for _, s := range scenarios {
		if strings.EqualFold(s.Name, name) {
			return &s, nil
		}
	}

	var available []string
	for _, s := range scenarios {
		available = append(available, s.Name)
	}
	return nil, fmt.Errorf("scenario '%s' not found. Available scenarios: %s", name, strings.Join(available, ", "))
}

// ResolveScenario selects a scenario from flag, defaults if only 1 exists, or prompts the user interactively.
func ResolveScenario(repoRoot, flagScenario, promptTitle string) (*Scenario, error) {
	scenarios, err := DiscoverScenarios(repoRoot)
	if err != nil {
		return nil, err
	}

	if flagScenario != "" {
		return GetScenario(repoRoot, flagScenario)
	}

	if len(scenarios) == 1 {
		ui.Info(fmt.Sprintf("Using demo scenario: %s%s%s", ui.Bold, scenarios[0].Name, ui.Reset))
		return &scenarios[0], nil
	}

	var names []string
	for _, s := range scenarios {
		desc := s.Name
		if s.Description != "" {
			desc = fmt.Sprintf("%s (%s)", s.Name, s.Description)
		}
		names = append(names, desc)
	}

	if promptTitle == "" {
		promptTitle = "Select Demo Scenario"
	}

	chosen := ui.PromptChoice(promptTitle, names, 0)
	for i, nameDesc := range names {
		if nameDesc == chosen {
			return &scenarios[i], nil
		}
	}

	return &scenarios[0], nil
}

// ComputeContentHash computes a deterministic 12-char SHA-256 fingerprint of all container files in a scenario.
// It explicitly ignores terraform/ configuration directories, hidden files, and build artifacts.
func ComputeContentHash(scenarioDir string) (string, error) {
	var filePaths []string

	err := filepath.Walk(scenarioDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(scenarioDir, path)
		if err != nil {
			return err
		}

		// Skip terraform directory and its contents
		if info.IsDir() {
			if relPath == "terraform" || relPath == ".terraform" || relPath == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files within terraform/ or hidden files
		parts := strings.Split(relPath, string(filepath.Separator))
		for _, part := range parts {
			if part == "terraform" || part == ".terraform" {
				return nil
			}
		}

		if strings.HasSuffix(info.Name(), ".tf") || strings.HasSuffix(info.Name(), ".tfvars") {
			return nil
		}
		if info.Name() == ".DS_Store" {
			return nil
		}

		filePaths = append(filePaths, relPath)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to scan scenario files in %s: %w", scenarioDir, err)
	}

	sort.Strings(filePaths)

	hasher := sha256.New()
	for _, relPath := range filePaths {
		fullPath := filepath.Join(scenarioDir, relPath)
		// Write relative path into hash to capture file renames
		hasher.Write([]byte(relPath))

		f, err := os.Open(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read %s for hashing: %w", fullPath, err)
		}
		_, err = io.Copy(hasher, f)
		f.Close()
		if err != nil {
			return "", fmt.Errorf("failed to hash content of %s: %w", fullPath, err)
		}
	}

	fullHash := hex.EncodeToString(hasher.Sum(nil))
	if len(fullHash) > 12 {
		return fullHash[:12], nil
	}
	return fullHash, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func humanizeName(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func readFirstHeading(readmePath string) string {
	content, err := os.ReadFile(readmePath)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}
	return ""
}
