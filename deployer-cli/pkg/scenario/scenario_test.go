package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverScenarios(t *testing.T) {
	tempDir := t.TempDir()

	configsDir := filepath.Join(tempDir, "configurations")
	scenario1 := filepath.Join(configsDir, "wiz-code-golden-demo", "terraform")
	scenario2 := filepath.Join(configsDir, "wiz-devex-multi-agent", "terraform")

	if err := os.MkdirAll(scenario1, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.MkdirAll(scenario2, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Write mock tf files
	os.WriteFile(filepath.Join(scenario1, "main.tf"), []byte("# mock"), 0644)
	os.WriteFile(filepath.Join(scenario2, "main.tf"), []byte("# mock"), 0644)

	scenarios, err := DiscoverScenarios(tempDir)
	if err != nil {
		t.Fatalf("unexpected error discovering scenarios: %v", err)
	}

	if len(scenarios) != 2 {
		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
	}

	// Check state prefixes
	var golden, multi *Scenario
	for i := range scenarios {
		if scenarios[i].Name == "wiz-code-golden-demo" {
			golden = &scenarios[i]
		} else if scenarios[i].Name == "wiz-devex-multi-agent" {
			multi = &scenarios[i]
		}
	}

	if golden == nil || golden.StatePrefix != "" {
		t.Errorf("expected empty state prefix for wiz-code-golden-demo, got %v", golden)
	}
	if multi == nil || multi.StatePrefix != "scenarios/wiz-devex-multi-agent" {
		t.Errorf("expected scenarios/wiz-devex-multi-agent prefix, got %v", multi)
	}
}

func TestComputeContentHash(t *testing.T) {
	tempDir := t.TempDir()
	scenarioDir := filepath.Join(tempDir, "sample-scenario")
	tfDir := filepath.Join(scenarioDir, "terraform")
	os.MkdirAll(tfDir, 0755)

	dockerfile := filepath.Join(scenarioDir, "Dockerfile")
	settings := filepath.Join(scenarioDir, "settings.json")
	tfMain := filepath.Join(tfDir, "main.tf")

	os.WriteFile(dockerfile, []byte("FROM alpine:latest"), 0644)
	os.WriteFile(settings, []byte(`{"key": "val1"}`), 0644)
	os.WriteFile(tfMain, []byte(`resource "foo" "bar" {}`), 0644)

	hash1, err := ComputeContentHash(scenarioDir)
	if err != nil {
		t.Fatalf("ComputeContentHash failed: %v", err)
	}
	if len(hash1) != 12 {
		t.Errorf("expected 12-char hash, got %s", hash1)
	}

	// 1. Changing a terraform file should NOT change the hash
	os.WriteFile(tfMain, []byte(`resource "foo" "modified" {}`), 0644)
	hash2, _ := ComputeContentHash(scenarioDir)
	if hash1 != hash2 {
		t.Errorf("expected hash to be unchanged after terraform edit, got %s vs %s", hash1, hash2)
	}

	// 2. Modifying a configuration file SHOULD change the hash
	os.WriteFile(settings, []byte(`{"key": "val2"}`), 0644)
	hash3, _ := ComputeContentHash(scenarioDir)
	if hash1 == hash3 {
		t.Errorf("expected hash to change after settings.json edit, got same hash %s", hash3)
	}

	// 3. Adding a new file SHOULD change the hash
	os.WriteFile(filepath.Join(scenarioDir, "entrypoint.sh"), []byte("#!/bin/bash"), 0755)
	hash4, _ := ComputeContentHash(scenarioDir)
	if hash3 == hash4 {
		t.Errorf("expected hash to change after adding file, got same hash %s", hash4)
	}
}
