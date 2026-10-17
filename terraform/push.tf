# Push notifications to the Android app, through SNS mobile push to Firebase
# Cloud Messaging. Optional: set firebase_service_account_file to turn it on.
# The owner's notices and the alarms are pushed as well as emailed; push is
# an extra, and email stays the record.

locals {
  push_enabled = var.firebase_service_account_file != ""
  # Empty when push is off, which the Lambdas take to mean no push
  push_application_arn = try(aws_sns_platform_application.fcm[0].arn, "")
}

# FCM HTTP v1, authenticated with the Firebase project's service account. The
# credential is kept in the Terraform state.
resource "aws_sns_platform_application" "fcm" {
  count = local.push_enabled ? 1 : 0

  name                = "${var.application_name}-fcm"
  platform            = "GCM" # SNS's name for FCM
  platform_credential = file(var.firebase_service_account_file)
}

resource "aws_iam_role_policy" "lambda_push" {
  count = local.push_enabled ? 1 : 0

  name = "${var.application_name}-lambda-push"
  role = aws_iam_role.lambda_role.id

  # Endpoints are created on demand, one per token, under the application.
  # SNS authorises some endpoint actions against the application's ARN (as
  # GetEndpointAttributes does) and others against the endpoint's, so both
  # are granted.
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sns:CreatePlatformEndpoint",
          "sns:GetEndpointAttributes",
          "sns:Publish"
        ]
        Resource = [
          aws_sns_platform_application.fcm[0].arn,
          "arn:aws:sns:${var.aws_region}:${data.aws_caller_identity.current.account_id}:endpoint/GCM/${aws_sns_platform_application.fcm[0].name}/*"
        ]
      }
    ]
  })
}

# Pushes the alarms. The alarms topic still emails them to the owner.
resource "aws_cloudwatch_log_group" "alarmpush_handler" {
  count = local.push_enabled ? 1 : 0

  name              = "/aws/lambda/${var.application_name}-alarmpush"
  retention_in_days = 14

  tags = var.tags
}

resource "aws_lambda_function" "alarmpush_handler" {
  count = local.push_enabled ? 1 : 0

  filename      = data.archive_file.alarmpush_lambda[0].output_path
  function_name = "${var.application_name}-alarmpush"
  role          = aws_iam_role.lambda_role.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"] # must match GOARCH in the Makefile
  timeout       = 30
  memory_size   = 128

  source_code_hash = data.archive_file.alarmpush_lambda[0].output_base64sha256

  depends_on = [aws_cloudwatch_log_group.alarmpush_handler]

  environment {
    variables = {
      STATE_TABLE_NAME              = aws_dynamodb_table.state.name
      PUSH_PLATFORM_APPLICATION_ARN = local.push_application_arn
    }
  }

  tags = merge(var.tags, {
    Name = "Alarm Push Lambda"
  })
}

data "archive_file" "alarmpush_lambda" {
  count = local.push_enabled ? 1 : 0

  type             = "zip"
  source_file      = "${path.module}/../bin/alarmpush/bootstrap"
  output_path      = "${path.module}/build/alarmpush.zip"
  output_file_mode = "0755"
}

resource "aws_sns_topic_subscription" "alarmpush" {
  count = local.push_enabled ? 1 : 0

  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.alarmpush_handler[0].arn
}

resource "aws_lambda_permission" "sns_invoke_alarmpush" {
  count = local.push_enabled ? 1 : 0

  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.alarmpush_handler[0].function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.alarms.arn
}

# Push failures. A failed push never fails the daily run or the document
# check, so they are counted from the Lambdas' "Push failed" log lines. The
# likeliest cause is a token FCM has dropped; checking in from the app
# registers a new one.
resource "aws_cloudwatch_log_metric_filter" "push_failed" {
  for_each = local.push_enabled ? {
    scheduled = aws_cloudwatch_log_group.scheduled_handler.name
    docwatch  = aws_cloudwatch_log_group.docwatch_handler.name
  } : {}

  name           = "${var.application_name}-${each.key}-push-failed"
  log_group_name = each.value
  pattern        = "{ $.msg = \"Push failed\" }"

  metric_transformation {
    namespace     = var.application_name
    name          = "PushFailures"
    value         = "1"
    default_value = "0"
  }
}

resource "aws_cloudwatch_metric_alarm" "push_failed" {
  count = local.push_enabled ? 1 : 0

  alarm_name          = "${var.application_name}-push-failed"
  alarm_description   = "A notice could not be pushed to the app (it was still emailed). If the push token is no longer valid, check in from the app to register a new one."
  namespace           = var.application_name
  metric_name         = "PushFailures"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}

# SNS published the push but FCM refused it
resource "aws_cloudwatch_metric_alarm" "push_delivery_failed" {
  count = local.push_enabled ? 1 : 0

  alarm_name          = "${var.application_name}-push-delivery-failed"
  alarm_description   = "Firebase Cloud Messaging refused a push to the app (it was still emailed). Check the Firebase project and the service account credential."
  namespace           = "AWS/SNS"
  metric_name         = "NumberOfNotificationsFailed"
  dimensions          = { Application = aws_sns_platform_application.fcm[0].name, Platform = "GCM" }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}

# An alarm could not be pushed. It notifies through the same topic, which
# triggers one more push attempt; alarms notify only on entering ALARM, so
# this cannot loop.
resource "aws_cloudwatch_metric_alarm" "alarmpush_errors" {
  count = local.push_enabled ? 1 : 0

  alarm_name          = "${var.application_name}-alarmpush-errors"
  alarm_description   = "An alarm could not be pushed to the app (it was still emailed). Check the alarmpush Lambda's CloudWatch logs."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  dimensions          = { FunctionName = aws_lambda_function.alarmpush_handler[0].function_name }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}
