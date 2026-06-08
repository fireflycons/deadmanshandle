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

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

variable "config_file_path" {
  description = "Path to initial configuration JSON file"
  type        = string
}

variable "sender_email" {
  description = "Email address for sending notifications"
  type        = string
  default     = "noreply@firefly-consulting.co.uk"
}

variable "document_bucket_name" {
  description = "Name of the S3 bucket for storing the document"
  type        = string
  default     = ""
}

variable "document_key" {
  description = "S3 object key for the document"
  type        = string
  default     = "document.pdf"
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
