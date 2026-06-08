# Project Structure

```
deadmanshandle/
│
├── cmd/                           # Lambda entry points
│   ├── http/
│   │   └── main.go               # HTTP API Gateway handler
│   └── scheduled/
│       └── main.go               # EventBridge scheduled handler
│
├── internal/                           # Application packages
│   ├── adapters/                 # AWS service implementations
│   │   ├── ssm_config_store.go   # Parameter Store adapter
│   │   ├── s3_document_store.go  # S3 storage adapter
│   │   ├── ses_email_sender.go   # SES email adapter
│   │   └── api_key_validator.go  # API key validation
│   │
│   ├── config/                   # Configuration models
│   │   └── config.go             # Config structure and utilities
│   │
│   ├── domain/                   # Core business logic
│   │   ├── service.go            # Main service with business rules
│   │   └── service_test.go       # Domain logic tests
│   │
│   ├── handlers/                 # Lambda handlers
│   │   ├── http_handler.go       # HTTP API handler
│   │   ├── http_handler_test.go  # HTTP handler tests
│   │   └── scheduled_event_handler.go  # EventBridge handler
│   │
│   ├── mocks/                    # Mock implementations for testing
│   │   └── mocks.go              # Mock interfaces
│   │
│   └── ports/                    # Port interfaces (hexagonal architecture)
│       └── ports.go              # Service interfaces
│
├── terraform/                     # Infrastructure as Code
│   ├── provider.tf               # AWS provider configuration
│   ├── variables.tf              # Input variables
│   ├── data.tf                   # Data sources
│   ├── iam.tf                    # IAM roles and policies
│   ├── lambda.tf                 # Lambda functions
│   ├── api_gateway.tf            # HTTP API Gateway
│   ├── s3.tf                     # S3 bucket configuration
│   ├── parameter_store.tf        # SSM Parameter Store
│   ├── eventbridge.tf            # EventBridge rules
│   ├── outputs.tf                # Output values
│   ├── terraform.tfvars.example  # Example variables file
│   └── build/                    # Build artifacts (generated)
│
├── bin/                          # Compiled binaries (generated)
│   ├── http                      # HTTP handler binary
│   └── scheduled                 # Scheduled handler binary
│
├── go.mod                        # Go module definition
├── go.sum                        # Go dependencies checksums
├── Makefile                      # Build automation
├── Dockerfile                    # Container image definition
├── docker-compose.yml            # Local development environment
├── .gitignore                    # Git ignore rules
│
├── README.md                     # Project documentation
├── DEPLOYMENT.md                 # Deployment guide
├── spec.md                       # Project specification
├── config.example.json           # Example configuration
└── config.json                   # Configuration (git-ignored)
```

## Directory Descriptions

### `cmd/` - Command/Entry Points
- **http**: HTTP API Gateway Lambda handler
  - Handles check-in requests
  - Validates API keys
  - Updates configuration timeout
  
- **scheduled**: EventBridge scheduled event handler
  - Processes daily checks
  - Sends warning emails
  - Distributes documents on timeout

### `internal/adapters/` - AWS Service Adapters
Implements the port interfaces using AWS SDK:
- **SSMConfigStore**: Reads/writes configuration from Parameter Store
- **S3DocumentStore**: Retrieves documents from S3
- **SESEmailSender**: Sends emails via SES with attachments
- **SimpleAPIKeyValidator**: Validates API keys with constant-time comparison

### `internal/config/` - Configuration Models
- Defines the Config struct matching Parameter Store JSON
- Provides parsing and serialization utilities
- Includes timeout calculation logic

### `internal/domain/` - Core Business Logic
- **DeadmansHandleService**: Main service encapsulating all business logic
  - `CheckIn()`: Process owner check-in, update timeout
  - `ProcessScheduledEvent()`: Determine what emails to send
  - `DaysUntilTimeout()`: Calculate days remaining
  - `IsTimeoutPassed()`: Check if deadline passed
  
- Tests using time injection for deterministic testing

### `internal/handlers/` - Lambda Handlers
- **HTTPHandler**: Processes API Gateway requests
  - Validates API key header
  - Calls domain service
  - Updates configuration
  - Returns JSON response
  
- **ScheduledEventHandler**: Processes EventBridge events
  - Fetches configuration
  - Calls domain service
  - Retrieves document if needed
  - Sends emails

### `internal/mocks/` - Testing Mocks
Mock implementations of all port interfaces for unit testing:
- **MockConfigStore**: In-memory configuration storage
- **MockDocumentStore**: In-memory document storage
- **MockEmailSender**: Captures sent emails without sending
- **MockAPIKeyValidator**: Simple key comparison

