# Development Guide

This guide covers how to develop and extend the Deadman's Handle application.

## Local Development Setup

### 1. Install Dependencies

```bash
# Go 1.21+
brew install go

# Git (for version control)
brew install git

# Optional: Make (for build automation)
brew install make

# Optional: Docker (for local testing)
brew install docker
```

### 2. Clone and Setup

```bash
git clone <repository>
cd deadmanshandle

# Download Go dependencies
go mod download
go mod tidy

# Verify build
go build -v ./...
```

### 3. Project Layout

```
internal/
  ├── config/       # Configuration models - START HERE
  ├── domain/       # Business logic - Main development focus
  ├── ports/        # Interfaces - Define new capabilities here
  ├── adapters/     # AWS implementations - Add AWS calls here
  ├── handlers/     # Lambda entry points
  └── mocks/        # Test doubles
cmd/
  ├── http/         # HTTP Lambda entry point
  └── scheduled/    # Scheduled Lambda entry point
```

## Development Workflow

### Adding a New Feature

1. **Update Config Model** (if needed)
   ```bash
   # Edit internal/config/config.go
   # Add new fields to Config struct
   # Add tests in internal/config/config_test.go
   ```

2. **Add Domain Logic**
   ```bash
   # Edit internal/domain/service.go
   # Add new methods to DeadmansHandleService
   # Add tests in internal/domain/service_test.go
   # Tests should use time injection for determinism
   ```

3. **Update Ports** (if new AWS service needed)
   ```bash
   # Edit internal/ports/ports.go
   # Add new interface (e.g., NotificationService)
   ```

4. **Implement Adapter**
   ```bash
   # Create internal/adapters/new_service.go
   # Implement the interface using AWS SDK
   ```

5. **Add Mock** (for testing)
   ```bash
   # Edit internal/mocks/mocks.go
   # Add MockNewService implementation
   ```

6. **Update Handler**
   ```bash
   # Edit internal/handlers/http_handler.go or scheduled_event_handler.go
   # Use new service capability
   # Add tests in handler_test.go
   ```

7. **Test End-to-End**
   ```bash
   go test -v ./...
   ```

### Code Style Guide

#### Naming Conventions
```go
// Interfaces: PascalCase, end with -er, -or, -ing
type EmailSender interface {}
type ConfigStore interface {}

// Structs: PascalCase
type Config struct {}
type HTTPHandler struct {}

// Methods: PascalCase
func (s *Service) CheckIn(cfg *Config) error {}

// Private functions: camelCase
func (s *Service) validateConfig(cfg *Config) error {}

// Constants: UPPER_SNAKE_CASE
const DEFAULT_TIMEOUT_DAYS = 30
```

#### Documentation
```go
// Package comment at top of file
// Package config provides configuration models for the application
package config

// Type comment before struct
// Config represents the application configuration
type Config struct {
    // Field comment
    Owner string // Email of the account owner
}

// Method comment
// CheckIn processes an owner check-in and returns the updated configuration
func (s *Service) CheckIn(cfg *Config) (*Config, error) {
    // ...
}
```

#### Error Handling
```go
// Return errors, don't ignore them
if err := s.emailSender.SendEmail(ctx, to, subject, body, attachments); err != nil {
    return fmt.Errorf("failed to send email: %w", err)
}

// Provide context in errors
if !s.keyValidator.ValidateAPIKey(ctx, key, expected) {
    return fmt.Errorf("invalid API key: key does not match expected value")
}
```

### Testing Guidelines

#### Unit Tests
- Located in `*_test.go` files alongside code
- Use table-driven tests for multiple scenarios
- Always use mock implementations from `internal/mocks/`
- Inject time for deterministic testing

