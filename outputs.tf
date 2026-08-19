output "portal_url" {
  description = "The direct web URL to access the IAP-secured Wiz Demo Platform."
  value       = google_cloud_run_v2_service.frontend.uri
}

output "configurations_bucket" {
  description = "Google Cloud Storage bucket storing demo templates and configurations."
  value       = local.config_bucket_name
}

output "artifact_registry_repository" {
  description = "The Artifact Registry Docker repository storing platform and demo container images."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${var.service_name}"
}

output "platform_frontend_service_account" {
  description = "GCP Service Account used by the Cloud Run Frontend."
  value       = google_service_account.platform_frontend_sa.email
}

output "demo_runner_service_account" {
  description = "GCP Service Account used by demo container pods with Workload Identity."
  value       = google_service_account.demo_runner_sa.email
}

output "firestore_database" {
  description = "Firestore database ID used for instance metadata and state tracking."
  value       = google_firestore_database.database.name
}
