resource "aws_s3_bucket" "document_bucket" {
  bucket = local.document_bucket_name != "" ? local.document_bucket_name : "${var.application_name}-documents-${data.aws_caller_identity.current.account_id}-${var.aws_region}"

  tags = merge(var.tags, {
    Name = "Document Bucket"
  })
}

resource "aws_s3_bucket_public_access_block" "document_bucket_pab" {
  bucket = aws_s3_bucket.document_bucket.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Turning versioning off suspends it rather than removing the resource, which
# would leave it enabled on a bucket that already has it.
resource "aws_s3_bucket_versioning" "document_bucket_versioning" {
  bucket = aws_s3_bucket.document_bucket.id

  versioning_configuration {
    status = var.document_versioning ? "Enabled" : "Suspended"
  }
}

# Old versions of the document are kept for 30 days after being replaced or
# deleted.
resource "aws_s3_bucket_lifecycle_configuration" "document_bucket_lifecycle" {
  count  = var.document_versioning ? 1 : 0
  bucket = aws_s3_bucket.document_bucket.id

  rule {
    id     = "expire-noncurrent-versions"
    status = "Enabled"

    filter {}

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }

  # The rule must not be applied before versioning is configured.
  depends_on = [aws_s3_bucket_versioning.document_bucket_versioning]
}

resource "aws_s3_bucket_server_side_encryption_configuration" "document_bucket_encryption" {
  bucket = aws_s3_bucket.document_bucket.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}
