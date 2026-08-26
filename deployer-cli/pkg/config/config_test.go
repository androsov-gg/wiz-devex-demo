package config

import (
	"os"
	"path/filepath"
	"testing"

	"deployer-cli/pkg/scenario"
)

func TestValidateRegion(t *testing.T) {
	tests := []struct {
		region  string
		wantErr bool
	}{
		{"europe-west1", false},
		{"us-east5", false},
		{"asia-southeast1", false},
		{"us-central1", true},
		{"europe-west4", true},
		{"", true},
	}

	for _, tt := range tests {
		err := ValidateRegion(tt.region)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateRegion(%q) error = %v, wantErr %v", tt.region, err, tt.wantErr)
		}
	}
}

func TestInferDefaultIAPMember(t *testing.T) {
	tests := []struct {
		account string
		want    string
	}{
		{"ajkearns@google.com", "domain:ajkearns.altostrat.com"},
		{"alex@ajkearns.altostrat.com", "domain:ajkearns.altostrat.com"},
		{"ce-user@argolis.altostrat.com", "domain:argolis.altostrat.com"},
		{"user@customdomain.com", "user:user@customdomain.com"},
		{"", "domain:yourname.altostrat.com"},
	}

	for _, tt := range tests {
		got := InferDefaultIAPMember(tt.account)
		if got != tt.want {
			t.Errorf("InferDefaultIAPMember(%q) = %q, want %q", tt.account, got, tt.want)
		}
	}
}

func TestWriteAndReadTFVars(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &DeployConfig{
		ProjectID:  "test-project-123",
		Region:     "europe-west1",
		ImageTag:   "a1b2c3d4e5f6",
		IAPMembers: []string{"domain:ce.altostrat.com"},
		Scenario: &scenario.Scenario{
			Name: "wiz-code-golden-demo",
		},
	}

	if err := WriteTFVars(tempDir, cfg); err != nil {
		t.Fatalf("WriteTFVars failed: %v", err)
	}

	tfvarsFile := filepath.Join(tempDir, "terraform.auto.tfvars")
	if _, err := os.Stat(tfvarsFile); err != nil {
		t.Fatalf("tfvars file was not created: %v", err)
	}

	pID, reg, sName, imgTag, iap := ReadExistingTFVars(tempDir)
	if pID != "test-project-123" {
		t.Errorf("expected project_id test-project-123, got %s", pID)
	}
	if reg != "europe-west1" {
		t.Errorf("expected region europe-west1, got %s", reg)
	}
	if sName != "wiz-code-golden-demo" {
		t.Errorf("expected service_name wiz-code-golden-demo, got %s", sName)
	}
	if imgTag != "a1b2c3d4e5f6" {
		t.Errorf("expected image_tag a1b2c3d4e5f6, got %s", imgTag)
	}
	if len(iap) != 1 || iap[0] != "domain:ce.altostrat.com" {
		t.Errorf("expected iap member domain:ce.altostrat.com, got %v", iap)
	}
}
