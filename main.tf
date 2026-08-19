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
    google-beta = {
      source  = "hashicorp/google-beta"
      version = ">= 5.30.0"
    }
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.0"
    }
    time = {
      source  = "hashicorp/time"
      version = ">= 0.9.0"
    }
  }

  backend "gcs" {
    # Dynamically configured per GCP project during terraform init:
    # terraform init -backend-config="bucket=${PROJECT_ID}-${SERVICE_NAME}-tfstate"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

data "google_project" "project" {
  project_id = var.project_id
}

# ------------------------------------------------------------------------------
# 2. Required Google Cloud APIs
# ------------------------------------------------------------------------------
resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "container.googleapis.com",
    "compute.googleapis.com",
    "firestore.googleapis.com",
    "cloudbuild.googleapis.com",
    "artifactregistry.googleapis.com",
    "iap.googleapis.com",
    "aiplatform.googleapis.com",
    "storage.googleapis.com",
    "iam.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "cloudresourcemanager.googleapis.com"
  ])
  service            = each.key
  disable_on_destroy = false
}

# ------------------------------------------------------------------------------
# 3. Service Identities (Ensures service agents exist before IAM bindings)
# ------------------------------------------------------------------------------
resource "google_project_service_identity" "cloudbuild_sa" {
  provider   = google-beta
  project    = var.project_id
  service    = "cloudbuild.googleapis.com"
  depends_on = [google_project_service.apis]
}

resource "google_project_service_identity" "iap_sa" {
  provider   = google-beta
  project    = var.project_id
  service    = "iap.googleapis.com"
  depends_on = [google_project_service.apis]
}

# ------------------------------------------------------------------------------
# 4. Artifact Registry Repository
# ------------------------------------------------------------------------------
resource "google_artifact_registry_repository" "images" {
  location      = var.region
  repository_id = var.service_name
  description   = "Docker repository for Wiz Demo Platform and demo images"
  format        = "DOCKER"
  depends_on    = [google_project_service.apis]
}

# ------------------------------------------------------------------------------
# 5. VPC Networking, Subnets & Cloud NAT (Argolis Private Cluster Compliance)
# ------------------------------------------------------------------------------

locals {
  network_name       = "${var.service_name}-vpc"
  database_name      = "${var.service_name}-db"
  frontend_image     = "${var.region}-docker.pkg.dev/${var.project_id}/${var.service_name}/portal:latest"
  config_bucket_name = "${var.project_id}-${var.service_name}-configs"
}

