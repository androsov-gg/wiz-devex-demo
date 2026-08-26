# ------------------------------------------------------------------------------
# 1. Root Providers, APIs & Data Sources
# ------------------------------------------------------------------------------
terraform {
  required_version = ">= 1.3.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.30.0"
    }
  }

  backend "gcs" {
    # Dynamically configured per GCP project during terraform init:
    # terraform init -backend-config="bucket=${PROJECT_ID}-wiz-devex-demo-tf-state"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "aiplatform.googleapis.com",
    "storage.googleapis.com",
    "iam.googleapis.com",
    "cloudbuild.googleapis.com",
    "artifactregistry.googleapis.com",
    "iap.googleapis.com"
  ])
  service            = each.key
  disable_on_destroy = false
}

data "google_project" "project" {
  project_id = var.project_id
}

# ------------------------------------------------------------------------------
# 2. Service Accounts & IAM Configurations
# ------------------------------------------------------------------------------

# Service Account for Cloud Run code-server instance
resource "google_service_account" "cloud_run_sa" {
  account_id   = "code-server-runner-sa"
  display_name = "Cloud Run Code Server Runner SA"
  depends_on   = [google_project_service.apis]
}

resource "google_project_iam_member" "cloud_run_ar_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.cloud_run_sa.email}"
}

resource "google_project_iam_member" "cloud_run_vertex_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.cloud_run_sa.email}"
}

resource "google_project_iam_member" "cloud_run_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.cloud_run_sa.email}"
}

# ------------------------------------------------------------------------------
# 3. Container Image Reference & Cloud Run Service Setup
# ------------------------------------------------------------------------------

locals {
  container_image = "${var.region}-docker.pkg.dev/${var.project_id}/wiz-devex-demo-images/${var.service_name}:latest"
}

# Cloud Run v2 Service with IAP Authentication & Always-On CPU
resource "google_cloud_run_v2_service" "code_server" {
  name        = var.service_name
  location    = var.region
  ingress     = "INGRESS_TRAFFIC_ALL"
  iap_enabled = true

  deletion_protection = false

  template {
    service_account = google_service_account.cloud_run_sa.email

    scaling {
      min_instance_count = 1
      max_instance_count = 1
    }

    containers {
      image = local.container_image

      ports {
        container_port = 8080
      }

      resources {
        cpu_idle = false # Always-on CPU allocation for WebSockets & background tasks
        limits = {
          cpu    = "4"
          memory = "8Gi"
        }
      }

      env {
        name  = "GOOGLE_CLOUD_PROJECT"
        value = var.project_id
      }
      env {
        name  = "GOOGLE_CLOUD_LOCATION"
        value = var.region
      }
      env {
        name  = "WIZ_ENV"
        value = "demo"
      }
      env {
        name  = "WIZ_DATACENTER"
        value = "eu1"
      }
      env {
        name  = "WORKSPACE_DIR"
        value = "/home/coder/project/sorcery-solutions-frontend"
      }
      env {
        name  = "CLAUDE_CODE_USE_VERTEX"
        value = "1"
      }
      env {
        name  = "ANTHROPIC_VERTEX_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "CLOUD_ML_REGION"
        value = "global"
      }
      env {
        name  = "ANTHROPIC_MODEL"
        value = "claude-sonnet-4-6"
      }
      env {
        name  = "CLAUDE_CODE_EFFORT_LEVEL"
        value = "high"
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_project_iam_member.cloud_run_ar_reader,
    google_project_iam_member.cloud_run_vertex_user,
    google_project_iam_member.cloud_run_logging
  ]
}

# Allow IAP Service Agent permission to invoke Cloud Run
resource "google_cloud_run_v2_service_iam_member" "iap_invoker" {
  project  = google_cloud_run_v2_service.code_server.project
  location = google_cloud_run_v2_service.code_server.location
  name     = google_cloud_run_v2_service.code_server.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:service-${data.google_project.project.number}@gcp-sa-iap.iam.gserviceaccount.com"
}

# Allow designated users access to code-server via IAP
resource "google_iap_web_cloud_run_service_iam_member" "user_access" {
  for_each               = toset(var.iap_members)
  project                = google_cloud_run_v2_service.code_server.project
  location               = google_cloud_run_v2_service.code_server.location
  cloud_run_service_name = google_cloud_run_v2_service.code_server.name
  role                   = "roles/iap.httpsResourceAccessor"
  member                 = each.key
}
