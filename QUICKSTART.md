# Quick Start Guide

Get up and running with the Deadman's Handle application in about 10 minutes.
SES production access, which real recipients need, is a separate request to
AWS that can take a day; see step 4.

## 1. Prerequisites (5 minutes)

Install required tools:

```bash
# Install Go 1.26+ (see go.mod)
brew install go  # macOS
# or download from https://golang.org/dl/

# Install Terraform 1.5+
brew install terraform  # macOS
# or download from https://www.terraform.io/downloads.html

# Install AWS CLI
brew install awscli  # macOS
# or follow https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html

# Configure AWS credentials
aws configure
```

## 2. Clone and Build (2 minutes)

```bash
# Clone the repository
git clone <repo-url>
cd deadmanshandle

# Download dependencies
go mod download

# Build the Lambda binaries
make build

# Verify build succeeded
ls bin/http bin/scheduled
# Each should contain: bootstrap
```

## 3. Prepare Configuration (1 minute)

```bash
# Copy example configuration
cp config.example.json config.json

# Edit with your values
nano config.json
```

Update these fields:
- `owner`: Your email address (it also receives the alarms)
- `recipients`: Email addresses to send document to
- `timeout`: Your first deadline, in the future (RFC 3339, e.g. `2030-01-01T00:00:00Z`)
- `apiKey`: Generate a random secure key, e.g. `openssl rand -hex 32`
- `resetDays`, `warnDays`: Days each check-in buys, and how many days before
  the deadline the warnings start (`warnDays` must be less than `resetDays`)

## 4. Deploy (2 minutes)

```bash
# Navigate to terraform directory
cd terraform

# Copy and edit variables
cp terraform.tfvars.example terraform.tfvars
nano terraform.tfvars
```

Update these fields:
- `config_file_path`: Path to your config.json
- `sender_email`: The address emails come from. Terraform verifies its domain
  in SES with DKIM
- `manage_dkim_dns_records`: `true` if the domain's DNS is in Route 53 in this
  account (Terraform adds the records), otherwise `false` and add the records
  from the `ses_dkim_dns_records` output yourself

Request SES production access in the SES console; until then, mail only reaches
verified addresses.

```bash
# Initialize Terraform
terraform init

# Deploy to AWS
terraform apply

# Note the outputs:
# - api_endpoint: Your check-in URL
# - document_bucket_name: S3 bucket for documents
```

The owner receives an email asking them to confirm the alarm subscription;
confirm it so that failures reach you.

## 5. Test the API (< 1 minute)

```bash
# Save these for testing
API_ENDPOINT="$(terraform output -raw api_endpoint)"
API_KEY="your-api-key"

# Test check-in
curl -X POST "${API_ENDPOINT}/checkin" \
  -H "x-api-key: ${API_KEY}"

# Expected response:
# {
#   "statusCode": 200,
#   "message": "Check-in successful",
#   "newTimeout": "<now + resetDays, e.g. 2026-11-05T09:30:00Z>"
# }
```

The API allows about one request a minute; a quick second call gets HTTP 429.

## What's Next?

### Upload Your Document
```bash
# From the terraform directory; the key must match document_key (default document.pdf)
aws s3 cp important-document.pdf \
  "s3://$(terraform output -raw document_bucket_name)/document.pdf"
```

The handle cannot send anything until the document is uploaded.

### Test Scheduled Event (Optional)

This is a real run: it sends the document if the timeout has passed, or a
warning if it is within `warnDays`.

```bash
aws lambda invoke \
  --function-name deadmanshandle-scheduled \
  response.json
cat response.json
```

### Monitor in CloudWatch
```bash
# Watch HTTP handler logs
aws logs tail /aws/lambda/deadmanshandle-http --follow

# Watch scheduled handler logs
aws logs tail /aws/lambda/deadmanshandle-scheduled --follow
```

### Update Configuration
```bash
# Edit config.json
nano config.json

# Upload to Parameter Store
aws ssm put-parameter \
  --name /deadmanshandle/dev/config \
  --value file://config.json \
  --overwrite \
  --type SecureString
```

## Running Tests Locally

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

# Run specific test
go test -v ./internal/domain
```

## Common Issues

### "Invalid API key" response
- Verify you copied the `apiKey` from your config.json correctly
- Include the `x-api-key` header in your curl request

### "Email address is not verified" in the scheduled Lambda's logs
- Check the sender domain is verified: its DKIM records must be in DNS
- In the SES sandbox, every recipient must be verified too; request production access

### "Failed to retrieve configuration" or "Failed to parse configuration"
- The HTTP Lambda's logs give the cause
- Check the parameter exists and that the config follows the rules in DEPLOYMENT.md

### "NoSuchKey" in the scheduled Lambda's logs
- Upload the document to the bucket from `terraform output document_bucket_name`
- Verify the object key matches `document_key` (default `document.pdf`)

## Cleanup

Remove everything from AWS:

```bash
cd terraform
terraform destroy
```

This removes everything Terraform created, including the stored config and
the SES identity. The versioned S3 bucket must be emptied (all object versions)
first, or `terraform destroy` fails.

## Architecture Overview

```
  Owner ── POST /checkin ──► API Gateway ──► HTTP Lambda ─────────┐
                                                                  ▼
                                                   Parameter Store (config,
                                                   timeout, API key)
                                                                  ▲
  EventBridge (daily) ─────────────────────► Scheduled Lambda ────┘
                                                │            │
                                     S3 (document)     SES ──► owner / recipients

  CloudWatch alarms ──► SNS ──► owner
```

## Next Steps

1. **Custom Logic**: Extend domain service with additional business rules
2. **Multiple Documents**: Support different documents per owner
3. **Web Dashboard**: Add UI for configuration management
4. **Multi-Region**: Deploy to multiple AWS regions for redundancy

## Support

For issues:
1. Check CloudWatch logs
2. Review DEPLOYMENT.md for detailed guides
3. Check PROJECT_STRUCTURE.md for architecture details
4. Review README.md for feature documentation

## Cost Estimation

Well under $1 a month, mostly the three CloudWatch alarms (about $0.10 each).
See DEPLOYMENT.md for the breakdown; costs vary by region.

---

You now have a fully functional Deadman's Handle application running on AWS Lambda!
