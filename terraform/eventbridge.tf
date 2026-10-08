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

# Uploads of the document, which may be unauthorised changes. Deletions are
# left to the daily run's missing-document check.
resource "aws_cloudwatch_event_rule" "document_changed" {
  name        = "${var.application_name}-document-changed"
  description = "Uploads of the deadman's handle document"

  event_pattern = jsonencode({
    source        = ["aws.s3"]
    "detail-type" = ["Object Created"]
    detail = {
      bucket = { name = [aws_s3_bucket.document_bucket.id] }
      object = { key = [local.document_key] }
    }
  })

  tags = merge(var.tags, {
    Name = "Document Changed Rule"
  })
}

resource "aws_cloudwatch_event_target" "docwatch_target" {
  rule      = aws_cloudwatch_event_rule.document_changed.name
  target_id = "DeadmansHandleDocWatch"
  arn       = aws_lambda_function.docwatch_handler.arn

  dead_letter_config {
    arn = aws_sqs_queue.dlq.arn
  }
}

resource "aws_lambda_permission" "eventbridge_invoke_docwatch" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.docwatch_handler.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.document_changed.arn
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
          ArnEquals = { "aws:SourceArn" = [aws_cloudwatch_event_rule.daily_check.arn, aws_cloudwatch_event_rule.document_changed.arn] }
        }
      }
    ]
  })
}
