resource "aws_ssm_parameter" "config" {
  name      = "/${var.application_name}/${var.environment}/config"
  type      = "String"
  value     = file(var.config_file_path)
  overwrite = true

  tags = merge(var.tags, {
    Name = "Application Configuration"
  })
}
