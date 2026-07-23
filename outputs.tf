output "cloud_run_service_url" {
  description = "The direct web URL to access the IAP-secured code-server Cloud Run service."
  value       = google_cloud_run_v2_service.code_server.uri
}

output "artifact_registry_repository" {
  description = "The Artifact Registry Docker repository storing custom code-server images."
  value       = google_artifact_registry_repository.code_server_images.id
}

output "custom_container_image_tag" {
  description = "The fully-qualified container image URI built via Cloud Build and deployed to Cloud Run."
  value       = local.container_image
}
