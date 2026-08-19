package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"wiz-demo-platform/internal/catalog"
	"wiz-demo-platform/internal/cloudbuild"
	"wiz-demo-platform/internal/cloudrun"
	"wiz-demo-platform/internal/config"
	"wiz-demo-platform/internal/handlers"
	"wiz-demo-platform/internal/store"
)

//go:embed dist/*
var staticFS embed.FS

func main() {
	cfg := config.Load()

	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	st := store.New(cfg)
	cat := catalog.New(cfg)
	cr := cloudrun.New(cfg)
	cb := cloudbuild.New(cfg, st)

	h := handlers.New(cfg, cat, st, cr, cb)

	r := gin.Default()

	// CORS middleware for local Vite development
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, x-goog-authenticated-user-email")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API Routes
	api := r.Group("/api")
	{
		api.GET("/health", h.HealthCheck)
		api.GET("/catalog", h.GetCatalog)
		api.GET("/instances", h.ListInstances)
		api.POST("/instances", h.CreateInstance)
		api.GET("/instances/:id", h.GetInstance)
		api.DELETE("/instances/:id", h.DeleteInstance)
		api.POST("/instances/:id/restart", h.RestartInstance)
		api.GET("/instances/:id/logs", h.GetInstanceLogs)
		api.GET("/builds", h.GetBuildStatus)
	}

	// Demo instance redirect
	r.GET("/demos/:id", h.RedirectDemo)

	// Embedded Static Frontend SPA handler
	subFS, err := fs.Sub(staticFS, "dist")
	if err == nil {
		indexHTML, readErr := fs.ReadFile(subFS, "index.html")
		r.NoRoute(func(c *gin.Context) {
			path := strings.TrimPrefix(c.Request.URL.Path, "/")
			if path != "" {
				if f, err := subFS.Open(path); err == nil {
					defer f.Close()
					if stat, err := f.Stat(); err == nil && !stat.IsDir() {
						c.FileFromFS(path, http.FS(subFS))
						return
					}
				}
			}
			// SPA fallback: return index.html directly with 200 OK
			if readErr == nil {
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
				return
			}
			c.String(http.StatusNotFound, "404 Not Found")
		})
	}

	log.Printf("🚀 Wiz Demo Platform running on :%s (Project: %s, Region: %s)", cfg.Port, cfg.ProjectID, cfg.Region)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
