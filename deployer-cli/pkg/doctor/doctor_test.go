package doctor

import (
	"testing"

	"github.com/hashicorp/go-version"
)

func TestExtractTerraformVersion(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		expectedVer string
	}{
		{
			name:        "Standard v1.15.8",
			output:      "Terraform v1.15.8\non linux_amd64\n+ provider...",
			expectedVer: "1.15.8",
		},
		{
			name:        "Standard v1.3.0",
			output:      "Terraform v1.3.0\n",
			expectedVer: "1.3.0",
		},
		{
			name:        "Older v1.2.9",
			output:      "Terraform v1.2.9",
			expectedVer: "1.2.9",
		},
		{
			name:        "Empty output",
			output:      "",
			expectedVer: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ver, _ := ExtractTerraformVersion(tc.output)
			if ver != tc.expectedVer {
				t.Errorf("expected version %q, got %q", tc.expectedVer, ver)
			}
		})
	}
}

func TestTerraformVersionComparison(t *testing.T) {
	minVer, err := version.NewVersion("1.3.0")
	if err != nil {
		t.Fatalf("failed to create minVer: %v", err)
	}

	validVersions := []string{"1.3.0", "1.3.1", "1.5.7", "1.15.8", "2.0.0"}
	for _, vStr := range validVersions {
		v, err := version.NewVersion(vStr)
		if err != nil {
			t.Errorf("failed to parse valid version %s: %v", vStr, err)
			continue
		}
		if v.LessThan(minVer) {
			t.Errorf("version %s should be >= 1.3.0", vStr)
		}
	}

	invalidVersions := []string{"0.12.0", "1.0.0", "1.2.9"}
	for _, vStr := range invalidVersions {
		v, err := version.NewVersion(vStr)
		if err != nil {
			t.Errorf("failed to parse invalid version %s: %v", vStr, err)
			continue
		}
		if !v.LessThan(minVer) {
			t.Errorf("version %s should be < 1.3.0", vStr)
		}
	}
}
