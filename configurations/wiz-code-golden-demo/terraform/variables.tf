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

variable "enable_scheduler" {
  type        = bool
  default     = true
  description = "Enable Cloud Scheduler jobs for automated off-hours scale down to 0 and working-hours scale up to 1."
}

variable "schedule_timezone" {
  type        = string
  default     = "UTC"
  description = "Timezone for Cloud Scheduler scaling jobs in IANA tz format (e.g. UTC, America/New_York, Europe/London, America/Los_Angeles)."
}

variable "scale_down_schedule" {
  type        = string
  default     = "0 19 * * 1-5"
  description = "Cron schedule expression for scaling down min instances to 0 at the end of the working day (e.g., 19:00 / 7 PM Mon-Fri)."
}

variable "scale_up_schedule" {
  type        = string
  default     = "0 8 * * 1-5"
  description = "Cron schedule expression for scaling up min instances to 1 at the start of the working day (e.g., 08:00 / 8 AM Mon-Fri)."
}

