package cloudbuild

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	artifactregistry "cloud.google.com/go/artifactregistry/apiv1"
	artifactregistrypb "cloud.google.com/go/artifactregistry/apiv1/artifactregistrypb"
	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	cloudbuildpb "cloud.google.com/go/cloudbuild/apiv1/v2/cloudbuildpb"

	"wiz-demo-platform/internal/config"
	"wiz-demo-platform/internal/models"
	"wiz-demo-platform/internal/store"
)

type CloudBuildService struct {
	cfg      *config.Config
	store    *store.Store
	arClient *artifactregistry.Client
	cbClient *cloudbuild.Client
}

func New(cfg *config.Config, s *store.Store) *CloudBuildService {
	ctx := context.Background()
	ar, err := artifactregistry.NewClient(ctx)
	if err != nil {
		log.Printf("⚠️ ArtifactRegistry client init warning: %v", err)
	}

	cb, err := cloudbuild.NewClient(ctx)
	if err != nil {
		log.Printf("⚠️ CloudBuild client init warning: %v", err)
	}

	return &CloudBuildService{
		cfg:      cfg,
		store:    s,
		arClient: ar,
		cbClient: cb,
	}
}

func (s *CloudBuildService) CheckImageExists(ctx context.Context, imageTag string) (bool, error) {
	if s.arClient == nil {
		return false, nil
	}

	// Tag format: ${region}-docker.pkg.dev/${projectId}/wiz-devex-demo-images/${imageName}:${tag}
	parts := strings.Split(imageTag, "/")
	repoAndImage := parts[len(parts)-1]
	imageName := repoAndImage
	tag := "latest"
	if strings.Contains(repoAndImage, ":") {
		subParts := strings.SplitN(repoAndImage, ":", 2)
		imageName = subParts[0]
		tag = subParts[1]
	}

	tagResourcePath := fmt.Sprintf("projects/%s/locations/%s/repositories/%s/packages/%s/tags/%s",
		s.cfg.ProjectID, s.cfg.Region, s.cfg.ArtifactRepoName, imageName, tag)

	req := &artifactregistrypb.GetTagRequest{
		Name: tagResourcePath,
	}

	resp, err := s.arClient.GetTag(ctx, req)
	if err != nil {
		return false, nil // Not found or error
	}
	return resp != nil && resp.Version != "", nil
}

func (s *CloudBuildService) TriggerCloudBuild(ctx context.Context, configName, version, targetImageURI string) (string, error) {
	buildID := fmt.Sprintf("build-%s-%s-%d", configName, version, time.Now().Unix())

	if s.cbClient == nil {
		log.Printf("[DEV MODE] Simulated Cloud Build trigger for %s:%s -> %s", configName, version, targetImageURI)
		now := time.Now().Format(time.RFC3339)
		_ = s.store.SaveBuildRecord(ctx, models.BuildRecord{
			BuildID:    buildID,
			TemplateID: configName,
			Version:    version,
			Status:     "SUCCESS",
			StartTime:  now,
			FinishTime: now,
			LogURL:     fmt.Sprintf("https://console.cloud.google.com/cloud-build/builds?project=%s", s.cfg.ProjectID),
		})
		return buildID, nil
	}

	builderSA := fmt.Sprintf("projects/%s/serviceAccounts/wiz-demo-builder-sa@%s.iam.gserviceaccount.com",
		s.cfg.ProjectID, s.cfg.ProjectID)

	rsyncCmd := fmt.Sprintf("gcloud storage rsync --recursive \"gs://%s/%s/%s\" . || gsutil -m rsync -r \"gs://%s/%s/%s\" .",
		s.cfg.ConfigBucketName, configName, version, s.cfg.ConfigBucketName, configName, version)

	buildReq := &cloudbuildpb.CreateBuildRequest{
		ProjectId: s.cfg.ProjectID,
		Build: &cloudbuildpb.Build{
			ServiceAccount: builderSA,
			Steps: []*cloudbuildpb.BuildStep{
				{
					Name:       "gcr.io/google.com/cloudsdktool/cloud-sdk:slim",
					Entrypoint: "sh",
					Args:       []string{"-c", rsyncCmd},
				},
				{
					Name: "gcr.io/cloud-builders/docker",
					Env:  []string{"DOCKER_BUILDKIT=1"},
					Args: []string{"build", "-t", targetImageURI, "."},
				},
			},
			Images: []string{targetImageURI},
			Options: &cloudbuildpb.BuildOptions{
				DefaultLogsBucketBehavior: cloudbuildpb.BuildOptions_REGIONAL_USER_OWNED_BUCKET,
			},
		},
	}

	op, err := s.cbClient.CreateBuild(ctx, buildReq)
	if err != nil {
		return "", fmt.Errorf("failed to dispatch Cloud Build: %w", err)
	}

	realBuildID := buildID
	logURL := fmt.Sprintf("https://console.cloud.google.com/cloud-build/builds?project=%s", s.cfg.ProjectID)

	if op.Metadata != nil {
		var metadata cloudbuildpb.BuildOperationMetadata
		if err := op.Metadata.UnmarshalTo(&metadata); err == nil && metadata.Build != nil {
			if metadata.Build.Id != "" {
				realBuildID = metadata.Build.Id
			}
			if metadata.Build.LogUrl != "" {
				logURL = metadata.Build.LogUrl
			}
		}
	}

	_ = s.store.SaveBuildRecord(ctx, models.BuildRecord{
		BuildID:    realBuildID,
		TemplateID: configName,
		Version:    version,
		Status:     "WORKING",
		StartTime:  time.Now().Format(time.RFC3339),
		LogURL:     logURL,
	})

	return realBuildID, nil
}

func (s *CloudBuildService) GetCloudBuildStatus(ctx context.Context, buildID string) (string, string, error) {
	if s.cbClient == nil || strings.HasPrefix(buildID, "build-") {
		return "SUCCESS", "", nil
	}

	req := &cloudbuildpb.GetBuildRequest{
		ProjectId: s.cfg.ProjectID,
		Id:        buildID,
	}

	build, err := s.cbClient.GetBuild(ctx, req)
	if err != nil {
		reqByName := &cloudbuildpb.GetBuildRequest{
			Name: fmt.Sprintf("projects/%s/locations/global/builds/%s", s.cfg.ProjectID, buildID),
		}
		build, err = s.cbClient.GetBuild(ctx, reqByName)
		if err != nil {
			return "WORKING", "", err
		}
	}

	statusMap := map[cloudbuildpb.Build_Status]string{
		cloudbuildpb.Build_QUEUED:    "QUEUED",
		cloudbuildpb.Build_WORKING:   "WORKING",
		cloudbuildpb.Build_SUCCESS:   "SUCCESS",
		cloudbuildpb.Build_FAILURE:   "FAILURE",
		cloudbuildpb.Build_CANCELLED: "CANCELLED",
		cloudbuildpb.Build_TIMEOUT:   "FAILURE",
	}

	status := statusMap[build.Status]
	if status == "" {
		status = "WORKING"
	}

	return status, build.LogUrl, nil
}
