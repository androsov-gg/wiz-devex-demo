output "cloud_run_service_url" {
  description = "The direct web URL to access the IAP-secured code-server Cloud Run service."
  value       = google_cloud_run_v2_service.code_server.uri
}

output "artifact_registry_repository" {
  description = "The Artifact Registry Docker repository storing custom code-server images."
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/wiz-devex-demo-images"
}

output "custom_container_image_tag" {
  description = "The fully-qualified container image URI built via Cloud Build and deployed to Cloud Run."
  value       = local.container_image
}

output "scheduler_enabled" {
  description = "Whether automated Cloud Scheduler scaling is enabled."
  value       = var.enable_scheduler
}

output "scheduler_scale_down_job" {
  description = "The name of the Cloud Scheduler scale-down job (if enabled)."
  value       = var.enable_scheduler ? google_cloud_scheduler_job.scale_down[0].name : null
}

output "scheduler_scale_up_job" {
  description = "The name of the Cloud Scheduler scale-up job (if enabled)."
  value       = var.enable_scheduler ? google_cloud_scheduler_job.scale_up[0].name : null
}

output "scheduler_schedule_timezone" {
  description = "Configured timezone for the scaling schedule."
  value       = var.enable_scheduler ? var.schedule_timezone : null
}