### `internal/ports/` - Port Interfaces (Hexagonal Architecture)
Defines contracts for external dependencies:
- **ConfigStore**: Get/set configuration
- **DocumentStore**: Retrieve documents
- **EmailSender**: Send emails (single and batch)
- **APIKeyValidator**: Validate API keys

### `terraform/` - Infrastructure as Code
Terraform modules defining AWS resources:
- **provider.tf**: AWS provider configuration
- **iam.tf**: Lambda execution role and policies
- **lambda.tf**: Lambda function definitions and archiving
- **api_gateway.tf**: HTTP API and routes
- **s3.tf**: S3 bucket with security settings
- **parameter_store.tf**: Configuration parameter
- **eventbridge.tf**: Scheduled rules and targets
- **variables.tf**: Input variables
- **outputs.tf**: Output values
- **data.tf**: Data sources (AWS account ID)

## Architecture Pattern: Hexagonal Architecture

The application follows hexagonal architecture (ports & adapters):

```
┌─────────────────────────────────────┐
│       Lambda Entry Points           │
│  (cmd/http, cmd/scheduled)          │
└──────────────┬──────────────────────┘
               │
┌──────────────┴──────────────────────┐
│        Handlers Layer               │
│   (internal/handlers)                    │
└──────────────┬──────────────────────┘
               │
┌──────────────┴──────────────────────┐
│    Core Domain Logic                │
│ (internal/domain - Independent)          │
└──────────────┬──────────────────────┘
               │
┌──────────────┴──────────────────────┐
│      Port Interfaces                │
│      (internal/ports)                    │
└──────────────┬──────────────────────┘
               │
    ┌──────────┴──────────┐
    │                     │
┌───┴────┐        ┌──────┴──┐
│Adapters│        │  Mocks  │
│ (AWS)  │        │ (Tests) │
└────────┘        └─────────┘
```

Benefits:
- Core logic is completely independent of AWS
- Easy to test with mock implementations
- Easy to add new adapters or handlers
- Clear separation of concerns

## Testing Strategy

### Unit Tests
Located in `*_test.go` files within each package:
- Domain service tests with time injection
- HTTP handler tests with mocks
- Configuration parsing tests

### Test Execution
```bash
go test -v ./...           # Run all tests
go test -cover ./...       # With coverage
go test -run TestName ./internal/domain  # Specific test
```

### Mock Usage
All tests use mock implementations from `internal/mocks/`:
```go
configStore := mocks.NewMockConfigStore()
emailSender := mocks.NewMockEmailSender()
validator := mocks.NewMockAPIKeyValidator("key")
```

## Build Process

### Local Development
```bash
make build      # Builds both handlers
make test       # Runs tests
make clean      # Removes artifacts
```

### Production Build
- Compiles for Linux ARM64 (Lambda architecture)
- Creates optimized binaries
- Packages as ZIP for Lambda deployment

### Terraform Deployment
- Archives binaries into ZIP files
- Creates S3-based deployment packages
- Manages all AWS infrastructure

## Environment Variables

### Runtime (Lambda)
- `CONFIG_PARAMETER_NAME`: Parameter Store path
- `SENDER_EMAIL`: SES verified sender
- `DOCUMENT_BUCKET`: S3 bucket name
- `DOCUMENT_KEY`: S3 object key

### Build/Deployment
- `GOOS`: Operating system (linux for Lambda)
- `GOARCH`: Architecture (arm64 for Lambda)
- AWS credentials via standard AWS CLI

## Dependencies

### Go Dependencies (Standard Library Focus)
- `github.com/aws/aws-lambda-go`: Lambda runtime
- `github.com/aws/aws-sdk-go-v2`: AWS SDK
- Standard library: encoding/json, time, context, etc.

### External Tools
- Go 1.21+
- Terraform 1.0+
- AWS CLI
- Make (optional)
- Docker (optional, for local development)

## Security Considerations

1. **API Key Validation**: Uses `subtle.ConstantTimeCompare` to prevent timing attacks
2. **S3 Security**: All public access blocked, versioning enabled
3. **Parameter Store**: Configuration encrypted at rest
4. **IAM Least Privilege**: Lambda role only has necessary permissions
5. **SES Verification**: Requires verified sender email
6. **Input Validation**: JSON parsing validates all configuration fields

## Deployment Flow

1. Developer runs `make build` to compile binaries
2. Developer runs Terraform `init`, `plan`, `apply`
3. Terraform archives binaries and creates Lambda functions
4. API Gateway routes HTTP requests to HTTP handler
5. EventBridge triggers scheduled handler daily
6. Handlers use adapters to call AWS services
7. Core domain logic remains independent and testable
