# SES sending identity for the domain of deployment.senderEmail in the config,
# verified with Easy DKIM.
# SES verifies the domain once the three DKIM CNAME records resolve. These are
# created in Route 53 when manage_dkim_dns_records is true; otherwise add the
# records from the ses_dkim_dns_records output at your DNS provider.
#
# With create_ses_identity false, an existing verified identity for the domain
# is used as it is, and neither it nor its DKIM records are managed here.
#
# Verifying the sender does not lift the SES sandbox: until the account has
# production access, mail is only delivered to verified recipients.

locals {
  sender_domain = try(split("@", local.sender_email)[1], "") # an invalid address fails a precondition in parameter_store.tf
}

resource "aws_sesv2_email_identity" "sender" {
  count = var.create_ses_identity ? 1 : 0

  email_identity = local.sender_domain

  tags = var.tags
}

data "aws_route53_zone" "sender" {
  count = var.create_ses_identity && var.manage_dkim_dns_records ? 1 : 0

  name         = local.sender_domain
  private_zone = false
}

# Easy DKIM always issues three tokens
resource "aws_route53_record" "dkim" {
  count = var.create_ses_identity && var.manage_dkim_dns_records ? 3 : 0

  zone_id = data.aws_route53_zone.sender[0].zone_id
  name    = "${aws_sesv2_email_identity.sender[0].dkim_signing_attributes[0].tokens[count.index]}._domainkey.${local.sender_domain}"
  type    = "CNAME"
  ttl     = 1800
  records = ["${aws_sesv2_email_identity.sender[0].dkim_signing_attributes[0].tokens[count.index]}.dkim.amazonses.com"]
}
