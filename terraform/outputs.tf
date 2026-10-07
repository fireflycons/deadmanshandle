output "api_endpoint" {
  description = "HTTP API Gateway endpoint"
  value       = trimsuffix(aws_apigatewayv2_stage.http_api_stage.invoke_url, "/") # the $default stage's URL ends in "/"
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

output "config_parameter_name" {
  description = "Parameter Store configuration parameter name"
  value       = aws_ssm_parameter.config.name
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