```go
func TestCheckIn(t *testing.T) {
    // Setup
    now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
    service := domain.NewDeadmansHandleServiceWithTime(now)
    cfg := createTestConfig()
    
    // Execute
    result, err := service.CheckIn(cfg)
    
    // Verify
    if err != nil {
        t.Fatalf("CheckIn failed: %v", err)
    }
    if !result.Timeout.Equal(now.AddDate(0, 0, 30)) {
        t.Error("Timeout not updated correctly")
    }
}
```

#### Test Naming
```go
// Format: Test<FunctionName><Scenario>
func TestCheckInWithValidConfig(t *testing.T) {}
func TestCheckInWithExpiredTimeout(t *testing.T) {}
func TestProcessScheduledEventSendsWarning(t *testing.T) {}
```

#### Mock Usage
```go
// Create mocks
configStore := mocks.NewMockConfigStore()
emailSender := mocks.NewMockEmailSender()

// Set mock data
configStore.Data["param"] = []byte(`{"owner":"test@example.com"}`)

// Execute code
handler.Handle(ctx, request)

// Verify mock calls
if len(emailSender.SentEmails) == 0 {
    t.Error("Expected email to be sent")
}
```

### Running Tests

```bash
# All tests
go test -v ./...

# With coverage
go test -cover ./...

# Specific package
go test -v ./internal/domain

# Specific test
go test -run TestCheckIn ./internal/domain

# Watch mode (requires entr)
ls internal/**/*.go | entr go test -v ./...
```

### Building

```bash
# Local build
go build ./cmd/http
go build ./cmd/scheduled

# Linux ARM64 (Lambda)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/http/bootstrap ./cmd/http
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/scheduled/bootstrap ./cmd/scheduled

# Using Makefile
make build
```

### Debugging

#### Enable Verbose Logging
```bash
# Export debug environment variable
export DEBUG=1
export DEBUG_LEVEL=2

# Add debug output in code
if os.Getenv("DEBUG") != "" {
    log.Printf("Debug: processing config: %+v\n", cfg)
}
```

#### Using Go Debugger (Delve)

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a test
dlv test ./internal/domain -- -test.run TestCheckIn

# Debug main application
dlv debug ./cmd/http
```

#### CloudWatch Logs

```bash
# View HTTP handler logs
aws logs tail /aws/lambda/deadmanshandle-http --follow

# View specific time range
aws logs filter-log-events \
  --log-group-name /aws/lambda/deadmanshandle-http \
  --start-time 1000 \
  --end-time 2000
```

## Extending the Application

### Adding a New AWS Service

1. **Define Port Interface**
   ```go
   // internal/ports/ports.go
   type NewService interface {
       DoSomething(ctx context.Context, ...) error
   }
   ```

2. **Implement Adapter**
   ```go
   // internal/adapters/new_service.go
   type AWSNewServiceAdapter struct {
       client *service.Client
   }
   
   func (a *AWSNewServiceAdapter) DoSomething(ctx context.Context, ...) error {
       // AWS SDK call
   }
   ```

3. **Add Mock**
   ```go
   // internal/mocks/mocks.go
   type MockNewService struct {
       // state for testing
   }
   
   func (m *MockNewService) DoSomething(ctx context.Context, ...) error {
       // mock implementation
   }
   ```

4. **Integrate with Domain**
   ```go
   // internal/domain/service.go
   type DeadmansHandleService struct {
       newService ports.NewService
   }
   ```

### Adding a New Handler

1. **Create handler file**
   ```bash
   touch internal/handlers/new_handler.go
   ```

2. **Define handler struct**
   ```go
   type NewHandler struct {
       // dependencies via interfaces
       configStore ports.ConfigStore
       service     *domain.DeadmansHandleService
   }
   ```

3. **Implement Handle method**
   ```go
   func (h *NewHandler) Handle(ctx context.Context, event interface{}) error {
       // implementation
   }
   ```

4. **Create Lambda entry point**
   ```bash
   touch cmd/new_handler/main.go
   ```

### Modifying Configuration

1. **Update Config struct**
   ```go
   type Config struct {
       // ... existing fields
       NewField string `json:"newField"`
   }
   ```

2. **Create migration guide** (in docs)
3. **Add tests** for new field
4. **Update documentation** in README.md

## Performance Optimization

### Cold Start Optimization
```go
// Reuse clients at module level
var ssmClient *ssm.Client

