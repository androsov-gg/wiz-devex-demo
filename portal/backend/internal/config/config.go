package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	ProjectID          string
	Region             string
	ProjectNumber      string
	FirestoreDBID      string
	ConfigBucketName   string
	DemoRunnerSAEmail  string
	IAPMembers         []string
	Port               string
	ArtifactRepoName   string
	ArtifactRepoImages string
}

func Load() *Config {
	projectID := getEnv("GOOGLE_CLOUD_PROJECT", getEnv("PROJECT_ID", "wiz-code-demo"))
	region := getEnv("GOOGLE_CLOUD_LOCATION", getEnv("REGION", "europe-west1"))

	serviceName := getEnv("SERVICE_NAME", "wiz-ide-demo-platform")
	artifactRepoName := getEnv("ARTIFACT_REPO_NAME", serviceName)
	firestoreDBID := getEnv("FIRESTORE_DATABASE_ID", fmt.Sprintf("%s-db", serviceName))
	configBucket := getEnv("CONFIG_BUCKET_NAME", fmt.Sprintf("%s-%s-configs", projectID, serviceName))

	runnerSA := getEnv("DEMO_RUNNER_SA_EMAIL", fmt.Sprintf("wiz-demo-runner-sa@%s.iam.gserviceaccount.com", projectID))
	port := getEnv("PORT", "3000")

	var iapMembers []string
	if rawMembers := os.Getenv("IAP_MEMBERS"); rawMembers != "" {
		for _, m := range strings.Split(rawMembers, ",") {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				iapMembers = append(iapMembers, trimmed)
			}
		}
	}

	projectNumber := getEnv("PROJECT_NUMBER", "")
	if projectNumber == "" || projectNumber == "1234567890" {
		projectNumber = resolveProjectNumber()
	}

	return &Config{
		ProjectID:          projectID,
		Region:             region,
		ProjectNumber:      projectNumber,
		FirestoreDBID:      firestoreDBID,
		ConfigBucketName:   configBucket,
		DemoRunnerSAEmail:  runnerSA,
		IAPMembers:         iapMembers,
		Port:               port,
		ArtifactRepoName:   artifactRepoName,
		ArtifactRepoImages: fmt.Sprintf("%s-docker.pkg.dev/%s/%s", region, projectID, artifactRepoName),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func resolveProjectNumber() string {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/numeric-project-id", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			return strings.TrimSpace(string(body))
		}
	}
	return ""
}
