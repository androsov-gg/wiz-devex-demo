package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"deployer-cli/pkg/ui"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"
)

// getTerraformClient creates and returns an initialized tfexec.Terraform instance.
func getTerraformClient(ctx context.Context, workingDir string) (*tfexec.Terraform, error) {
	// First check if terraform is available in PATH
	execPath, err := exec.LookPath("terraform")
	if err != nil {
		ui.Info("Terraform binary not found in PATH; locating via hc-install...")
		installer := &releases.ExactVersion{
			Product: product.Terraform,
			Version: version.Must(version.NewVersion("1.5.7")),
		}
		execPath, err = installer.Install(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to locate or install Terraform: %w", err)
		}
	}

	tf, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform-exec instance: %w", err)
	}

	return tf, nil
}

// Init initializes Terraform working directory with the remote GCS state backend using tfexec.
func Init(terraformDir, bucket, prefix string, verbose bool) error {
	ctx := context.Background()
	tf, err := getTerraformClient(ctx, terraformDir)
	if err != nil {
		return err
	}

	ui.Info(fmt.Sprintf("Initializing Terraform backend (bucket: %s, prefix: %s)...", bucket, prefix))

	var initOpts []tfexec.InitOption
	initOpts = append(initOpts, tfexec.BackendConfig(fmt.Sprintf("bucket=%s", bucket)))
	if prefix != "" {
		initOpts = append(initOpts, tfexec.BackendConfig(fmt.Sprintf("prefix=%s", prefix)))
	}
	initOpts = append(initOpts, tfexec.Reconfigure(true))

	if verbose {
		tf.SetStdout(os.Stdout)
		tf.SetStderr(os.Stderr)
	}

	if err := tf.Init(ctx, initOpts...); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}
	return nil
}

// Plan executes terraform plan using tfexec and returns a concise human-readable summary.
func Plan(terraformDir string, verbose bool) (string, error) {
	ctx := context.Background()
	tf, err := getTerraformClient(ctx, terraformDir)
	if err != nil {
		return "", err
	}

	ui.Info("Generating Terraform execution plan...")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if verbose {
		tf.SetStdout(os.Stdout)
		tf.SetStderr(os.Stderr)
	} else {
		tf.SetStdout(&stdout)
		tf.SetStderr(&stderr)
	}

	hasChanges, err := tf.Plan(ctx)
	if err != nil {
		if !verbose && stderr.Len() > 0 {
			return "", fmt.Errorf("terraform plan failed: %s (%w)", strings.TrimSpace(stderr.String()), err)
		}
		return "", fmt.Errorf("terraform plan failed: %w", err)
	}

	if !hasChanges {
		return "No changes. Your infrastructure matches the configuration.", nil
	}

	if stdout.Len() > 0 {
		lines := strings.Split(stdout.String(), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Plan:") || strings.HasPrefix(trimmed, "No changes.") {
				return trimmed, nil
			}
		}
	}

	return "Plan completed: infrastructure changes required.", nil
}

// Apply executes terraform apply using tfexec.
func Apply(terraformDir string, verbose bool) error {
	ctx := context.Background()
	tf, err := getTerraformClient(ctx, terraformDir)
	if err != nil {
		return err
	}

	ui.Info("Applying Terraform configuration...")
	tf.SetStdout(os.Stdout)
	tf.SetStderr(os.Stderr)

	if err := tf.Apply(ctx); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}
	return nil
}

// Destroy executes terraform destroy using tfexec.
func Destroy(terraformDir string, verbose bool) error {
	ctx := context.Background()
	tf, err := getTerraformClient(ctx, terraformDir)
	if err != nil {
		return err
	}

	ui.Info("Destroying Terraform resources...")
	tf.SetStdout(os.Stdout)
	tf.SetStderr(os.Stderr)

	if err := tf.Destroy(ctx); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}
	return nil
}

// GetOutputs extracts output values from the Terraform state using tfexec.
func GetOutputs(terraformDir string) (map[string]string, error) {
	ctx := context.Background()
	tf, err := getTerraformClient(ctx, terraformDir)
	if err != nil {
		return nil, err
	}

	outputs, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read terraform outputs: %w", err)
	}

	results := make(map[string]string)
	for k, v := range outputs {
		var strVal string
		if err := json.Unmarshal(v.Value, &strVal); err == nil {
			results[k] = strVal
		} else {
			results[k] = string(v.Value)
		}
	}

	return results, nil
}
