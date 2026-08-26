package gcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"deployer-cli/pkg/ui"

	"cloud.google.com/go/artifactregistry/apiv1"
	"cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	"cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/run/apiv2/runpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/serviceusage/v1"
)

// RequiredAPIs is the foundational set of Google Cloud APIs needed for the build & state pipeline.
var RequiredAPIs = []string{
	"cloudbuild.googleapis.com",
	"artifactregistry.googleapis.com",
	"storage.googleapis.com",
	"iam.googleapis.com",
}

// CloudRunServiceInfo holds summary details about a deployed Cloud Run service.
type CloudRunServiceInfo struct {
	Name   string
	Region string
	URL    string
	Status string
	Image  string
}

// EnsurePrerequisites prepares all base project requirements (APIs, Artifact Registry repo, builder SA).
func EnsurePrerequisites(project, region string, verbose bool) error {
	if err := EnableAPIs(project, RequiredAPIs, verbose); err != nil {
		return fmt.Errorf("API enablement failed: %w", err)
	}
	if err := EnsureArtifactRegistry(project, region, "wiz-devex-demo-images", verbose); err != nil {
		return fmt.Errorf("Artifact Registry setup failed: %w", err)
	}
	if err := EnsureBuilderServiceAccount(project, verbose); err != nil {
		return fmt.Errorf("Builder Service Account setup failed: %w", err)
	}
	return nil
}

// EnableAPIs enables the required Google Cloud APIs idempotently using Service Usage SDK.
func EnableAPIs(project string, apis []string, verbose bool) error {
	ctx := context.Background()
	ui.Info("Enabling required GCP APIs via Service Usage API...")

	suService, err := serviceusage.NewService(ctx)
	if err != nil {
		return enableAPIsGcloud(project, apis, verbose)
	}

	req := &serviceusage.BatchEnableServicesRequest{
		ServiceIds: apis,
	}
	op, err := suService.Services.BatchEnable(fmt.Sprintf("projects/%s", project), req).Do()
	if err != nil {
		return enableAPIsGcloud(project, apis, verbose)
	}

	for !op.Done {
		time.Sleep(2 * time.Second)
		op, err = suService.Operations.Get(op.Name).Do()
		if err != nil {
			break
		}
	}

	ui.Info("Waiting 15s for Google Cloud API enablement & Service Agent propagation...")
	time.Sleep(15 * time.Second)
	return nil
}

func enableAPIsGcloud(project string, apis []string, verbose bool) error {
	args := append([]string{"services", "enable", "--project", project}, apis...)
	cmd := exec.Command("gcloud", args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gcloud services enable failed: %w", err)
	}
	ui.Info("Waiting 15s for Google Cloud API enablement & Service Agent propagation...")
	time.Sleep(15 * time.Second)
	return nil
}

// EnsureArtifactRegistry creates the Docker repository if it does not exist using Artifact Registry Go SDK.
func EnsureArtifactRegistry(project, region, repoName string, verbose bool) error {
	ctx := context.Background()
	client, err := artifactregistry.NewClient(ctx)
	if err != nil {
		return ensureArtifactRegistryGcloud(project, region, repoName, verbose)
	}
	defer client.Close()

	parent := fmt.Sprintf("projects/%s/locations/%s", project, region)
	repoPath := fmt.Sprintf("%s/repositories/%s", parent, repoName)

	_, err = client.GetRepository(ctx, &artifactregistrypb.GetRepositoryRequest{Name: repoPath})
	if err == nil {
		ui.Info(fmt.Sprintf("Artifact Registry repository '%s' already exists in %s", repoName, region))
		return nil
	}

	ui.Info(fmt.Sprintf("Creating Artifact Registry repository '%s' in %s...", repoName, region))
	req := &artifactregistrypb.CreateRepositoryRequest{
		Parent:       parent,
		RepositoryId: repoName,
		Repository: &artifactregistrypb.Repository{
			Format:      artifactregistrypb.Repository_DOCKER,
			Description: "Docker repository for code-server Cloud Run images",
		},
	}
	op, err := client.CreateRepository(ctx, req)
	if err != nil {
		return ensureArtifactRegistryGcloud(project, region, repoName, verbose)
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed creating Artifact Registry repository: %w", err)
	}

	return nil
}

