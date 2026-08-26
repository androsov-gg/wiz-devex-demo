variable "project_id" {
  type        = string
  description = "Google Cloud project ID where Cloud Run code-server will be deployed."
}

variable "region" {
  type        = string
  description = "Google Cloud region for deployment."
  validation {
    condition     = contains(tolist(["europe-west1", "us-east5", "asia-southeast1"]), var.region)
    error_message = "The only supported regions are europe-west1, us-east5 or asia-southeast1"
  }
}

variable "service_name" {
  type        = string
  default     = "wiz-code-golden-demo"
  description = "Name of the Cloud Run service."
}

variable "image_tag" {
  type        = string
  default     = "latest"
  description = "Specific container image tag or content hash to deploy."
}

variable "iap_members" {
  type        = list(string)
  default     = ["domain:yourname.altostrat.com"]
  description = "List of IAM members (e.g. domain:yourname.altostrat.com, user:user@yourname.altostrat.com) allowed access via IAP."
}
