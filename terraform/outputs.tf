output "api_endpoint" {
  description = "HTTP API Gateway endpoint"
  value       = local.api_endpoint
}

output "document_bucket_name" {
  description = "S3 bucket for storing documents"
  value       = aws_s3_bucket.document_bucket.id
}

output "http_lambda_function_name" {
  description = "HTTP Handler Lambda function name"
  value       = aws_lambda_function.http_handler.function_name
}

output "scheduled_lambda_function_name" {
  description = "Scheduled Handler Lambda function name"
  value       = aws_lambda_function.scheduled_handler.function_name
}

output "docwatch_lambda_function_name" {
  description = "Document Watch Lambda function name"
  value       = aws_lambda_function.docwatch_handler.function_name
}

output "alarmpush_lambda_function_name" {
  description = "Alarm Push Lambda function name (null when push is off)"
  value       = try(aws_lambda_function.alarmpush_handler[0].function_name, null)
}

output "push_platform_application_arn" {
  description = "SNS platform application for push to the Android app (null when push is off)"
  value       = try(aws_sns_platform_application.fcm[0].arn, null)
}

output "state_table_name" {
  description = "DynamoDB table holding the timeout and delivery state"
  value       = aws_dynamodb_table.state.name
}

output "config_parameter_name" {
  description = "Parameter Store configuration parameter name"
  value       = aws_ssm_parameter.config.name
}

output "api_endpoint_parameter_name" {
  description = "Parameter Store parameter holding the API endpoint, read by dmh qr -aws"
  value       = aws_ssm_parameter.api_endpoint.name
}

output "alarm_topic_arn" {
  description = "SNS topic that receives the scheduled run's alarms"
  value       = aws_sns_topic.alarms.arn
}

output "ses_dkim_dns_records" {
  description = "DKIM CNAME records that verify the sender domain (created automatically when manage_dkim_dns_records is true; empty when create_ses_identity is false)"
  value = [
    for token in try(aws_sesv2_email_identity.sender[0].dkim_signing_attributes[0].tokens, []) : {
      name  = "${token}._domainkey.${local.sender_domain}"
      type  = "CNAME"
      value = "${token}.dkim.amazonses.com"
    }
  ]
}

output "eventbridge_rule_name" {
  description = "EventBridge rule name for daily check"
  value       = aws_cloudwatch_event_rule.daily_check.name
}

output "recipient_policy_arn" {
  description = "IAM policy to attach to a recipient's user or role, so open-document reads the document password from Parameter Store"
  value       = aws_iam_policy.recipient.arn
}
