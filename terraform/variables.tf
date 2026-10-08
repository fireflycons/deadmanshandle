variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "eu-west-1"
}

variable "application_name" {
  description = "Name of the application"
  type        = string
  default     = "deadmanshandle"
}

variable "config_file_path" {
  description = "Path to initial configuration JSON file, relative to the directory Terraform runs in"
  type        = string
  # Defaults must be literals (no path.module), so this assumes Terraform runs
  # from terraform/ and finds config.json at the repo root.
  default = "../config.json"
}

variable "create_ses_identity" {
  description = "Create the SES domain identity for the domain of deployment.senderEmail in the config. Set false if the domain is already a verified SES identity in this account and region; it is then used as it is, and Terraform never manages or deletes it."
  type        = bool
  default     = true
}

variable "manage_dkim_dns_records" {
  description = "Create the SES DKIM records in the Route 53 public zone named after the domain of deployment.senderEmail in the config (in this account). Set false to add the records from the ses_dkim_dns_records output at another DNS provider. Ignored when create_ses_identity is false."
  type        = bool
  default     = true
}

variable "document_versioning" {
  description = "Enable versioning on the document bucket. Noncurrent versions are deleted after 30 days. False suspends versioning on a bucket that had it."
  type        = bool
  default     = false
}

variable "schedule_expression" {
  description = "EventBridge schedule expression for daily check (cron format)"
  type        = string
  default     = "cron(0 2 * * ? *)"
}

variable "tags" {
  description = "Common tags for all resources"
  type        = map(string)
  default = {
    Application = "deadmanshandle"
    ManagedBy   = "terraform"
  }
}
