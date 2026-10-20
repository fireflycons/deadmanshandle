# Development Guide

This guide covers working with the source in a clone of the repository:
building, testing, developing and extending the Deadman's Handle application
and its Android app, and how CI makes the releases. To deploy a release, which
needs no building, see [DEPLOYMENT.md](DEPLOYMENT.md).

## Local Development Setup

### 1. Install Dependencies

```bash
# Go 1.26+ (see go.mod)
brew install go

# Git (for version control)
brew install git

# Optional: Make (for build automation)
brew install make

# To deploy, and for infrastructure changes: Terraform 1.5+ and the AWS CLI
brew install terraform awscli

# For the Android app: Android Studio, for its JDK and the Android SDK
brew install --cask android-studio
```

The Makefile needs GNU make 4.x and works from any shell, including
PowerShell or cmd on Windows. On Windows, use Git Bash for the other shell
commands in this guide.

The Android app needs nothing on the PATH. Its build uses:

- **Gradle**, through the wrapper in `android/` (`gradlew`), which downloads
  the version it needs.
- **JDK 25** for the Gradle daemon (`android/gradle/gradle-daemon-jvm.properties`).
  Android Studio's own JDK (its `jbr` directory) is suitable.
- **The Android SDK**, with the platform for `compileSdk` (in
  `android/app/build.gradle.kts`). Install it with Android Studio's SDK
  Manager; opening `android/` in Android Studio offers to install what is
  missing. Gradle downloads the app's libraries (Compose, Biometric, the
  Play services code scanner, Firebase Messaging) itself.