func ensureArtifactRegistryGcloud(project, region, repoName string, verbose bool) error {
	checkCmd := exec.Command("gcloud", "artifacts", "repositories", "describe", repoName,
		"--location", region, "--project", project, "--format=value(name)")
	if err := checkCmd.Run(); err == nil {
		ui.Info(fmt.Sprintf("Artifact Registry repository '%s' already exists in %s", repoName, region))
		return nil
	}

	args := []string{
		"artifacts", "repositories", "create", repoName,
		"--repository-format=docker",
		"--location", region,
		"--project", project,
		"--description=Docker repository for code-server Cloud Run images",
	}
	cmd := exec.Command("gcloud", args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// EnsureBuilderServiceAccount ensures the Cloud Build builder SA and policy bindings exist using IAM & Resource Manager SDKs.
func EnsureBuilderServiceAccount(project string, verbose bool) error {
	ctx := context.Background()
	saName := "cloudbuild-builder-sa"
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saName, project)

	iamService, err := iam.NewService(ctx)
	crmService, crmErr := cloudresourcemanager.NewService(ctx)

	if err != nil || crmErr != nil {
		return ensureBuilderServiceAccountGcloud(project, verbose)
	}

	fullSAName := fmt.Sprintf("projects/%s/serviceAccounts/%s", project, saEmail)
	_, err = iamService.Projects.ServiceAccounts.Get(fullSAName).Do()
	if err != nil {
		ui.Info(fmt.Sprintf("Creating Service Account '%s'...", saName))
		createReq := &iam.CreateServiceAccountRequest{
			AccountId: saName,
			ServiceAccount: &iam.ServiceAccount{
				DisplayName: "Cloud Build Dedicated Builder SA",
			},
		}
		_, err = iamService.Projects.ServiceAccounts.Create(fmt.Sprintf("projects/%s", project), createReq).Do()
		if err != nil {
			return ensureBuilderServiceAccountGcloud(project, verbose)
		}
		ui.Info("Waiting 15s for Service Account propagation...")
		time.Sleep(15 * time.Second)
	} else {
		ui.Info(fmt.Sprintf("Builder Service Account '%s' already exists", saEmail))
	}

	roles := []string{
		"roles/storage.admin",
		"roles/storage.objectAdmin",
		"roles/artifactregistry.writer",
		"roles/logging.logWriter",
	}

	policy, err := crmService.Projects.GetIamPolicy(project, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		return ensureBuilderServiceAccountGcloud(project, verbose)
	}

	member := fmt.Sprintf("serviceAccount:%s", saEmail)
	for _, role := range roles {
		found := false
		for _, b := range policy.Bindings {
			if b.Role == role {
				for _, m := range b.Members {
					if m == member {
						found = true
						break
					}
				}
				if !found {
					b.Members = append(b.Members, member)
					found = true
				}
				break
			}
		}
		if !found {
			policy.Bindings = append(policy.Bindings, &cloudresourcemanager.Binding{
				Role:    role,
				Members: []string{member},
			})
		}
	}

	setReq := &cloudresourcemanager.SetIamPolicyRequest{Policy: policy}
	_, err = crmService.Projects.SetIamPolicy(project, setReq).Do()
	if err != nil {
		return ensureBuilderServiceAccountGcloud(project, verbose)
	}

	return nil
}

func ensureBuilderServiceAccountGcloud(project string, verbose bool) error {
	saName := "cloudbuild-builder-sa"
	saEmail := fmt.Sprintf("%s@%s.iam.gserviceaccount.com", saName, project)

	checkCmd := exec.Command("gcloud", "iam", "service-accounts", "describe", saEmail, "--project", project)
	if err := checkCmd.Run(); err != nil {
		createArgs := []string{
			"iam", "service-accounts", "create", saName,
			"--display-name=Cloud Build Dedicated Builder SA",
			"--project", project,
		}
		cmd := exec.Command("gcloud", createArgs...)
		if verbose {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		if err := cmd.Run(); err != nil {
			return err
		}
		ui.Info("Waiting 15s for Service Account propagation...")
		time.Sleep(15 * time.Second)
	}

	roles := []string{
		"roles/storage.admin",
		"roles/storage.objectAdmin",
		"roles/artifactregistry.writer",
		"roles/logging.logWriter",
	}

	for _, role := range roles {
		bindArgs := []string{
			"projects", "add-iam-policy-binding", project,
			fmt.Sprintf("--member=serviceAccount:%s", saEmail),
			fmt.Sprintf("--role=%s", role),
			"--quiet",
		}
		cmd := exec.Command("gcloud", bindArgs...)
		if err := cmd.Run(); err != nil {
			time.Sleep(5 * time.Second)
			_ = exec.Command("gcloud", bindArgs...).Run()
		}
	}

	return nil
}

// ImageExists checks whether a container image tag already exists in Artifact Registry using SDK.
func ImageExists(imageTag string) bool {
	ctx := context.Background()
	parts := strings.Split(imageTag, "/")
	if len(parts) < 3 {
		return imageExistsGcloud(imageTag)
	}

	host := parts[0]
	project := parts[1]
	repoPkg := strings.Join(parts[2:], "/")

	location := strings.TrimSuffix(host, "-docker.pkg.dev")
	var repo, pkgTag string
	subparts := strings.SplitN(repoPkg, "/", 2)
	if len(subparts) == 2 {
		repo = subparts[0]
		pkgTag = subparts[1]
	} else {
		return imageExistsGcloud(imageTag)
	}

	pkgTagParts := strings.SplitN(pkgTag, ":", 2)
	if len(pkgTagParts) != 2 {
		return imageExistsGcloud(imageTag)
	}
	pkg := pkgTagParts[0]
	tag := pkgTagParts[1]

	client, err := artifactregistry.NewClient(ctx)
	if err != nil {
		return imageExistsGcloud(imageTag)
	}
	defer client.Close()

	tagName := fmt.Sprintf("projects/%s/locations/%s/repositories/%s/packages/%s/tags/%s", project, location, repo, pkg, tag)
	_, err = client.GetTag(ctx, &artifactregistrypb.GetTagRequest{Name: tagName})
	return err == nil
}

func imageExistsGcloud(imageTag string) bool {
	cmd := exec.Command("gcloud", "artifacts", "docker", "images", "describe", imageTag, "--format=value(image_summary.digest)")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) != "" {
		return true
	}
	return false
}

// ResolveStateBucket checks if an existing state bucket is present in the project using Storage SDK.
func ResolveStateBucket(project string) string {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err == nil {
		defer client.Close()
		legacyBucket := fmt.Sprintf("%s-wiz-ide-demo-platform-tfstate", project)
		if _, err := client.Bucket(legacyBucket).Attrs(ctx); err == nil {
			return legacyBucket
		}
		newBucket := fmt.Sprintf("%s-wiz-devex-demo-tf-state", project)
		if _, err := client.Bucket(newBucket).Attrs(ctx); err == nil {
			return newBucket
		}
	}

	return fmt.Sprintf("%s-wiz-devex-demo-tf-state", project)
}

// EnsureStateBucket creates the GCS state bucket if it does not already exist using Storage SDK.
func EnsureStateBucket(project, region, bucketName string, verbose bool) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return ensureStateBucketGcloud(project, region, bucketName, verbose)
	}
	defer client.Close()

	bucket := client.Bucket(bucketName)
	if _, err := bucket.Attrs(ctx); err == nil {
		ui.Info(fmt.Sprintf("Terraform state bucket 'gs://%s' already exists", bucketName))
		return nil
	}

	ui.Info(fmt.Sprintf("Creating state bucket 'gs://%s' in %s...", bucketName, region))
	attrs := &storage.BucketAttrs{
		Location: region,
		UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
			Enabled: true,
		},
	}
	if err := bucket.Create(ctx, project, attrs); err != nil {
		return ensureStateBucketGcloud(project, region, bucketName, verbose)
	}

	return nil
}

