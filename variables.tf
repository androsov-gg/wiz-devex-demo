variable "project_id" {
  type        = string
  description = "Google Cloud project ID where the Wiz Demo Platform will be deployed."
}

variable "region" {
  type        = string
  description = "Google Cloud region for Cloud Run and infrastructure deployment."
  validation {
    condition     = contains(tolist(["europe-west1", "us-east5", "asia-southeast1", "us-central1"]), var.region)
    error_message = "The supported regions are europe-west1, us-east5, asia-southeast1, or us-central1."
  }
}

variable "network_name" {
  type        = string
  default     = "wiz-devex-demo-platform-vpc"
  description = "Name of the custom VPC network."
}

variable "service_name" {
  type        = string
  default     = "wiz-devex-demo-platform"
  description = "Name of the IAP-secured Cloud Run frontend service."
}

variable "config_bucket_name" {
  type        = string
  default     = ""
  description = "Optional custom name for the GCS configurations storage bucket. Defaults to <project_id>-wiz-devex-demo-configs."
}

variable "iap_members" {
  type        = list(string)
  default     = ["domain:yourname.altostrat.com"]
  description = "List of IAM members (e.g. domain:yourname.altostrat.com, user:user@yourname.altostrat.com) allowed access via IAP."
}
