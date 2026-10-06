resource "aws_cloudwatch_event_rule" "daily_check" {
  name                = "${var.application_name}-daily-check"
  description         = "Daily check for deadman's handle"
  schedule_expression = var.schedule_expression

  tags = merge(var.tags, {
    Name = "Daily Check Rule"
  })
}

resource "aws_cloudwatch_event_target" "lambda_target" {
  rule      = aws_cloudwatch_event_rule.daily_check.name
  target_id = "DeadmansHandleLambda"
  arn       = aws_lambda_function.scheduled_handler.arn

  dead_letter_config {
    arn = aws_sqs_queue.dlq.arn
  }
}

resource "aws_lambda_permission" "eventbridge_invoke" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.scheduled_handler.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.daily_check.arn
}

resource "aws_sqs_queue" "dlq" {
  name                      = "${var.application_name}-dlq"
  message_retention_seconds = 86400

  tags = merge(var.tags, {
    Name = "Dead Letter Queue"
  })
}

# Without this, EventBridge cannot write failed events to the DLQ.
resource "aws_sqs_queue_policy" "dlq" {
  queue_url = aws_sqs_queue.dlq.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Principal = { Service = "events.amazonaws.com" }
        Action    = "sqs:SendMessage"
        Resource  = aws_sqs_queue.dlq.arn
        Condition = {
          ArnEquals = { "aws:SourceArn" = aws_cloudwatch_event_rule.daily_check.arn }
        }
      }
    ]
  })
}