func ensureStateBucketGcloud(project, region, bucketName string, verbose bool) error {
	checkCmd := exec.Command("gcloud", "storage", "buckets", "describe", fmt.Sprintf("gs://%s", bucketName), "--project", project)
	if err := checkCmd.Run(); err == nil {
		ui.Info(fmt.Sprintf("Terraform state bucket 'gs://%s' already exists", bucketName))
		return nil
	}

	args := []string{
		"storage", "buckets", "create", fmt.Sprintf("gs://%s", bucketName),
		"--project", project,
		"--location", region,
		"--uniform-bucket-level-access",
	}
	cmd := exec.Command("gcloud", args...)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

// SubmitCloudBuild runs gcloud builds submit for the scenario container image.
func SubmitCloudBuild(project, region, scenarioDir, imageTag, latestTag string, verbose bool) error {
	gcloudIgnorePath := filepath.Join(scenarioDir, ".gcloudignore")
	if _, err := os.Stat(gcloudIgnorePath); os.IsNotExist(err) {
		_ = os.WriteFile(gcloudIgnorePath, []byte(".gcloudignore\n.git\n.gitignore\nterraform/\n.terraform/\n*.tf\n*.tfstate*\n*.tfvars\n"), 0644)
	}

	saEmail := fmt.Sprintf("projects/%s/serviceAccounts/cloudbuild-builder-sa@%s.iam.gserviceaccount.com", project, project)
	cloudBuildConfig := fmt.Sprintf("%s/cloudbuild.yaml", scenarioDir)

	substitutions := fmt.Sprintf("_IMAGE_TAG=%s,_IMAGE_LATEST_TAG=%s", imageTag, latestTag)
	args := []string{
		"builds", "submit",
		"--project", project,
		"--region", region,
		fmt.Sprintf("--service-account=%s", saEmail),
		fmt.Sprintf("--config=%s", cloudBuildConfig),
		fmt.Sprintf("--substitutions=%s", substitutions),
		scenarioDir,
	}

	ui.Info(fmt.Sprintf("Submitting Cloud Build for image tag: %s", imageTag))
	cmd := exec.Command("gcloud", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ListCloudRunServices queries Cloud Run services in the project and region using Cloud Run SDK.
func ListCloudRunServices(project, region string) ([]CloudRunServiceInfo, error) {
	ctx := context.Background()
	client, err := run.NewServicesClient(ctx)
	if err != nil {
		return listCloudRunServicesGcloud(project, region)
	}
	defer client.Close()

	parent := fmt.Sprintf("projects/%s/locations/%s", project, region)
	it := client.ListServices(ctx, &runpb.ListServicesRequest{Parent: parent})

	var services []CloudRunServiceInfo
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return listCloudRunServicesGcloud(project, region)
		}

		nameParts := strings.Split(resp.Name, "/")
		shortName := nameParts[len(nameParts)-1]

		image := ""
		if resp.Template != nil && len(resp.Template.Containers) > 0 {
			image = resp.Template.Containers[0].Image
		}

		services = append(services, CloudRunServiceInfo{
			Name:   shortName,
			Region: region,
			URL:    resp.Uri,
			Status: "Ready",
			Image:  image,
		})
	}

	return services, nil
}

func listCloudRunServicesGcloud(project, region string) ([]CloudRunServiceInfo, error) {
	args := []string{
		"run", "services", "list",
		"--project", project,
		"--region", region,
		"--format=value(metadata.name,status.url,status.conditions[0].status,spec.template.spec.containers[0].image)",
	}

	cmd := exec.Command("gcloud", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var services []CloudRunServiceInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			svc := CloudRunServiceInfo{
				Name:   fields[0],
				URL:    fields[1],
				Region: region,
			}
			if len(fields) >= 3 {
				svc.Status = fields[2]
			}
			if len(fields) >= 4 {
				svc.Image = fields[3]
			}
			services = append(services, svc)
		}
	}

	return services, nil
}

