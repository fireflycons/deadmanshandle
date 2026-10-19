# One role per Lambda, each with only the permissions its function uses, so
# that the public-facing HTTP Lambda cannot send email, read the document or
# change the API's throttle.

locals {
  config_read = {
    Effect   = "Allow"
    Action   = ["ssm:GetParameter"]
    Resource = [aws_ssm_parameter.config.arn]
  }

  state_read = {
    Effect   = "Allow"
    Action   = ["dynamodb:GetItem"]
    Resource = [aws_dynamodb_table.state.arn]
  }

  state_update = {
    Effect   = "Allow"
    Action   = ["dynamodb:UpdateItem"]
    Resource = [aws_dynamodb_table.state.arn]
  }

  # HeadObject is authorised as GetObject
  document_read = {
    Effect   = "Allow"
    Action   = ["s3:GetObject"]
    Resource = ["${aws_s3_bucket.document_bucket.arn}/*"]
  }

  # Lets HeadObject report a missing document as NotFound rather than
  # AccessDenied, so the owner can be warned
  document_list = {
    Effect   = "Allow"
    Action   = ["s3:ListBucket"]
    Resource = [aws_s3_bucket.document_bucket.arn]
  }

  email_send = {
    Effect = "Allow"
    Action = ["ses:SendEmail", "ses:SendRawEmail"]
    # Any identity in this account and region rather than just the sender:
    # in the SES sandbox the verified recipient identities are checked too
    Resource = "arn:aws:ses:${var.aws_region}:${data.aws_caller_identity.current.account_id}:identity/*"
  }

  # Endpoints are created on demand, one per token, under the application.
  # SNS authorises some endpoint actions against the application's ARN (as
  # GetEndpointAttributes does) and others against the endpoint's, so both
  # are granted. Empty when push is off.
  push_send = local.push_enabled ? [{
    Effect = "Allow"
    Action = [
      "sns:CreatePlatformEndpoint",
      "sns:GetEndpointAttributes",
      "sns:Publish"
    ]
    Resource = [
      local.push_application_arn,
      "arn:aws:sns:${var.aws_region}:${data.aws_caller_identity.current.account_id}:endpoint/GCM/${var.application_name}-fcm/*"
    ]
  }] : []

  lambda_functions = concat(
    ["http", "scheduled", "docwatch"],
    var.api_flood_guard ? ["apiguard"] : [],
    local.push_enabled ? ["alarmpush"] : [],
  )

  # Each deployed function's policy document
  lambda_policies = {
    for name, statements in {
      http = [local.config_read, local.state_update]
      scheduled = concat([
        local.config_read, local.state_read, local.state_update,
        local.document_read, local.document_list, local.email_send
      ], local.push_send)
      docwatch = concat([
        local.config_read, local.state_read, local.state_update,
        local.document_read, local.document_list, local.email_send
      ], local.push_send)
      apiguard = concat(
        [local.config_read, local.state_read, local.email_send],
        local.push_send, local.api_guard_statements
      )
      alarmpush = concat([local.state_read], local.push_send)
    } : name => jsonencode({ Version = "2012-10-17", Statement = statements })
    if contains(local.lambda_functions, name)
  }
}

resource "aws_iam_role" "lambda" {
  for_each = local.lambda_policies

  name = "${var.application_name}-${each.key}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = var.tags
}

resource "aws_iam_role_policy_attachment" "lambda_basic_execution" {
  for_each = local.lambda_policies

  role       = aws_iam_role.lambda[each.key].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "lambda" {
  for_each = local.lambda_policies

  name   = "${var.application_name}-${each.key}-policy"
  role   = aws_iam_role.lambda[each.key].id
  policy = each.value
}
