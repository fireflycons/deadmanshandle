# A policy for recipients with AWS access. Attached to a recipient's IAM user
# or role, it lets open-document read the document password from Parameter
# Store instead of asking for it. It grants nothing else: not the config,
# which holds the API key a recipient could check in with.

resource "aws_iam_policy" "recipient" {
  name        = "DMH-${var.application_name}-Recipient"
  description = "Lets a recipient's open-document read the dead man's handle document password (/${var.application_name}/document-password). Grants nothing else."

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        # Built from the name: only dmh creates the parameter, so it may not
        # exist yet. SecureStrings use the AWS-managed aws/ssm key, so no KMS
        # permission is needed.
        Effect   = "Allow"
        Action   = ["ssm:GetParameter"]
        Resource = "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter/${var.application_name}/document-password"
      }
    ]
  })

  tags = var.tags
}