func init() {
    // Initialize once
    cfg, _ := config.LoadDefaultConfig(context.Background())
    ssmClient = ssm.NewFromConfig(cfg)
}
```

### Memory Optimization
- Lambda memory directly affects CPU allocation and cost
- Typical needs: 256MB (minimal), 512MB (recommended)
- Monitor CloudWatch metrics for actual usage

### Connection Pooling
```go
// Reuse HTTP clients and AWS service clients
var sesClient *ses.Client

func init() {
    cfg, _ := config.LoadDefaultConfig(context.Background())
    sesClient = ses.NewFromConfig(cfg)
}
```

## Security Best Practices

### API Key Validation
```go
// Always use constant-time comparison
import "crypto/subtle"

if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
    return errors.New("invalid key")
}
```

### Input Validation
```go
// Validate all inputs
if cfg.Owner == "" {
    return fmt.Errorf("owner email required")
}

if len(cfg.Recipients) == 0 {
    return fmt.Errorf("at least one recipient required")
}
```

### Error Messages
```go
// Don't leak sensitive information
return fmt.Errorf("authentication failed")  // ✓ Good

return fmt.Errorf("invalid key: expected %s, got %s", expected, provided)  // ✗ Bad
```

## Deployment from Development

### Manual Deployment

```bash
# Build
make build

# Deploy specific handler
aws lambda update-function-code \
  --function-name deadmanshandle-http \
  --zip-file fileb://terraform/build/http.zip

# Test the deployment
aws lambda invoke \
  --function-name deadmanshandle-http \
  --payload '{"body":"test"}' \
  response.json
```

### Blue-Green Deployment

```bash
# Create new version
aws lambda publish-version \
  --function-name deadmanshandle-http

# Update alias
aws lambda update-alias \
  --function-name deadmanshandle-http \
  --name prod \
  --function-version <new-version>
```

## Troubleshooting Development

### Tests Fail on Timeout
```go
// Increase timeout for CI environments
if os.Getenv("CI") != "" {
    timeout = 10 * time.Second
} else {
    timeout = 5 * time.Second
}
```

### Import Issues
```bash
# Update dependencies
go get -u

# Clean cache
go clean -modcache

# Verify modules
go mod verify

# Tidy modules
go mod tidy
```

### Lambda Execution Fails Locally
- Use LocalStack for local AWS emulation
- Or use AWS SAM: `sam local start-api`
- Check CloudWatch logs on real AWS deployment

## Code Review Checklist

Before submitting a pull request:

- [ ] All tests pass: `go test -v ./...`
- [ ] Code formatted: `go fmt ./...`
- [ ] Linting: `golangci-lint run ./...`
- [ ] No unused imports: `go mod tidy`
- [ ] Documentation updated: README.md, code comments
- [ ] Tests added for new functionality
- [ ] Security review: no hardcoded secrets
- [ ] Performance considered: memory, cold starts
- [ ] Error handling: all errors handled
- [ ] Terraform changes valid: `terraform fmt`, `terraform validate`

## Useful Commands

```bash
# Format code
go fmt ./...

# Lint code (install golangci-lint first)
golangci-lint run ./...

# Generate mocks (if using mockgen)
go generate ./...

# Build docs
go doc -all ./internal/domain

# List test coverage by file
go tool cover -html=coverage.out

# Profile memory usage
go test -memprofile=mem.prof ./...
go tool pprof mem.prof
```

## Resources

- [Go Documentation](https://golang.org/doc/)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/)
- [Hexagonal Architecture](https://en.wikipedia.org/wiki/Hexagonal_architecture)
- [AWS Lambda Best Practices](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
