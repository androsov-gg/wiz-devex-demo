package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"wiz-demo-platform/internal/catalog"
	"wiz-demo-platform/internal/cloudbuild"
	"wiz-demo-platform/internal/cloudrun"
	"wiz-demo-platform/internal/config"
	"wiz-demo-platform/internal/models"
	"wiz-demo-platform/internal/store"
)

const IAPPropagationDelay = 90 * time.Second

type Handler struct {
	cfg        *config.Config
	catalogSvc *catalog.CatalogService
	store      *store.Store
	cloudRun   *cloudrun.CloudRunService
	cloudBuild *cloudbuild.CloudBuildService
}

func New(
	cfg *config.Config,
	cat *catalog.CatalogService,
	st *store.Store,
	cr *cloudrun.CloudRunService,
	cb *cloudbuild.CloudBuildService,
) *Handler {
	return &Handler{
		cfg:        cfg,
		catalogSvc: cat,
		store:      st,
		cloudRun:   cr,
		cloudBuild: cb,
	}
}

func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) GetCatalog(c *gin.Context) {
	forceRefresh := c.Query("refresh") == "true"
	cat, err := h.catalogSvc.GetCatalog(c.Request.Context(), forceRefresh)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"catalog": cat})
}

func (h *Handler) ListInstances(c *gin.Context) {
	ctx := c.Request.Context()
	instances, err := h.store.ListInstances(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updated []models.DemoInstance
	for _, inst := range instances {
		// 1. In-flight Cloud Build reconciliation
		if inst.Status == models.StatusBuildingImage {
			tpl, err := h.catalogSvc.GetTemplate(ctx, inst.TemplateID, inst.Version)
			if err == nil && tpl != nil {
				targetImageURI := h.catalogSvc.GetFullImageURI(tpl.ContainerImage)
				imageExists, _ := h.cloudBuild.CheckImageExists(ctx, targetImageURI)
				isSuccess := imageExists

				if !isSuccess && inst.BuildID != "" {
					status, _, _ := h.cloudBuild.GetCloudBuildStatus(ctx, inst.BuildID)
					isSuccess = status == "SUCCESS"
					if status == "FAILURE" || status == "CANCELLED" {
						errMsg := "Cloud Build failed to compile container image."
						_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusFailed, map[string]interface{}{"errorMessage": errMsg})
						inst.Status = models.StatusFailed
						inst.ErrorMessage = errMsg
						updated = append(updated, inst)
						continue
					}
				}

				if isSuccess {
					_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusProvisioning, nil)
					inst.Status = models.StatusProvisioning
					svcURL, err := h.cloudRun.DeployDemoService(ctx, inst, *tpl, targetImageURI)
					if err == nil {
						readyTS := time.Now().Format(time.RFC3339)
						_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusProvisioning, map[string]interface{}{
							"cloudRunReadyAt": readyTS,
							"serviceUrl":      svcURL,
						})
						inst.CloudRunReadyAt = readyTS
						inst.ServiceURL = svcURL
					}
					updated = append(updated, inst)
					continue
				}
			}
		}

		// 2. Live Cloud Run service reconciliation
		if inst.Status == models.StatusRunning || inst.Status == models.StatusProvisioning || inst.Status == models.StatusRestarting {
			liveStatus, svcURL, _, err := h.cloudRun.GetDemoServiceStatus(ctx, inst.ID)
			if err == nil {
				if liveStatus == models.StatusFailed {
					_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusFailed, map[string]interface{}{"serviceUrl": svcURL})
					inst.Status = models.StatusFailed
					inst.ServiceURL = svcURL
					updated = append(updated, inst)
					continue
				}

				if liveStatus == models.StatusStopped {
					_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusStopped, nil)
					inst.Status = models.StatusStopped
					updated = append(updated, inst)
					continue
				}

				if liveStatus == models.StatusRunning {
					readyAt := inst.CloudRunReadyAt
					if readyAt == "" {
						readyAt = time.Now().Format(time.RFC3339)
						_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusProvisioning, map[string]interface{}{
							"cloudRunReadyAt": readyAt,
							"serviceUrl":      svcURL,
						})
						inst.CloudRunReadyAt = readyAt
						inst.ServiceURL = svcURL
					}

					readyTime, _ := time.Parse(time.RFC3339, readyAt)
					elapsed := time.Since(readyTime)

					if elapsed >= IAPPropagationDelay {
						if inst.Status != models.StatusRunning || (svcURL != "" && svcURL != inst.ServiceURL) {
							_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusRunning, map[string]interface{}{
								"serviceUrl": svcURL,
							})
						}
						inst.Status = models.StatusRunning
						inst.ServiceURL = svcURL
					} else {
						inst.Status = models.StatusProvisioning
					}
					updated = append(updated, inst)
					continue
				}
			}
		}

		updated = append(updated, inst)
	}

	c.JSON(http.StatusOK, gin.H{"instances": updated})
}

