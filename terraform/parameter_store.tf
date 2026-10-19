locals {
  # The seed config, checked on every plan with the same rules as
  # Config.Validate in internal/config. Later updates made with the AWS CLI
  # bypass these checks; the Lambdas reject an invalid config at run time.
  seed_config = jsondecode(file(var.config_file_path))

  # The deployment section holds infrastructure settings. Unlike the rest of
  # the file, it is read on every plan, and it is not stored in the parameter.
  # Nor is the timeout, which seeds the state item in dynamodb.tf.
  deployment           = try(local.seed_config.deployment, {})
  sender_email         = try(local.deployment.senderEmail, "")
  document_bucket_name = try(local.deployment.documentBucket, "")

  # Stored in the parameter, so that it can be changed without Terraform.
  # Older seed files have it in the deployment section.
  document_key = try(local.seed_config.documentKey, local.deployment.documentKey, "document.pdf")

  # A bare address, with a dot in the domain. Config.Validate uses the same
  # pattern (emailAddressPattern in internal/config), and a test checks they
  # match.
  email_address_pattern = "^[A-Za-z0-9_%+'-]+([.][A-Za-z0-9_%+'-]+)*@[A-Za-z0-9-]+([.][A-Za-z0-9-]+)+$"
}

resource "aws_ssm_parameter" "config" {
  name = "/${var.application_name}/config"
  type = "SecureString" # holds the API key; encrypted with the AWS-managed key alias/aws/ssm
  value = jsonencode(merge(
    { for k, v in local.seed_config : k => v if !contains(["deployment", "timeout", "sentTo", "ownerNotified"], k) },
    { documentKey = local.document_key },
  ))

  # The config file only seeds the parameter; after that it is updated with
  # the AWS CLI (see DEPLOYMENT.md), so later applies must not reset it from
  # the file. The Lambdas only read it. To re-seed deliberately, run:
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
    # Blank or missing addresses are reported above
    precondition {
      condition     = try(trimspace(local.seed_config.owner) == "" || can(regex(local.email_address_pattern, local.seed_config.owner)), true)
      error_message = "Config: owner is not an email address."
    }
    precondition {
      condition     = try(alltrue([for r in local.seed_config.recipients : trimspace(r) == "" || can(regex(local.email_address_pattern, r))]), true)
      error_message = "Config: recipients contains an entry that is not an email address."
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
      condition     = try(trimspace(local.seed_config.apiKey) != "", false)
      error_message = "Config: apiKey is empty."
    }
    precondition {
      condition     = can(regex(local.email_address_pattern, local.sender_email))
      error_message = "Config: deployment.senderEmail must be an email address."
    }
    precondition {
      condition     = try(trimspace(local.document_key) != "", false)
      error_message = "Config: documentKey is blank."
    }
  }

  tags = merge(var.tags, {
    Name = "Application Configuration"
  })
}

# The endpoint, kept up to date by Terraform, so that `dmh qr -aws` can set up
# the Android app from AWS alone. Not secret, and not read by the Lambdas.
resource "aws_ssm_parameter" "api_endpoint" {
  name  = "/${var.application_name}/api-endpoint"
  type  = "String"
  value = local.api_endpoint

  tags = merge(var.tags, {
    Name = "API Endpoint"
  })
}

# Like the endpoint, these let the CLI find the resources from the
# application name alone. Not secret, and not read by the Lambdas.
resource "aws_ssm_parameter" "document_bucket" {
  name  = "/${var.application_name}/document-bucket"
  type  = "String"
  value = aws_s3_bucket.document_bucket.id

  tags = merge(var.tags, {
    Name = "Document Bucket"
  })
}

resource "aws_ssm_parameter" "state_table" {
  name  = "/${var.application_name}/state-table"
  type  = "String"
  value = aws_dynamodb_table.state.name

  tags = merge(var.tags, {
    Name = "State Table"
  })
}

resource "aws_ssm_parameter" "api_stage" {
  name = "/${var.application_name}/api-stage"
  type = "String"
  value = jsonencode({
    apiId      = aws_apigatewayv2_api.http_api.id
    stage      = aws_apigatewayv2_stage.http_api_stage.name
    rateLimit  = local.api_throttle_rate
    burstLimit = local.api_throttle_burst
    guardRule  = local.api_guard_rule_name # empty when the flood guard is off
  })

  tags = merge(var.tags, {
    Name = "API Stage"
  })
}

resource "aws_ssm_parameter" "sender_email" {
  name  = "/${var.application_name}/sender-email"
  type  = "String"
  value = local.sender_email

  tags = merge(var.tags, {
    Name = "Sender Email"
  })
}

# A warning, not an error: the file's timeout only matters when the state
# item is created (or re-seeded), and is expected to be past on later applies.
check "seed_timeout_in_future" {
  assert {
    condition     = try(timecmp(local.seed_config.timeout, plantimestamp()) > 0, false)
    error_message = "The timeout in config_file_path is not in the future. If this apply creates or re-seeds the state item, the document will be sent on the next daily run."
  }
}

