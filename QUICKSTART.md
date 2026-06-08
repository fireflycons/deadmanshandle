# Quick Start Guide

Get up and running with the Deadman's Handle application in 10 minutes.

## 1. Prerequisites (5 minutes)

Install required tools:

```bash
# Install Go 1.21+
brew install go  # macOS
# or download from https://golang.org/dl/

# Install Terraform 1.0+
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
ls -la bin/
# Should show: http, scheduled
```

## 3. Prepare Configuration (1 minute)

```bash
# Copy example configuration
cp config.example.json config.json

# Edit with your values
nano config.json
```

Update these fields:
- `owner`: Your email address
- `recipients`: Email addresses to send document to
- `apiKey`: Generate a random secure key

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
- `sender_email`: A verified SES email address

```bash
# Initialize Terraform
terraform init

# Deploy to AWS
terraform apply

# Note the outputs:
# - api_endpoint: Your check-in URL
# - document_bucket_name: S3 bucket for documents
```

## 5. Test the API (< 1 minute)

```bash
# Save these for testing
API_ENDPOINT="https://your-endpoint.execute-api.region.amazonaws.com/dev"
API_KEY="your-api-key"

# Test check-in
curl -X POST "${API_ENDPOINT}/checkin" \
  -H "x-api-key: ${API_KEY}"

# Expected response:
# {
#   "statusCode": 200,
#   "message": "Check-in successful",
#   "newTimeout": "2024-..."
# }
```

## What's Next?

### Upload Your Document
```bash
aws s3 cp important-document.pdf \
  s3://your-bucket-name/document.pdf
```

### Test Scheduled Event (Optional)
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
  --overwrite
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

### "SES Email Address Not Verified"
- The sender email must be verified in SES
- Go to SES console and verify your email address

### "Failed to retrieve configuration"
- Check the `config_file_path` in terraform.tfvars
- Verify Parameter Store has the configuration

### "Failed to get document"
- Upload the document: `aws s3 cp document.pdf s3://bucket-name/document.pdf`
- Verify bucket name and document key match

## Cleanup

Remove everything from AWS:

```bash
cd terraform
terraform destroy
```

This removes:
- Lambda functions
- API Gateway
- S3 bucket
- EventBridge rule
- Parameter Store configuration
- IAM roles

## Architecture Overview

```
┌──────────────────┐
│   Your Emails    │
└────────┬─────────┘
         │
    ┌────┴──────┐
    │            │
┌───▼──┐    ┌───▼───────┐
│ API  │    │ EventBridge
│Gate  │    │ (Daily)
└───┬──┘    └───┬────────┘
    │            │
    └────┬───────┘
         │
    ┌────▼────────────────────┐
    │  Lambda Functions       │
    │  (Hexagonal Arch)       │
    └────┬───────────────────┘
         │
    ┌────┴────┬──────────┬─────────┐
    │          │          │         │
┌───▼──┐  ┌───▼──┐  ┌───▼──┐  ┌──▼───┐
│S3 Doc│  │Param │  │  SES │  │  API │
│Store │  │Store │  │Email │  │ Keys │
└──────┘  └──────┘  └──────┘  └──────┘
```

## Next Steps

1. **Production Setup**: Configure SNS alarms, enhanced monitoring
2. **Custom Logic**: Extend domain service with additional business rules
3. **Multiple Documents**: Support different documents per owner
4. **Web Dashboard**: Add UI for configuration management
5. **Multi-Region**: Deploy to multiple AWS regions for redundancy

## Support

For issues:
1. Check CloudWatch logs
2. Review DEPLOYMENT.md for detailed guides
3. Check PROJECT_STRUCTURE.md for architecture details
4. Review README.md for feature documentation

## Cost Estimation

For typical monthly usage:
- ~100 check-ins: ~$0.02
- ~30 scheduled events: ~$0.01
- API Gateway: ~$0.10
- SES emails: ~$0.05
- **Total: ~$0.20/month**

(Costs vary by AWS region and usage)

---

**Time to Production: ~15 minutes** ⚡

You now have a fully functional Deadman's Handle application running on AWS Lambda!