resource "google_compute_network" "vpc" {
  name                    = local.network_name
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "subnet" {
  name                     = "${local.network_name}-subnet"
  ip_cidr_range            = "10.0.0.0/24"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  private_ip_google_access = true
}

# Cloud Router for outbound NAT
resource "google_compute_router" "router" {
  name    = "${local.network_name}-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

# Cloud NAT for outbound internet connectivity
resource "google_compute_router_nat" "nat" {
  name                               = "${local.network_name}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

# ------------------------------------------------------------------------------
# 6. Firestore Database (Native MoFde for State & Audit Storage)
# ------------------------------------------------------------------------------

resource "google_firestore_database" "database" {
  project     = var.project_id
  name        = local.database_name
  location_id = var.region
  type        = "FIRESTORE_NATIVE"

  deletion_policy = "DELETE_PROTECTION_DISABLED"

  depends_on = [google_project_service.apis]
}

# ------------------------------------------------------------------------------
# 7. Configurations Cloud Storage Bucket & Initial Templates
# ------------------------------------------------------------------------------

resource "google_storage_bucket" "configs" {
  name                        = local.config_bucket_name
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = true

  depends_on = [google_project_service.apis]
}

resource "google_storage_bucket_object" "config_files" {
  for_each = fileset("${path.module}/configurations", "**")
  name     = each.value
  bucket   = google_storage_bucket.configs.name
  source   = "${path.module}/configurations/${each.value}"
}

# ------------------------------------------------------------------------------
# 8. Build Frontend Portal Image via Cloud Build
# ------------------------------------------------------------------------------

# Ensure IAM permissions and service identities have propagated before invoking Cloud Build
resource "time_sleep" "wait_for_iam_propagation" {
  create_duration = "20s"

  depends_on = [
    google_project_service_identity.cloudbuild_sa,
    google_project_service_identity.iap_sa,
    google_service_account.cloudbuild_builder_sa,
    google_project_iam_member.cloudbuild_builder_roles,
    google_service_account_iam_member.cloudbuild_sa_actas_builder,
    google_project_iam_member.cloudbuild_sa_storage_admin,
    google_artifact_registry_repository.images,
    google_project_service.apis
  ]
}

resource "null_resource" "build_portal_image" {
  triggers = {
    # Hash all source files in portal/ excluding local dependencies and build artifacts
    portal_sha = sha256(join("", [
      for f in fileset("${path.module}/portal", "**") :
      filesha256("${path.module}/portal/${f}")
      if !startswith(f, "frontend/node_modules/") &&
      !startswith(f, "frontend/dist/") &&
      !startswith(f, "backend/dist/") &&
      !startswith(f, ".next/")
    ]))
    image_tag = local.frontend_image
  }

  provisioner "local-exec" {
    command = <<-EOT
      gcloud builds submit \
        --project="${var.project_id}" \
        --region="${var.region}" \
        --service-account="projects/${var.project_id}/serviceAccounts/${google_service_account.cloudbuild_builder_sa.email}" \
        --config="${path.module}/portal/cloudbuild.yaml" \
        --substitutions=_IMAGE_TAG="${local.frontend_image}" \
        "${path.module}/portal"
    EOT
  }

  depends_on = [
    time_sleep.wait_for_iam_propagation,
    google_artifact_registry_repository.images,
    google_service_account.cloudbuild_builder_sa,
    google_project_service.apis
  ]
}

# ------------------------------------------------------------------------------
# 9. Cloud Run Frontend Service (Google-Managed IAP & Direct VPC Egress)
# ------------------------------------------------------------------------------

resource "google_cloud_run_v2_service" "frontend" {
  name        = var.service_name
  location    = var.region
  ingress     = "INGRESS_TRAFFIC_ALL"
  iap_enabled = true

  deletion_protection = false

  scaling {
    manual_instance_count = 1
    scaling_mode          = "MANUAL"
  }

  template {
    service_account = google_service_account.platform_frontend_sa.email

    scaling {
      min_instance_count = 1
      max_instance_count = 1
    }

    vpc_access {
      network_interfaces {
        network    = google_compute_network.vpc.name
        subnetwork = google_compute_subnetwork.subnet.name
      }
      egress = "ALL_TRAFFIC"
    }

    containers {
      image = local.frontend_image

      ports {
        container_port = 3000
      }

      resources {
        limits = {
          cpu    = "2"
          memory = "2Gi"
        }
      }

      env {
        name  = "NODE_ENV"
        value = "production"
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
        name  = "CONFIG_BUCKET_NAME"
        value = local.config_bucket_name
      }
      env {
        name  = "IAP_MEMBERS"
        value = join(",", var.iap_members)
      }
      env {
        name  = "DEMO_RUNNER_SA_EMAIL"
        value = google_service_account.demo_runner_sa.email
      }
      env {
        name  = "PROJECT_NUMBER"
        value = data.google_project.project.number
      }
      env {
        name  = "SERVICE_NAME"
        value = var.service_name
      }
      env {
        name  = "ARTIFACT_REPO_NAME"
        value = google_artifact_registry_repository.images.repository_id
      }
      env {
        name  = "FIRESTORE_DATABASE_ID"
        value = google_firestore_database.database.name
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_firestore_database.database,
    google_storage_bucket.configs,
    null_resource.build_portal_image
  ]
}

# Allow IAP Service Agent permission to invoke Cloud Run
resource "google_cloud_run_v2_service_iam_member" "iap_invoker" {
  project    = google_cloud_run_v2_service.frontend.project
  location   = google_cloud_run_v2_service.frontend.location
  name       = google_cloud_run_v2_service.frontend.name
  role       = "roles/run.invoker"
  member     = "serviceAccount:${google_project_service_identity.iap_sa.email}"
  depends_on = [google_project_service_identity.iap_sa]
}

# Allow designated users access to the platform via IAP
resource "google_iap_web_cloud_run_service_iam_member" "user_access" {
  for_each               = toset(var.iap_members)
  project                = google_cloud_run_v2_service.frontend.project
  location               = google_cloud_run_v2_service.frontend.location
  cloud_run_service_name = google_cloud_run_v2_service.frontend.name
  role                   = "roles/iap.httpsResourceAccessor"
  member                 = each.key
}

