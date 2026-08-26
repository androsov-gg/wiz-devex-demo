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
    time = {
      source  = "hashicorp/time"
      version = ">= 0.9.0"
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
    "iap.googleapis.com",
    "iam.googleapis.com",
    "cloudscheduler.googleapis.com"
  ])
  service            = each.key
  disable_on_destroy = false
}

# Wait 30 seconds after API enablement to allow Google Cloud auth caches and Service Agents to stabilize
resource "time_sleep" "wait_for_apis" {
  depends_on      = [google_project_service.apis]
  create_duration = "30s"
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
  depends_on   = [time_sleep.wait_for_apis]
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
  container_image = "${var.region}-docker.pkg.dev/${var.project_id}/wiz-devex-demo-images/${var.service_name}:${var.image_tag}"
}

# Cloud Run v2 Service with IAP Authentication & Always-On CPU
resource "google_cloud_run_v2_service" "code_server" {
  name        = var.service_name
  location    = var.region
  ingress     = "INGRESS_TRAFFIC_ALL"
  iap_enabled = true

  deletion_protection = false

  scaling {
    scaling_mode          = "MANUAL"
    manual_instance_count = 1
  }

  template {
    service_account = google_service_account.cloud_run_sa.email

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

  lifecycle {
    ignore_changes = [
      scaling[0].manual_instance_count
    ]
  }

  depends_on = [
    time_sleep.wait_for_apis,
    google_project_iam_member.cloud_run_ar_reader,
    google_project_iam_member.cloud_run_vertex_user,
    google_project_iam_member.cloud_run_logging
  ]
}

# Allow IAP Service Agent permission to invoke Cloud Run
resource "google_cloud_run_v2_service_iam_member" "iap_invoker" {
  project    = google_cloud_run_v2_service.code_server.project
  location   = google_cloud_run_v2_service.code_server.location
  name       = google_cloud_run_v2_service.code_server.name
  role       = "roles/run.invoker"
  member     = "serviceAccount:service-${data.google_project.project.number}@gcp-sa-iap.iam.gserviceaccount.com"
  depends_on = [time_sleep.wait_for_apis]
}

# Allow designated users access to code-server via IAP
resource "google_iap_web_cloud_run_service_iam_member" "user_access" {
  for_each               = toset(var.iap_members)
  project                = google_cloud_run_v2_service.code_server.project
  location               = google_cloud_run_v2_service.code_server.location
  cloud_run_service_name = google_cloud_run_v2_service.code_server.name
  role                   = "roles/iap.httpsResourceAccessor"
  member                 = each.key
  depends_on             = [time_sleep.wait_for_apis]
}

# ------------------------------------------------------------------------------
# 4. Cloud Scheduler Scheduled Scaling (Off-Hours Cost Optimization)
# ------------------------------------------------------------------------------

# Service account for Cloud Scheduler to patch Cloud Run service scaling
resource "google_service_account" "scheduler_sa" {
  count        = var.enable_scheduler ? 1 : 0
  account_id   = "code-server-scheduler-sa"
  display_name = "Cloud Run Scaling Scheduler Service Account"
  depends_on   = [time_sleep.wait_for_apis]
}

# Grant Cloud Scheduler SA permission to update Cloud Run service configuration
resource "google_cloud_run_v2_service_iam_member" "scheduler_run_developer" {
  count      = var.enable_scheduler ? 1 : 0
  project    = google_cloud_run_v2_service.code_server.project
  location   = google_cloud_run_v2_service.code_server.location
  name       = google_cloud_run_v2_service.code_server.name
  role       = "roles/run.developer"
  member     = "serviceAccount:${google_service_account.scheduler_sa[0].email}"
  depends_on = [time_sleep.wait_for_apis]
}

# Grant Cloud Scheduler SA permission to act as Cloud Run runtime service account
resource "google_service_account_iam_member" "scheduler_sa_user" {
  count              = var.enable_scheduler ? 1 : 0
  service_account_id = google_service_account.cloud_run_sa.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.scheduler_sa[0].email}"
  depends_on         = [time_sleep.wait_for_apis]
}

# Cloud Scheduler Job: Scale Down to 0 at end of working day
resource "google_cloud_scheduler_job" "scale_down" {
  count       = var.enable_scheduler ? 1 : 0
  name        = "${var.service_name}-scale-down"
  description = "Scale Cloud Run manual instances to 0 at the end of the working day"
  schedule    = var.scale_down_schedule
  time_zone   = var.schedule_timezone
  region      = var.region

  http_target {
    http_method = "PATCH"
    uri         = "https://run.googleapis.com/v2/projects/${var.project_id}/locations/${var.region}/services/${google_cloud_run_v2_service.code_server.name}?updateMask=scaling.manual_instance_count,scaling.scaling_mode"
    body = base64encode(jsonencode({
      scaling = {
        scalingMode         = "MANUAL"
        manualInstanceCount = 0
      }
    }))
    headers = {
      "Content-Type" = "application/json"
    }

    oauth_token {
      service_account_email = google_service_account.scheduler_sa[0].email
      scope                 = "https://www.googleapis.com/auth/cloud-platform"
    }
  }

  depends_on = [
    time_sleep.wait_for_apis,
    google_cloud_run_v2_service_iam_member.scheduler_run_developer,
    google_service_account_iam_member.scheduler_sa_user
  ]
}

# Cloud Scheduler Job: Scale Up to 1 at start of working day
resource "google_cloud_scheduler_job" "scale_up" {
  count       = var.enable_scheduler ? 1 : 0
  name        = "${var.service_name}-scale-up"
  description = "Scale Cloud Run manual instances to 1 at the start of the working day"
  schedule    = var.scale_up_schedule
  time_zone   = var.schedule_timezone
  region      = var.region

  http_target {
    http_method = "PATCH"
    uri         = "https://run.googleapis.com/v2/projects/${var.project_id}/locations/${var.region}/services/${google_cloud_run_v2_service.code_server.name}?updateMask=scaling.manual_instance_count,scaling.scaling_mode"
    body = base64encode(jsonencode({
      scaling = {
        scalingMode         = "MANUAL"
        manualInstanceCount = 1
      }
    }))
    headers = {
      "Content-Type" = "application/json"
    }

    oauth_token {
      service_account_email = google_service_account.scheduler_sa[0].email
      scope                 = "https://www.googleapis.com/auth/cloud-platform"
    }
  }

  depends_on = [
    time_sleep.wait_for_apis,
    google_cloud_run_v2_service_iam_member.scheduler_run_developer,
    google_service_account_iam_member.scheduler_sa_user
  ]
}