- **An emulator or a device** for anything beyond the JVM tests; see
  [Android App](#android-app).

See [Building the Android App](#building-the-android-app) for pointing the
build at the JDK and the SDK.

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

See [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) for the layout of the
repository and the hexagonal architecture. Most development is in
`internal/domain`, the business rules.

### 4. Conventions

- Text files use LF line endings (enforced by `.gitattributes`).
- Times are UTC. Emails show deadlines as `Monday 2 January 2006 at 15:04 MST`.
- Use the standard library plus the AWS SDK only. The one exception is
  `github.com/skip2/go-qrcode`, used only by `cmd/dmh`.

## Building

`make help` lists the targets. `make clean` removes everything they build.

### Building the Lambdas

```bash
# Check everything compiles (the binaries only run inside Lambda)
go build ./...

# Lambda binaries: linux/arm64, named bootstrap
make build

# Equivalent to:
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags "-s -w" -o bin/http/bootstrap ./cmd/http
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags "-s -w" -o bin/scheduled/bootstrap ./cmd/scheduled
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags "-s -w" -o bin/docwatch/bootstrap ./cmd/docwatch
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags "-s -w" -o bin/alarmpush/bootstrap ./cmd/alarmpush
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -ldflags "-s -w" -o bin/apiguard/bootstrap ./cmd/apiguard
```

Terraform deploys these binaries from `bin/`; see
[Deploying Changes](#deploying-changes). Go records the commit in each binary,
so every rebuild shows as a code change in the plan.

### Building the Android App

```bash
make android-test   # JVM unit tests and lint
make android        # Release APK: android/app/build/outputs/apk/release/app-release.apk
```

On Windows the Makefile points `JAVA_HOME` at Android Studio's JDK
(`C:\Program Files\Android\Android Studio\jbr`) and `ANDROID_HOME` at
`%LOCALAPPDATA%\Android\Sdk`; override them with `ANDROID_JAVA_HOME=...` and
`ANDROID_HOME=...`. Elsewhere, set `JAVA_HOME` and `ANDROID_HOME` yourself.

Don't pipe `make android` or `make android-test` into another command: the
Gradle daemon keeps the pipe open and the command never ends. Redirect to a
file instead.

The release build is signed with the release key when `ANDROID_KEYSTORE_FILE`
(a PKCS12 keystore), `ANDROID_KEYSTORE_PASSWORD` and `ANDROID_KEY_ALIAS` are
set. The key's password is the keystore's. Without them it is debug-signed,
which is fine for testing but can't update an installed release.

### The dmh CLI

`make dmh` builds the CLI for this machine as `bin/dmh/dmh` (`dmh.exe` on
Windows). Releases build it for each desktop platform. `go run ./cmd/dmh`
also works.

### The Recipients' Tools

```bash
make open-document  # bin/recipients/open-document(.exe), for this machine
make decrypt-test   # tools/decrypt/decrypt.html against the Go test vector
```

`open-document` (`cmd/open-document`, logic in `internal/opener`) and
`tools/decrypt/decrypt.html` open a document encrypted by `dmh document put`.
The format is in `internal/doccrypt`. Its committed test vector,
`internal/doccrypt/testdata/vector.{json,dmh}`, is checked by the Go tests and,
through `make decrypt-test`, against the page's own decryption code (between
its `BEGIN decrypt` and `END decrypt` markers) with Node 20 or later's
WebCrypto. A change to the format must change both, and `go test
./internal/doccrypt -update` rewrites `vector.dmh`. The page itself needs a
browser to test: open it from disk and open a `.dmh` file.

### The Setup QR Code

`make qrcode` builds and runs `dmh qr` for this machine, writing the QR
code that sets up the app; see
[Make the Setup QR Code](DEPLOYMENT.md#2-make-the-setup-qr-code).

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
The Lambdas log JSON lines with `log/slog` (set up in `cmd/*/main.go`), which
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
    timeout := service.CheckIn(cfg)

    // Verify
    if !timeout.Equal(now.AddDate(0, 0, 30)) {
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
stateStore := mocks.NewMockStateStore(config.State{Timeout: timeout})
emailSender := mocks.NewMockEmailSender()

// Store a valid config (ParseConfig rejects an invalid one)
configStore.Data["test-param"] = configJSON(t, cfg)

// Simulate failures
configStore.GetErr = errors.New("ssm down")
stateStore.CheckInErr = errors.New("dynamodb down")
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
# All Go tests
make test
go test -v ./...

# With coverage
make local
go test -cover ./...

# go vet and golangci-lint, as CI runs them
make lint

# The Terraform configuration, without the backend
make validate

# Specific package
go test -v ./internal/domain

# Specific test
go test -run TestCheckIn ./internal/domain
```

The Android app's tests are run with `make android-test`; see
[Building the Android App](#building-the-android-app).

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
   Create the adapter in `cmd/*/main.go`, and add the permissions it needs
   to that Lambda's entry in `lambda_policies` in `terraform/iam.tf`. Each
   Lambda has its own role.

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
   Give it a role by adding it to `lambda_functions` and `lambda_policies` in
   `terraform/iam.tf`, and use `aws_iam_role.lambda["<name>"].arn`.

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
   are updated with `dmh config set` (see DEPLOYMENT.md), since Terraform only
   seeds the parameter: add a flag for the field to `cmd/dmh/config.go` and
   `cli.ConfigChange`, and show it in `cli.Report`.

## Performance

AWS clients are created once in `init()` in `cmd/*/main.go` and reused on warm
starts. Anything that must be current on each invocation (such as the clock)
is read per call instead. The HTTP, scheduled and docwatch functions run with
256 MB, and alarmpush and apiguard with 128 MB; the work is a few AWS calls a
day, so there is little to tune.

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
All config rules live in `Config.Validate` (called by `ParseConfig`), so the
Lambdas reject an invalid config. Add new rules there rather than at the point
of use.

### Error Messages
```go
// Don't leak sensitive information
return fmt.Errorf("authentication failed")  // ✓ Good

return fmt.Errorf("invalid key: expected %s, got %s", expected, provided)  // ✗ Bad
```

The check-in API returns short, generic messages; the details go to the logs.

## Android App

The check-in app is in `android/`: Kotlin and Jetpack Compose, `minSdk` 30
(Android 11). It opens in Android Studio; build it with make (see
[Building the Android App](#building-the-android-app)).

Its source layout is in
[PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md#android---android-check-in-app).

The API key is encrypted with an AES-GCM Android Keystore key that needs the
fingerprint or screen lock for every use. The endpoint is stored in clear, so
it can be shown without a prompt. Backups and device transfers are off.

Things to know:

- The JVM tests cover the config, the QR payload, the check-in result and
  body, and the push message; the Keystore, the prompt, the scanner and
  push need a device or an emulator. Push also needs a Google Play image and
  a deployment with a Firebase project.
- Firebase is started at run time from the setup QR code's push settings
  (`DeadmansHandleApp`), so each deployment can use its own Firebase project
  with the same APK. Don't add the Google services Gradle plugin or a
  `google-services.json`.
- The release build is minified with R8. `android/app/proguard-rules.pro`
  keeps ML Kit, whose internals break when R8 processes them.
- Escape apostrophes in `strings.xml` as `\'`; an unescaped one fails the build.
- To test QR scanning on an emulator, use a Google Play system image, signed
  in to a Google account: the scanner is a Play services module, and a Google
  APIs image can't download it. Load the code with Extended controls >
  Camera > Virtual scene images, as the Table image, and look down at the
  table.
- Enrol a fingerprint on an emulator in its settings, touching the sensor with
  `adb emu finger touch 1`.
- Test the release APK on the emulator (`make android`, then `adb install -r`
  with the APK): R8 problems only show in the minified build.

## Deploying Changes

Deploying from a clone is as in
[DEPLOYMENT.md](DEPLOYMENT.md#deploy-infrastructure), with your own build in
place of a release's binaries. Deploy with Terraform, which zips the binaries
from `bin/` during `plan` and `apply` and updates a function when its code
changes:

```bash
make build
cd terraform
terraform apply
```

Don't upload code with `aws lambda update-function-code`: the zips in
`terraform/build/` are only refreshed by Terraform, so they may be stale.

## CI and Releases

GitHub Actions runs these workflows (in `.github/workflows/`):

- **CI** (`ci.yml`), on a push to any branch except `master` and on a pull
  request to `master`: golangci-lint, `go test ./...` and `make decrypt-test`;
  `terraform fmt
  -check`, `terraform validate` and TFLint (with the AWS ruleset, configured in
  `terraform/.tflint.hcl`); the Android unit tests and lint, and a
  (debug-signed) release build. On a pull request it also checks the version
  (below).
  The `CI gate` job passes only if all of these do; a ruleset on `master`
  requires it, so a pull request can't be merged until it passes against the
  latest `master`.
- **Release** (`release.yml`), on a push to `master` (a merged pull request):
  runs the CI checks, builds the Lambdas and the Android app, and creates the
  GitHub release `v<VERSION>`. Its assets are `SHA256SUMS` and
  `deadmanshandle-v<VERSION>.zip`, which unzips to
  `deadmanshandle-v<VERSION>/` with `terraform/`, `config.example.json` and
  `LICENSE` (from `git archive`, so tracked files only, without
  `.tflint.hcl`), each Lambda's executable `bootstrap` (linux/arm64) in
  `bin/<function>/`, `app/deadmanshandle-v<VERSION>.apk`, signed with the
  release key, `dmh/dmh-<os>-<arch>`, the CLI for
  Windows, macOS and Linux on amd64 and arm64, and `recipients/`, for the
  owner to give the recipients: `open-document-<os>-<arch>` for the same
  platforms, `decrypt.html` and `README.txt`. The notes link to
  [Deploying from a Release](DEPLOYMENT.md#deploying-from-a-release) and list
  the commits since the previous release.
  A pull request with `[no release]` (any case) in its title is merged
  without one: the workflow still runs the checks, but skips the release.
  Its changes go out with the next release, whose notes list them.

The release key comes from three repository secrets: `ANDROID_KEYSTORE_BASE64`
(the PKCS12 keystore, base64-encoded), `ANDROID_KEYSTORE_PASSWORD` and
`ANDROID_KEY_ALIAS`. Without them the release fails, rather than publish an
APK that can't update the installed one. Every release must be signed with the
same key, so keep a copy of the keystore and its password outside GitHub: if
the key is lost, users must uninstall the app, which erases its config, to
install a new one.

The version is in `VERSION` at the root, as `MAJOR.MINOR.PATCH`. **Bump it in
every pull request to `master`**: the pull request fails if `v<VERSION>` is
already a tag, or isn't greater than the latest release and than `VERSION` on
`master`. The release fails too if the tag exists. A `[no release]` pull
request skips the check, so leave `VERSION` as it is. To run the same check
locally:

```bash
bash .github/scripts/check-version.sh
```

The Android app's `versionName` is `VERSION`, and its `versionCode` is
`MAJOR * 10000 + MINOR * 100 + PATCH`, so each release can update the last.

## Architecture Diagram

`docs/architecture.svg` is drawn by `tools/architecture_diagram.py`, using
Python's `diagrams` package and Graphviz for the layout, and the official SVG
icons from AWS's [Architecture Icons](https://aws.amazon.com/architecture/icons/)
package, which the script embeds so the SVG needs no other files.
`docs/architecture.html` shows it with a legend; it is edited by hand.

When the infrastructure changes, edit the script and rerun it; its docstring
says how, and which icon package release it was drawn with. Graphviz lays the
same diagram out differently from run to run, so rerun it until the layout
reads well, and check the page in a browser before committing.

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
- [ ] Android changes pass `make android-test` and `make android`
- [ ] `VERSION` bumped, or `[no release]` in the title

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
