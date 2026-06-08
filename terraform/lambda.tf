resource "aws_lambda_function" "http_handler" {
  filename      = data.archive_file.http_lambda.output_path
  function_name = "${var.application_name}-http"
  role          = aws_iam_role.lambda_role.arn
  handler       = "main"
  runtime       = "provided.al2023"
  timeout       = 30
  memory_size   = 256

  source_code_hash = data.archive_file.http_lambda.output_base64sha256

  environment {
    variables = {
      CONFIG_PARAMETER_NAME = aws_ssm_parameter.config.name
      SENDER_EMAIL          = var.sender_email
    }
  }

  tags = merge(var.tags, {
    Name = "HTTP Handler Lambda"
  })
}

resource "aws_lambda_function" "scheduled_handler" {
  filename      = data.archive_file.scheduled_lambda.output_path
  function_name = "${var.application_name}-scheduled"
  role          = aws_iam_role.lambda_role.arn
  handler       = "main"
  runtime       = "provided.al2023"
  timeout       = 60
  memory_size   = 256

  source_code_hash = data.archive_file.scheduled_lambda.output_base64sha256

  environment {
    variables = {
      CONFIG_PARAMETER_NAME = aws_ssm_parameter.config.name
      SENDER_EMAIL          = var.sender_email
      DOCUMENT_BUCKET       = aws_s3_bucket.document_bucket.id
      DOCUMENT_KEY          = var.document_key
    }
  }

  tags = merge(var.tags, {
    Name = "Scheduled Handler Lambda"
  })
}

# Archive files for Lambda deployment
data "archive_file" "http_lambda" {
  type        = "zip"
  source_file = "${path.module}/../bin/http"
  output_path = "${path.module}/build/http.zip"
}

data "archive_file" "scheduled_lambda" {
  type        = "zip"
  source_file = "${path.module}/../bin/scheduled"
  output_path = "${path.module}/build/scheduled.zip"
}
