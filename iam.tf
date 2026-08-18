# ------------------------------------------------------------------------------
# Service Accounts, IAM Bindings & Workload Identity
# ------------------------------------------------------------------------------

# Service Account for the Cloud Run Next.js Platform Frontend
resource "google_service_account" "platform_frontend_sa" {
  account_id   = "wiz-platform-frontend-sa"
  display_name = "Wiz Demo Platform Frontend Service Account"
  depends_on   = [google_project_service.apis]
}

# Service Account for launched Cloud Run Demo Workloads (e.g. code-server + Claude Code)
resource "google_service_account" "demo_runner_sa" {
  account_id   = "wiz-demo-runner-sa"
  display_name = "Wiz Demo Runner Workload Service Account"
  depends_on   = [google_project_service.apis]
}

# ------------------------------------------------------------------------------
# Frontend SA IAM Roles: Firestore, Cloud Build, Artifact Registry, Cloud Run, GCS
# ------------------------------------------------------------------------------

# Firestore / Datastore access to manage demo instances & metadata state
resource "google_project_iam_member" "frontend_datastore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Cloud Storage bucket reader to access configurations/index.yaml and demo templates
resource "google_storage_bucket_iam_member" "frontend_configs_reader" {
  bucket = local.config_bucket_name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Cloud Build editor to trigger on-demand image builds
resource "google_project_iam_member" "frontend_cloudbuild_editor" {
  project = var.project_id
  role    = "roles/cloudbuild.builds.editor"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Cloud Build viewer to monitor build progress and stream logs
resource "google_project_iam_member" "frontend_cloudbuild_viewer" {
  project = var.project_id
  role    = "roles/cloudbuild.builds.viewer"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Service Account User permission to act as Cloud Build service account when triggering builds
resource "google_project_iam_member" "frontend_sa_user" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Artifact Registry reader/writer to check image existence and pull images
resource "google_project_iam_member" "frontend_ar_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

resource "google_project_iam_member" "frontend_ar_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Cloud Run Admin permission to dynamically deploy, inspect, and teardown demo services
resource "google_project_iam_member" "frontend_run_admin" {
  project = var.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# IAP Admin permission to grant IAP domain access on launched demo services
resource "google_project_iam_member" "frontend_iap_admin" {
  project = var.project_id
  role    = "roles/iap.admin"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# Allow Frontend SA to act as the Demo Runner SA when deploying demo Cloud Run services
resource "google_service_account_iam_member" "frontend_act_as_demo_runner" {
  service_account_id = google_service_account.demo_runner_sa.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

resource "google_project_iam_member" "frontend_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

resource "google_project_iam_member" "frontend_logging_viewer" {
  project = var.project_id
  role    = "roles/logging.viewer"
  member  = "serviceAccount:${google_service_account.platform_frontend_sa.email}"
}

# ------------------------------------------------------------------------------
# Demo Runner SA IAM Roles: Vertex AI (Claude Sonnet 4.6), AR, Logging
# ------------------------------------------------------------------------------

resource "google_project_iam_member" "demo_runner_vertex_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.demo_runner_sa.email}"
}

resource "google_project_iam_member" "demo_runner_ar_reader" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.demo_runner_sa.email}"
}

resource "google_project_iam_member" "demo_runner_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.demo_runner_sa.email}"
}


