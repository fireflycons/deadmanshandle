# Alerting. A dead man's handle that fails silently is the worst outcome, so
# every failure mode of the daily run notifies the owner via SNS.
# The email subscription must be confirmed from the link AWS sends.

locals {
  # Read from the seed file on every apply. If the owner changes, update the
  # file as well as the parameter so the subscription follows.
  owner_email = jsondecode(file(var.config_file_path)).owner
}

resource "aws_sns_topic" "alarms" {
  name = "${var.application_name}-alarms"

  tags = var.tags
}

resource "aws_sns_topic_subscription" "owner" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = local.owner_email
}

# The run failed: it crashed, could not read config/document, or a send failed.
resource "aws_cloudwatch_metric_alarm" "scheduled_errors" {
  alarm_name          = "${var.application_name}-scheduled-errors"
  alarm_description   = "The daily dead man's handle run returned an error. Check its CloudWatch logs."
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  dimensions          = { FunctionName = aws_lambda_function.scheduled_handler.function_name }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}

# The run did not happen at all in the last day (rule disabled, permission
# removed, throttled). Lambda emits no datapoints when not invoked, so missing
# data is breaching. Expect this to fire once after the first deploy, until
# the first scheduled run.
resource "aws_cloudwatch_metric_alarm" "scheduled_not_run" {
  alarm_name          = "${var.application_name}-scheduled-not-run"
  alarm_description   = "The daily dead man's handle run has not been invoked in the last 24 hours."
  namespace           = "AWS/Lambda"
  metric_name         = "Invocations"
  dimensions          = { FunctionName = aws_lambda_function.scheduled_handler.function_name }
  statistic           = "Sum"
  period              = 86400
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "breaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}

# EventBridge could not deliver the event to the Lambda.
resource "aws_cloudwatch_metric_alarm" "dlq_messages" {
  alarm_name          = "${var.application_name}-dlq-messages"
  alarm_description   = "EventBridge failed to invoke the daily run; the event is in the DLQ."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  dimensions          = { QueueName = aws_sqs_queue.dlq.name }
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  tags = var.tags
}
