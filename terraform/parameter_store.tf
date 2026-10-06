resource "aws_ssm_parameter" "config" {
  name  = "/${var.application_name}/${var.environment}/config"
  type  = "SecureString" # holds the API key; encrypted with the AWS-managed key alias/aws/ssm
  value = file(var.config_file_path)

  # The config file only seeds the parameter. After that the Lambda owns the
  # value (each check-in rewrites the timeout), so later applies must not
  # reset it from the file. To re-seed deliberately, run:
  #   terraform apply -replace=aws_ssm_parameter.config
  lifecycle {
    ignore_changes = [value]
  }

  tags = merge(var.tags, {
    Name = "Application Configuration"
  })
}
