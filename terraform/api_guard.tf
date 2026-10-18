# The flood guard. When the api-rejected-requests alarm fires, the apiguard
# Lambda closes the check-in API (stage throttle rate 0, burst 0, so API
# Gateway refuses every request without invoking the HTTP Lambda) and starts
# the check rule. Each check reopens the API once the alarm is back to OK,
# and stops the rule. Optional: set api_flood_guard = false to turn it off;
# dmh api disable and enable work either way.

locals {
  api_guard_count = var.api_flood_guard ? 1 : 0
  # Published in the api-stage parameter for dmh; empty when the guard is off
  api_guard_rule_name = var.api_flood_guard ? "${var.application_name}-api-guard-check" : ""
}

resource "aws_cloudwatch_log_group" "apiguard_handler" {
  count = local.api_guard_count

  name              = "/aws/lambda/${var.application_name}-apiguard"
  retention_in_days = 14

  tags = var.tags
}

resource "aws_lambda_function" "apiguard_handler" {
  count = local.api_guard_count

  filename      = data.archive_file.apiguard_lambda[0].output_path
  function_name = "${var.application_name}-apiguard"
  role          = aws_iam_role.lambda_role.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"] # must match GOARCH in the Makefile
  timeout       = 30
  memory_size   = 128

  source_code_hash = data.archive_file.apiguard_lambda[0].output_base64sha256

  depends_on = [aws_cloudwatch_log_group.apiguard_handler]

  environment {
    variables = {
      CONFIG_PARAMETER_NAME         = aws_ssm_parameter.config.name
      STATE_TABLE_NAME              = aws_dynamodb_table.state.name
      SENDER_EMAIL                  = local.sender_email
      PUSH_PLATFORM_APPLICATION_ARN = local.push_application_arn
      API_ID                        = aws_apigatewayv2_api.http_api.id
      API_STAGE                     = aws_apigatewayv2_stage.http_api_stage.name
      API_RATE_LIMIT                = local.api_throttle_rate
      API_BURST_LIMIT               = local.api_throttle_burst
      ALARM_NAME                    = aws_cloudwatch_metric_alarm.api_rejected_requests.alarm_name
      CHECK_RULE_NAME               = local.api_guard_rule_name
    }
  }

  tags = merge(var.tags, {
    Name = "API Guard Lambda"
  })
}

data "archive_file" "apiguard_lambda" {
  count = local.api_guard_count

  type             = "zip"
  source_file      = "${path.module}/../bin/apiguard/bootstrap"
  output_path      = "${path.module}/build/apiguard.zip"
  output_file_mode = "0755"
}

# The alarms, among them the one that closes the API. The Lambda ignores
# the others.
resource "aws_sns_topic_subscription" "apiguard" {
  count = local.api_guard_count

  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.apiguard_handler[0].arn
}

resource "aws_lambda_permission" "sns_invoke_apiguard" {
  count = local.api_guard_count

  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.apiguard_handler[0].function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.alarms.arn
}

# The check, which runs only while the guard has the API closed: the Lambda
# enables and disables the rule. The rate only seeds the rule; after that
# dmh api check-interval changes it, so later applies leave both alone.
resource "aws_cloudwatch_event_rule" "api_guard_check" {
  count = local.api_guard_count

  name                = local.api_guard_rule_name
  description         = "Checks whether to reopen the check-in API after a flood"
  schedule_expression = var.api_guard_check_minutes == 1 ? "rate(1 minute)" : "rate(${var.api_guard_check_minutes} minutes)"
  state               = "DISABLED"

  lifecycle {
    ignore_changes = [state, schedule_expression]
  }

  tags = merge(var.tags, {
    Name = "API Guard Check Rule"
  })
}

resource "aws_cloudwatch_event_target" "api_guard_check" {
  count = local.api_guard_count

  rule      = aws_cloudwatch_event_rule.api_guard_check[0].name
  target_id = "DeadmansHandleAPIGuard"
  arn       = aws_lambda_function.apiguard_handler[0].arn
}

resource "aws_lambda_permission" "eventbridge_invoke_apiguard" {
  count = local.api_guard_count

  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.apiguard_handler[0].function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.api_guard_check[0].arn
}

# On the shared Lambda role, which already has the config, state, SES and
# push permissions the notices need
resource "aws_iam_role_policy" "lambda_api_guard" {
  count = local.api_guard_count

  name = "${var.application_name}-lambda-api-guard"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # GetStage and UpdateStage on the check-in API's stage
        Effect   = "Allow"
        Action   = ["apigateway:GET", "apigateway:PATCH"]
        Resource = "arn:aws:apigateway:${var.aws_region}::/apis/${aws_apigatewayv2_api.http_api.id}/stages/${aws_apigatewayv2_stage.http_api_stage.name}"
      },
      {
        # DescribeAlarms has no resource-level permissions
        Effect   = "Allow"
        Action   = "cloudwatch:DescribeAlarms"
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["events:DescribeRule", "events:EnableRule", "events:DisableRule"]
        Resource = aws_cloudwatch_event_rule.api_guard_check[0].arn
      }
    ]
  })
}

# The guard failed: it could not close or reopen the API, or tell the owner.
# It notifies through the alarms topic, which the guard ignores.
resource "aws_cloudwatch_metric_alarm" "apiguard_errors" {
  count = local.api_guard_count

  alarm_name          = "${var.application_name}-apiguard-errors"
  alarm_description   = "The API flood guard returned an error: the check-in API may not have been closed or reopened, or you were not told. Check the apiguard Lambda's CloudWatch logs, and dmh api status."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  dimensions          = { FunctionName = aws_lambda_function.apiguard_handler[0].function_name }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}