// DeleteArtifactRegistry removes the Artifact Registry repository using SDK.
func DeleteArtifactRegistry(project, region, repoName string) error {
	ctx := context.Background()
	client, err := artifactregistry.NewClient(ctx)
	if err == nil {
		defer client.Close()
		repoPath := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", project, region, repoName)
		op, err := client.DeleteRepository(ctx, &artifactregistrypb.DeleteRepositoryRequest{Name: repoPath})
		if err == nil {
			_ = op.Wait(ctx)
			return nil
		}
	}

	args := []string{
		"artifacts", "repositories", "delete", repoName,
		"--location", region,
		"--project", project,
		"--quiet",
	}
	return exec.Command("gcloud", args...).Run()
}

// DeleteBuilderServiceAccount removes the dedicated builder SA using SDK.
func DeleteBuilderServiceAccount(project string) error {
	ctx := context.Background()
	saEmail := fmt.Sprintf("cloudbuild-builder-sa@%s.iam.gserviceaccount.com", project)
	iamService, err := iam.NewService(ctx)
	if err == nil {
		fullSAName := fmt.Sprintf("projects/%s/serviceAccounts/%s", project, saEmail)
		_, err = iamService.Projects.ServiceAccounts.Delete(fullSAName).Do()
		if err == nil {
			return nil
		}
	}

	args := []string{
		"iam", "service-accounts", "delete", saEmail,
		"--project", project,
		"--quiet",
	}
	return exec.Command("gcloud", args...).Run()
}