type CreateInstanceRequest struct {
	TemplateID string `json:"templateId" binding:"required"`
	Version    string `json:"version"`
	CustomName string `json:"customName"`
}

func (h *Handler) CreateInstance(c *gin.Context) {
	ctx := c.Request.Context()
	var req CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tpl, err := h.catalogSvc.GetTemplate(ctx, req.TemplateID, req.Version)
	if err != nil || tpl == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Template %q not found.", req.TemplateID)})
		return
	}

	selectedVer := tpl.SelectedVersion
	if selectedVer == "" {
		selectedVer = req.Version
	}
	if selectedVer == "" {
		selectedVer = "1.0.0"
	}

	rawIAPEmail := c.GetHeader("x-goog-authenticated-user-email")
	ownerEmail := strings.TrimPrefix(rawIAPEmail, "accounts.google.com:")
	if ownerEmail == "" {
		ownerEmail = "ajkearns@altostrat.com"
	}
	ownerName := strings.Split(ownerEmail, "@")[0]

	randomBytes := make([]byte, 3)
	_, _ = rand.Read(randomBytes)
	randomSuffix := hex.EncodeToString(randomBytes)[:5]

	instanceID := fmt.Sprintf("%s-%s", req.TemplateID, randomSuffix)
	if req.CustomName != "" {
		instanceID = fmt.Sprintf("%s-%s", strings.ToLower(req.CustomName), randomSuffix)
	}

	now := time.Now().Format(time.RFC3339)
	targetImageURI := h.catalogSvc.GetFullImageURI(tpl.ContainerImage)

	imageExists, _ := h.cloudBuild.CheckImageExists(ctx, targetImageURI)
	if !imageExists {
		buildID, err := h.cloudBuild.TriggerCloudBuild(ctx, tpl.ID, selectedVer, targetImageURI)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Cloud Build trigger failed: %v", err)})
			return
		}

		inst := models.DemoInstance{
			ID:           instanceID,
			TemplateID:   tpl.ID,
			TemplateName: tpl.Name,
			Version:      selectedVer,
			OwnerEmail:   ownerEmail,
			OwnerName:    ownerName,
			Status:       models.StatusBuildingImage,
			CreatedAt:    now,
			Port:         tpl.Port,
			BuildID:      buildID,
		}

		_ = h.store.SaveInstance(ctx, inst)
		c.JSON(http.StatusOK, gin.H{"instance": inst, "status": "BUILDING_IMAGE", "buildId": buildID})
		return
	}

	inst := models.DemoInstance{
		ID:           instanceID,
		TemplateID:   tpl.ID,
		TemplateName: tpl.Name,
		Version:      selectedVer,
		OwnerEmail:   ownerEmail,
		OwnerName:    ownerName,
		Status:       models.StatusProvisioning,
		CreatedAt:    now,
		Port:         tpl.Port,
	}

	_ = h.store.SaveInstance(ctx, inst)

	// Asynchronous Cloud Run deployment
	go func() {
		bgCtx := context.Background()
		svcURL, err := h.cloudRun.DeployDemoService(bgCtx, inst, *tpl, targetImageURI)
		if err != nil {
			log.Printf("Async deploy error for %s: %v", inst.ID, err)
			_ = h.store.UpdateInstanceStatus(bgCtx, inst.ID, models.StatusFailed, map[string]interface{}{"errorMessage": err.Error()})
			return
		}
		readyTS := time.Now().Format(time.RFC3339)
		_ = h.store.UpdateInstanceStatus(bgCtx, inst.ID, models.StatusProvisioning, map[string]interface{}{
			"cloudRunReadyAt": readyTS,
			"serviceUrl":      svcURL,
		})
	}()

	c.JSON(http.StatusOK, gin.H{"instance": inst, "status": "PROVISIONING"})
}

