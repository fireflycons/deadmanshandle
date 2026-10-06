# Development Guide

This guide covers how to develop and extend the Deadman's Handle application.

## Local Development Setup

### 1. Install Dependencies

```bash
# Go 1.26+ (see go.mod)
brew install go

# Git (for version control)
brew install git

# Optional: Make (for build automation)
brew install make

# For infrastructure changes: Terraform 1.5+
brew install terraform
```

On Windows, use Git Bash for `make` and the shell commands in this guide.

### 2. Clone and Setup

```bash
git clone <repository>
cd deadmanshandle

# Download Go dependencies
go mod download

# Verify build
go build -v ./...
```

### 3. Project Layout

```
internal/
  ├── config/       # Config JSON type and its validation
  ├── domain/       # Business logic and the clock - main development focus
  ├── ports/        # Interfaces to AWS services (must not import domain)
  ├── adapters/     # AWS implementations of the ports
  ├── handlers/     # Lambda entry logic: wires ports and domain together
  └── mocks/        # Test doubles for the ports
cmd/
  ├── http/         # HTTP Lambda main (check-in)
  └── scheduled/    # Scheduled Lambda main (daily run)
terraform/          # Infrastructure
```

### 4. Conventions

- Text files use LF line endings (enforced by `.gitattributes`).
- Times are UTC. Emails show deadlines as `Monday 2 January 2006 at 15:04 MST`.
- Use the standard library plus the AWS SDK only.

## Development Workflow

### Adding a New Feature

1. **Update Config Model** (if needed)
   ```bash
   # Edit internal/config/config.go
   # Add new fields to Config, and rules to Config.Validate
   # Mirror the rules in the preconditions in terraform/parameter_store.tf
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
   # Add MockNewService and its compile-time check:
   #   _ ports.NewService = (*MockNewService)(nil)
   ```

6. **Update Handler**
   ```bash
   # Edit internal/handlers/http_handler.go or scheduled_event_handler.go
   # Use new service capability
   # Add tests in http_handler_test.go or scheduled_event_handler_test.go
   ```

7. **Test**
   ```bash
   go build ./... && go vet ./... && go test ./internal/...
   ```

### Code Style Guide

#### Naming Conventions
```go
// Interfaces: MixedCaps, often ending in -er
type EmailSender interface {}
type ConfigStore interface {}

// Exported types and functions: MixedCaps
type Config struct {}
type HTTPHandler struct {}
func (s *DeadmansHandleService) CheckIn(cfg *config.Config) (*config.Config, error) {}

// Unexported: mixedCaps
func warningBody(remaining time.Duration, timeout time.Time) string {}

// Constants: MixedCaps too, not UPPER_SNAKE_CASE
const defaultResetDays = 30
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
func (s *DeadmansHandleService) CheckIn(cfg *config.Config) (*config.Config, error) {
    // ...
}
```

#### Error Handling
```go
// Return errors with context, don't ignore them
if err := h.emailSender.SendEmail(ctx, to, subject, body, attachments); err != nil {
    return fmt.Errorf("sending email to %s: %w", to, err)
}

// Where one failure should not stop the rest (as when sending to each
// recipient), collect errors and return errors.Join(errs...)
```

#### Logging
Both Lambdas log JSON lines with `log/slog` (set up in `cmd/*/main.go`), which
Lambda sends to CloudWatch Logs. Log events and errors with key/value
attributes:

```go
slog.Info("Email sent", "to", email.To, "subject", email.Subject)
slog.Error("Check-in failed: saving configuration", "error", err)
```

Never log the config as a whole: it contains the API key. Never log the
document.

### Testing Guidelines

#### Unit Tests
- Located in `*_test.go` files alongside code
- Use table-driven tests for multiple scenarios
- Use the mocks in `internal/mocks/` for the ports; use the real domain service
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
func TestCheckInClearsDeliveryState(t *testing.T) {}
func TestProcessScheduledEventWarning(t *testing.T) {}
func TestScheduledHandlerRetriesOnlyFailedRecipients(t *testing.T) {}
```

#### Mock Usage
```go
// Create mocks
configStore := mocks.NewMockConfigStore()
emailSender := mocks.NewMockEmailSender()

// Store a valid config (ParseConfig rejects an invalid one)
cfgData, _ := cfg.ToJSON()
configStore.Data["test-param"] = cfgData

// Simulate failures
configStore.SetErr = errors.New("ssm down")
emailSender.FailFor["recipient@example.com"] = errors.New("rejected")

