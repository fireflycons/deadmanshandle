locals {
  # The seed config, checked on every plan with the same rules as
  # Config.Validate in internal/config. Later updates made with the AWS CLI
  # bypass these checks; the Lambdas reject an invalid config at run time.
  seed_config = jsondecode(file(var.config_file_path))
}

resource "aws_ssm_parameter" "config" {
  name  = "/${var.application_name}/config"
  type  = "SecureString" # holds the API key; encrypted with the AWS-managed key alias/aws/ssm
  value = file(var.config_file_path)

  # The config file only seeds the parameter. After that the Lambda owns the
  # value (each check-in rewrites the timeout), so later applies must not
  # reset it from the file. To re-seed deliberately, run:
  #   terraform apply -replace=aws_ssm_parameter.config
  lifecycle {
    ignore_changes = [value]

    precondition {
      condition     = try(trimspace(local.seed_config.owner) != "", false)
      error_message = "Config: owner is empty."
    }
    precondition {
      condition     = try(length(local.seed_config.recipients) > 0 && alltrue([for r in local.seed_config.recipients : trimspace(r) != ""]), false)
      error_message = "Config: recipients is empty or contains a blank entry."
    }
    precondition {
      condition     = try(local.seed_config.resetDays > 0, false)
      error_message = "Config: resetDays must be greater than 0."
    }
    precondition {
      condition     = try(local.seed_config.warnDays >= 0 && local.seed_config.warnDays < local.seed_config.resetDays, false)
      error_message = "Config: warnDays must be at least 0 and less than resetDays."
    }
    precondition {
      condition     = try(timecmp(local.seed_config.timeout, "0001-01-01T00:00:00Z") > 0, false)
      error_message = "Config: timeout is missing or not an RFC 3339 time."
    }
    precondition {
      condition     = try(trimspace(local.seed_config.apiKey) != "", false)
      error_message = "Config: apiKey is empty."
    }
  }

  tags = merge(var.tags, {
    Name = "Application Configuration"
  })
}

# A warning, not an error: the file's timeout only matters when the parameter
# is created (or re-seeded), and is expected to be past on later applies.
check "seed_timeout_in_future" {
  assert {
    condition     = try(timecmp(local.seed_config.timeout, plantimestamp()) > 0, false)
    error_message = "The timeout in config_file_path is not in the future. If this apply creates or re-seeds the parameter, the document will be sent on the next daily run."
  }
}
