package catalog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"wiz-demo-platform/internal/config"
	"wiz-demo-platform/internal/models"
)

type CatalogService struct {
	cfg           *config.Config
	storageClient *storage.Client
	mu            sync.RWMutex
	cached        []models.DemoTemplate
	cacheExpiry   time.Time
}

func New(cfg *config.Config) *CatalogService {
	ctx := context.Background()
	sc, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("⚠️ Storage client init warning: %v", err)
	}

	return &CatalogService{
		cfg:           cfg,
		storageClient: sc,
	}
}

type yamlCatalogItem struct {
	Title       string            `yaml:"title"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Category    string            `yaml:"category"`
	Module      string            `yaml:"module"`
	Icon        string            `yaml:"icon"`
	Port        int               `yaml:"port"`
	CPU         string            `yaml:"cpu"`
	Memory      string            `yaml:"memory"`
	DefaultEnv  map[string]string `yaml:"defaultEnv"`
	DocsURL     string            `yaml:"docsUrl"`
	Versions    yaml.Node         `yaml:"versions"`
}

func parseVersions(node *yaml.Node) []models.DemoVersion {
	if node == nil || node.Kind == 0 {
		return []models.DemoVersion{{Version: "1.0.0", Description: ""}}
	}

	var versions []models.DemoVersion

	// If sequence (array)
	if node.Kind == yaml.SequenceNode {
		for _, item := range node.Content {
			var v models.DemoVersion
			if err := item.Decode(&v); err == nil && v.Version != "" {
				versions = append(versions, v)
			}
		}
	} else if node.Kind == yaml.MappingNode {
		// If map: "1.0.0": { description: "..." } or "1.0.0": "description"
		for i := 0; i < len(node.Content); i += 2 {
			verKey := node.Content[i].Value
			valNode := node.Content[i+1]

			if valNode.Kind == yaml.ScalarNode {
				versions = append(versions, models.DemoVersion{
					Version:     verKey,
					Description: valNode.Value,
				})
			} else if valNode.Kind == yaml.MappingNode {
				var desc string
				for j := 0; j < len(valNode.Content); j += 2 {
					if valNode.Content[j].Value == "description" {
						desc = valNode.Content[j+1].Value
					}
				}
				versions = append(versions, models.DemoVersion{
					Version:     verKey,
					Description: desc,
				})
			}
		}
	}

	if len(versions) == 0 {
		versions = append(versions, models.DemoVersion{Version: "1.0.0", Description: ""})
	}

	// Sort descending by semver
	sort.Slice(versions, func(i, j int) bool {
		vA, errA := semver.NewVersion(versions[i].Version)
		vB, errB := semver.NewVersion(versions[j].Version)
		if errA == nil && errB == nil {
			return vA.GreaterThan(vB)
		}
		return versions[i].Version > versions[j].Version
	})

	return versions
}

func (s *CatalogService) GetCatalog(ctx context.Context, forceRefresh bool) ([]models.DemoTemplate, error) {
	s.mu.RLock()
	if !forceRefresh && s.cached != nil && time.Now().Before(s.cacheExpiry) {
		res := s.cached
		s.mu.RUnlock()
		return res, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !forceRefresh && s.cached != nil && time.Now().Before(s.cacheExpiry) {
		return s.cached, nil
	}

	yamlContent, err := s.fetchCatalogYAML(ctx)
	if err != nil {
		if s.cached != nil {
			return s.cached, nil
		}
		return nil, err
	}

	var rawCatalog map[string]yamlCatalogItem
	if err := yaml.Unmarshal([]byte(yamlContent), &rawCatalog); err != nil {
		return nil, fmt.Errorf("failed to parse catalog YAML: %w", err)
	}

	var templates []models.DemoTemplate
	for key, val := range rawCatalog {
		versions := parseVersions(&val.Versions)
		primaryVersion := "1.0.0"
		if len(versions) > 0 {
			primaryVersion = versions[0].Version
		}

		desc := val.Description
		if desc == "" && len(versions) > 0 {
			desc = versions[0].Description
		}

		title := val.Title
		if title == "" {
			title = val.Name
		}
		if title == "" {
			title = key
		}

		icon := val.Icon
		if icon == "" {
			icon = "Code2"
		}

		port := val.Port
		if port == 0 {
			port = 8080
		}

		cpu := val.CPU
		if cpu == "" {
			cpu = "2"
		}

		memory := val.Memory
		if memory == "" {
			memory = "4Gi"
		}

		templates = append(templates, models.DemoTemplate{
			ID:              key,
			Name:            title,
			Title:           title,
			Description:     desc,
			Category:        val.Category,
			Module:          val.Module,
			Icon:            icon,
			ContainerImage:  fmt.Sprintf("%s:%s", key, primaryVersion),
			BuildSourcePath: fmt.Sprintf("configurations/%s/%s", key, primaryVersion),
			Port:            port,
			CPU:             cpu,
			Memory:          memory,
			DefaultEnv:      val.DefaultEnv,
			DocsURL:         val.DocsURL,
			Versions:        versions,
			SelectedVersion: primaryVersion,
		})
	}

	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	s.cached = templates
	s.cacheExpiry = time.Now().Add(30 * time.Second)
	return templates, nil
}

func (s *CatalogService) fetchCatalogYAML(ctx context.Context) (string, error) {
	// 1. Try pulling from GCS
	if s.storageClient != nil && s.cfg.ConfigBucketName != "" {
		rc, err := s.storageClient.Bucket(s.cfg.ConfigBucketName).Object("index.yaml").NewReader(ctx)
		if err == nil {
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err == nil && len(data) > 0 {
				return string(data), nil
			}
		}
	}

	// 2. Fallback to local files
	candidatePaths := []string{
		"configurations/index.yaml",
		"../configurations/index.yaml",
		"../../configurations/index.yaml",
	}

	for _, p := range candidatePaths {
		absPath, _ := filepath.Abs(p)
		if data, err := os.ReadFile(absPath); err == nil && len(data) > 0 {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("could not find or read index.yaml from GCS or local filesystem")
}

func (s *CatalogService) GetTemplate(ctx context.Context, id, version string) (*models.DemoTemplate, error) {
	catalog, err := s.GetCatalog(ctx, false)
	if err != nil {
		return nil, err
	}

	for _, tpl := range catalog {
		if tpl.ID == id {
			if version != "" {
				for _, v := range tpl.Versions {
					if v.Version == version {
						tpl.SelectedVersion = v.Version
						if v.Description != "" {
							tpl.Description = v.Description
						}
						tpl.ContainerImage = fmt.Sprintf("%s:%s", tpl.ID, v.Version)
						tpl.BuildSourcePath = fmt.Sprintf("configurations/%s/%s", tpl.ID, v.Version)
						return &tpl, nil
					}
				}
				tpl.SelectedVersion = version
				tpl.ContainerImage = fmt.Sprintf("%s:%s", tpl.ID, version)
				tpl.BuildSourcePath = fmt.Sprintf("configurations/%s/%s", tpl.ID, version)
			}
			return &tpl, nil
		}
	}

	return nil, fmt.Errorf("template %q not found", id)
}

func (s *CatalogService) GetFullImageURI(imageName string) string {
	return fmt.Sprintf("%s/%s", s.cfg.ArtifactRepoImages, imageName)
}