// Execute code
handler.Handle(ctx)

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
```

### Building

```bash
# Check everything compiles (the binaries only run inside Lambda)
go build ./...

# Lambda binaries: linux/arm64, named bootstrap
make build

# Equivalent to:
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/http/bootstrap ./cmd/http
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/scheduled/bootstrap ./cmd/scheduled
```

### Debugging

There is no local Lambda environment: `cmd/*/main.go` only runs inside Lambda.
Exercise the handlers through their tests, with mocks for AWS, and use the
CloudWatch logs of the deployed functions.

#### Using Go Debugger (Delve)

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a test
dlv test ./internal/domain -- -test.run TestCheckIn
```

#### CloudWatch Logs

```bash
# Follow the HTTP handler's logs
aws logs tail /aws/lambda/deadmanshandle-http --follow

# The scheduled handler's logs for the last day
aws logs tail /aws/lambda/deadmanshandle-scheduled --since 1d
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

4. **Inject into the Handler**

   The domain stays free of ports: it makes decisions, and the handlers carry
   them out through the ports.
   ```go
   // internal/handlers/scheduled_event_handler.go
   type ScheduledEventHandler struct {
       // ... existing fields
       newService ports.NewService
   }
   ```
   Create the adapter in `cmd/*/main.go`, and grant the Lambda's IAM role the
   permissions it needs in `terraform/iam.tf`.

### Adding a New Handler

1. **Create handler file**
   ```bash
   touch internal/handlers/new_handler.go
   ```

2. **Define handler struct**
   ```go
   type NewHandler struct {
       // ports as interfaces, the domain service directly
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

5. **Build and deploy it**: add it to the `build` target in the Makefile, and
   add the function, its archive and its log group in `terraform/lambda.tf`.

### Modifying Configuration

1. **Update Config struct**
   ```go
   type Config struct {
       // ... existing fields
       NewField string `json:"newField"`
   }
   ```

2. **Validate it** in `Config.Validate`, and with a matching precondition in
   `terraform/parameter_store.tf`
3. **Add tests** for new field
4. **Update documentation** in README.md and DEPLOYMENT.md. Deployed configs
   are updated with `aws ssm put-parameter` (see DEPLOYMENT.md), since
   Terraform only seeds the parameter.

## Performance

AWS clients are created once in `init()` in `cmd/*/main.go` and reused on warm
starts. Anything that must be current on each invocation (such as the clock)
is read per call instead. Both functions run with 256 MB; the work is a few
AWS calls a day, so there is little to tune.

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
All config rules live in `Config.Validate` (called by `ParseConfig`), so both
Lambdas reject an invalid config. Add new rules there rather than at the point
of use.

### Error Messages
```go
// Don't leak sensitive information
return fmt.Errorf("authentication failed")  // ✓ Good

return fmt.Errorf("invalid key: expected %s, got %s", expected, provided)  // ✗ Bad
```

The check-in API returns short, generic messages; the details go to the logs.

## Deploying Changes

Deploy with Terraform, which zips the binaries from `bin/` during `plan` and
`apply` and updates a function when its code changes:

```bash
make build
cd terraform
terraform apply
```

Don't upload code with `aws lambda update-function-code`: the zips in
`terraform/build/` are only refreshed by Terraform, so they may be stale.

## Troubleshooting Development

### Module Issues
```bash
# Clean cache
go clean -modcache

# Verify modules
go mod verify

# Tidy modules
go mod tidy
```

## Code Review Checklist

Before submitting a pull request:

- [ ] All tests pass: `go test ./...`
- [ ] Code formatted: `gofmt -l .` prints nothing
- [ ] Vet passes: `go vet ./...`
- [ ] Modules tidy: `go mod tidy` makes no changes
- [ ] Documentation updated: README.md, DEPLOYMENT.md, code comments
- [ ] Tests added for new functionality
- [ ] Security review: no hardcoded secrets, nothing sensitive logged
- [ ] Error handling: all errors handled
- [ ] Terraform changes valid: `terraform fmt -check`, `terraform validate`

## Useful Commands

```bash
# Format code
go fmt ./...

# Build docs
go doc -all ./internal/domain

# Coverage by line, in a browser
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Profile memory usage
go test -memprofile=mem.prof ./internal/domain
go tool pprof mem.prof
```

## Resources

- [Go Documentation](https://go.dev/doc/)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/)
- [Hexagonal Architecture](https://en.wikipedia.org/wiki/Hexagonal_architecture)
- [AWS Lambda Best Practices](https://docs.aws.amazon.com/lambda/latest/dg/best-practices.html)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
