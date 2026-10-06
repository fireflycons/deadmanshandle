# Deadman's Handle Application

A Golang AWS Lambda application that implements a deadman's handle - a mechanism to send documents if the owner fails to check in periodically.

## Architecture

The application uses hexagonal architecture with clear separation between:
- **Domain**: Core business logic (`internal/domain`)
- **Ports**: Interfaces for AWS services (`internal/ports`)
- **Adapters**: AWS service implementations (`internal/adapters`)
- **Handlers**: Lambda entry points (`internal/handlers`)

## Features

- **HTTP API Check-in**: Owner checks in via HTTP API to reset timeout
- **Scheduled Event Processing**: Daily EventBridge event checks timeout and sends warnings
- **Document Distribution**: On timeout, document from S3 is emailed to recipients. For additional security you should encrypt this document yourself prior to uploading to S3 and ensure the recipient(s) have the key and the knowledge to decrypt it upon receipt. A good choice is to put files into a [7-Zip](https://www.7-zip.org/) password protected archive which uses AES-256 encryption.
- **Warning Emails**: Owner is warned daily once the timeout is within `warnDays`
- **Alarms**: The owner is emailed if the daily run fails or does not run

## Project Structure

```
.
├── cmd/
│   ├── http/          # HTTP Lambda handler
│   └── scheduled/     # Scheduled event Lambda handler
├── internal/
│   ├── adapters/      # AWS service implementations
│   ├── config/        # Configuration models
│   ├── domain/        # Core business logic
│   ├── handlers/      # Lambda handlers
│   ├── mocks/         # Mock interfaces for testing
│   └── ports/         # Port interfaces
├── terraform/         # Infrastructure as Code
├── go.mod            # Go module definition
└── Makefile          # Build and deployment targets
```

## Configuration

Configuration is stored in AWS Parameter Store as JSON:

```json
{
    "owner": "owner@example.com",
    "recipients": [
        "recipient1@example.com",
        "recipient2@example.com"
    ],
    "resetDays": 30,
    "warnDays": 7,
    "timeout": "2030-01-01T00:00:00Z",
    "apiKey": "your-secure-api-key"
}
```

The config is validated by `terraform plan` and by both Lambdas; see
[DEPLOYMENT.md](DEPLOYMENT.md) for the rules.

Once the timeout passes, the scheduled Lambda emails the document to each
recipient and sends the owner a notice that it has done so. It records progress
in two extra fields, so the document goes out only once and a failed send is
retried on the next daily run without re-sending to anyone else:

- `sentTo`: recipients who have already been sent the document
- `ownerNotified`: whether the owner has been told

A check-in clears both. Leave them out of a config file you upload by hand.

## Building

### Prerequisites
- Go 1.26+ (see `go.mod`)
- Terraform 1.5+
- AWS CLI
- Make

### Build Steps

```bash
# Build the Lambda binaries
make build

# Run tests
make test

# Clean build artifacts
make clean
```

## Deployment

### Prerequisites
1. Create a configuration JSON file with your settings
2. Choose a `sender_email` whose domain you control. Terraform verifies the
   domain in SES with DKIM; request SES production access so that unverified
   recipients can receive mail
3. Run `make build`; Terraform deploys the zipped binaries

### Deploy with Terraform

```bash
# Initialize Terraform
cd terraform
terraform init

# Plan the deployment
terraform plan \
  -var="config_file_path=/path/to/config.json" \
  -var="sender_email=your-email@example.com"

# Apply the deployment
terraform apply \
  -var="config_file_path=/path/to/config.json" \
  -var="sender_email=your-email@example.com"

# Upload the document (the default key is document.pdf)
aws s3 cp your-document.pdf "s3://$(terraform output -raw document_bucket_name)/document.pdf"
```

After the first apply, the owner must confirm the SNS subscription email to
receive alarms. See [DEPLOYMENT.md](DEPLOYMENT.md) for details.

## API Usage

### Check-in Endpoint

**POST** `/checkin`

Headers:
- `x-api-key`: Your API key from configuration

Response:
```json
{
    "statusCode": 200,
    "message": "Check-in successful",
    "newTimeout": "2024-07-15T12:00:00Z"
}
```

## Testing

All core logic is thoroughly tested with mock interfaces, allowing unit tests without AWS dependencies:

```bash
# Run all tests
go test -v ./...

# Run specific test package
go test -v ./internal/domain

# Run with coverage
go test -cover ./...
```

## Environment Variables

- `CONFIG_PARAMETER_NAME`: Parameter Store path for configuration (both Lambdas)
- `SENDER_EMAIL`: Email address for notifications (scheduled Lambda)
- `DOCUMENT_BUCKET`: S3 bucket containing the document (scheduled Lambda)
- `DOCUMENT_KEY`: S3 object key for the document (scheduled Lambda)

## AWS Services Used

- **AWS Lambda**: Compute (2 functions)
- **API Gateway**: HTTP endpoint for check-ins
- **EventBridge**: Scheduled daily checks
- **Parameter Store**: Configuration storage
- **S3**: Document storage
- **SES**: Email delivery
- **CloudWatch**: Logs (14-day retention) and alarms
- **SNS**: Alarm emails to the owner

## Design Patterns

### Hexagonal Architecture
- Core domain logic is completely decoupled from AWS services
- Mock implementations for testing
- Easy to add new adapters or handlers

### Dependency Injection
- All dependencies injected at handler creation
- Testable by injecting mocks
- Clear separation of concerns

### Constant-Time Comparison
- API key validation uses `subtle.ConstantTimeCompare` to prevent timing attacks

## Security Considerations

- All S3 buckets have public access blocked
- The Parameter Store config is a SecureString, since it holds the API key
- S3 bucket versioning enabled
- API key validation uses constant-time comparison
- The SES sender domain is verified with DKIM by Terraform

## Troubleshooting

### Lambda Execution Errors
- Check CloudWatch logs for the Lambda functions; failures are logged with their error
- Verify IAM role has correct permissions
- Ensure the Parameter Store configuration is valid (see the rules in DEPLOYMENT.md)

### Email Not Sending
- Check the SES sender domain is verified (DKIM records in DNS)
- In the SES sandbox, only verified recipients receive mail; request production access
- Check SES sending limits
- Review email addresses in configuration

### EventBridge Not Triggering
- Verify Lambda has permission from EventBridge
- Check EventBridge rule is enabled
- The schedule expression is in UTC

## License

MIT
