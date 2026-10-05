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
- **Document Distribution**: On timeout, document from S3 is emailed to recipients. For additional security you should encrypt this document yourself prior to uploading to S3 and ensure the recipient(s) have the key and the knowledge to decrypt it upon receipt. A good choice is to put files into a [7-Zip](https://www.7-zip.org/) password protected archive which uses AES-256 encrpytion.
- **Warning Emails**: Owner is warned when days to timeout falls below threshold

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
    "timeout": "2024-07-15T12:00:00Z",
    "apiKey": "your-secure-api-key"
}
```

Once the timeout passes, the scheduled Lambda emails the document to each
recipient and sends the owner a notice that it has done so. It records progress
in two extra fields, so the document goes out only once and a failed send is
retried on the next daily run without re-sending to anyone else:

- `sentTo`: recipients who have already been sent the document
- `ownerNotified`: whether the owner has been told

A check-in clears both. Leave them out of a config file you upload by hand.

## Building

### Prerequisites
- Go 1.21+
- Terraform 1.0+
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
2. Prepare an S3 bucket name for documents
3. Ensure SES is configured for your sender email

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
```

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

- `CONFIG_PARAMETER_NAME`: Parameter Store path for configuration
- `SENDER_EMAIL`: Email address for notifications
- `DOCUMENT_BUCKET`: S3 bucket containing the document
- `DOCUMENT_KEY`: S3 object key for the document

## AWS Services Used

- **AWS Lambda**: Compute (2 functions)
- **API Gateway**: HTTP endpoint for check-ins
- **EventBridge**: Scheduled daily checks
- **Parameter Store**: Configuration storage
- **S3**: Document storage
- **SES**: Email delivery
- **CloudWatch**: Logging and monitoring

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
- Parameter Store uses encryption
- S3 bucket versioning enabled
- API key validation uses constant-time comparison
- SES sender verification required (configure in AWS console)

## Troubleshooting

### Lambda Execution Errors
- Check CloudWatch logs for the Lambda functions
- Verify IAM role has correct permissions
- Ensure Parameter Store configuration is valid JSON

### Email Not Sending
- Verify SES sender email is verified in AWS
- Check SES sending limits
- Review email addresses in configuration

### EventBridge Not Triggering
- Verify Lambda has permission from EventBridge
- Check EventBridge rule is enabled
- Review scheduled rule timezone settings

## License

MIT