func (h *Handler) GetInstance(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	inst, err := h.store.GetInstance(ctx, id)
	if err != nil || inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Instance not found."})
		return
	}

	if inst.Status == models.StatusRunning || inst.Status == models.StatusProvisioning || inst.Status == models.StatusRestarting {
		liveStatus, svcURL, _, err := h.cloudRun.GetDemoServiceStatus(ctx, id)
		if err == nil {
			if liveStatus == models.StatusFailed {
				_ = h.store.UpdateInstanceStatus(ctx, id, models.StatusFailed, map[string]interface{}{"serviceUrl": svcURL})
				inst.Status = models.StatusFailed
				inst.ServiceURL = svcURL
			} else if liveStatus == models.StatusStopped {
				_ = h.store.UpdateInstanceStatus(ctx, id, models.StatusStopped, nil)
				inst.Status = models.StatusStopped
			} else if liveStatus == models.StatusRunning {
				readyAt := inst.CloudRunReadyAt
				if readyAt == "" {
					readyAt = time.Now().Format(time.RFC3339)
					_ = h.store.UpdateInstanceStatus(ctx, id, models.StatusProvisioning, map[string]interface{}{
						"cloudRunReadyAt": readyAt,
						"serviceUrl":      svcURL,
					})
					inst.CloudRunReadyAt = readyAt
					inst.ServiceURL = svcURL
				}

				readyTime, _ := time.Parse(time.RFC3339, readyAt)
				if time.Since(readyTime) >= IAPPropagationDelay {
					if inst.Status != models.StatusRunning || (svcURL != "" && svcURL != inst.ServiceURL) {
						_ = h.store.UpdateInstanceStatus(ctx, id, models.StatusRunning, map[string]interface{}{
							"serviceUrl": svcURL,
						})
					}
					inst.Status = models.StatusRunning
					inst.ServiceURL = svcURL
				} else {
					inst.Status = models.StatusProvisioning
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"instance": inst})
}

func (h *Handler) DeleteInstance(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	inst, err := h.store.GetInstance(ctx, id)
	if err != nil || inst == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Instance not found."})
		return
	}

	_ = h.store.UpdateInstanceStatus(ctx, id, models.StatusStopping, nil)
	_ = h.cloudRun.DeleteDemoService(ctx, id)
	_ = h.store.DeleteInstance(ctx, id)

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Instance %s torn down successfully.", id)})
}

func (h *Handler) RestartInstance(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	inst, _ := h.store.GetInstance(ctx, id)
	_ = h.store.UpdateInstanceStatus(ctx, id, models.StatusRestarting, map[string]interface{}{"cloudRunReadyAt": ""})

	ownerEmail := ""
	if inst != nil {
		ownerEmail = inst.OwnerEmail
	}

	go func() {
		bgCtx := context.Background()
		err := h.cloudRun.RestartDemoService(bgCtx, id, ownerEmail)
		if err != nil {
			log.Printf("Restart failed for %s: %v", id, err)
			_ = h.store.UpdateInstanceStatus(bgCtx, id, models.StatusFailed, map[string]interface{}{
				"errorMessage": fmt.Sprintf("Restart failed: %v", err),
			})
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Instance %s restart initiated.", id),
		"status":  "RESTARTING",
	})
}

func (h *Handler) GetInstanceLogs(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	lines := 100
	if l := c.Query("lines"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			lines = val
		}
	}

	logs, err := h.cloudRun.GetDemoServiceLogs(ctx, id, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (h *Handler) GetBuildStatus(c *gin.Context) {
	ctx := c.Request.Context()
	buildID := c.Query("buildId")
	instanceID := c.Query("instanceId")

	if buildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "buildId query parameter is required."})
		return
	}

	status, logURL, _ := h.cloudBuild.GetCloudBuildStatus(ctx, buildID)

	// Reconcile instance if attached
	instances, _ := h.store.ListInstances(ctx)
	for _, inst := range instances {
		if (instanceID != "" && inst.ID == instanceID) || inst.BuildID == buildID {
			if status == "SUCCESS" && inst.Status == models.StatusBuildingImage {
				tpl, err := h.catalogSvc.GetTemplate(ctx, inst.TemplateID, inst.Version)
				if err == nil && tpl != nil {
					targetImageURI := h.catalogSvc.GetFullImageURI(tpl.ContainerImage)
					_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusProvisioning, nil)
					svcURL, err := h.cloudRun.DeployDemoService(ctx, inst, *tpl, targetImageURI)
					if err == nil {
						readyTS := time.Now().Format(time.RFC3339)
						_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusProvisioning, map[string]interface{}{
							"cloudRunReadyAt": readyTS,
							"serviceUrl":      svcURL,
						})
					}
				}
			} else if status == "FAILURE" || status == "CANCELLED" {
				_ = h.store.UpdateInstanceStatus(ctx, inst.ID, models.StatusFailed, map[string]interface{}{
					"errorMessage": "Cloud Build failed to compile the container image.",
				})
			}
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": status, "logUrl": logURL})
}

func (h *Handler) RedirectDemo(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	inst, err := h.store.GetInstance(ctx, id)
	if err == nil && inst != nil && inst.Status == models.StatusRunning && inst.ServiceURL != "" {
		c.Redirect(http.StatusFound, inst.ServiceURL)
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Demo instance not found or not yet ready."})
}
