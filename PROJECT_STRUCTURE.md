# Project Structure

```
deadmanshandle/
│
├── cmd/                                  # Lambda entry points
│   ├── http/
│   │   └── main.go                       # HTTP API Gateway handler
│   ├── scheduled/
│   │   └── main.go                       # EventBridge scheduled handler
│   ├── docwatch/
│   │   └── main.go                       # S3 upload (EventBridge) handler
│   ├── alarmpush/
│   │   └── main.go                       # Alarm topic (SNS) handler: pushes alarms
│   └── qrconfig/
│       └── main.go                       # CLI: QR code that sets up the Android app
│
├── internal/                             # Application packages
│   ├── adapters/                         # AWS service implementations
│   │   ├── ssm_config_store.go           # Parameter Store adapter
│   │   ├── caching_config_store.go       # Config cache for the HTTP Lambda
│   │   ├── caching_config_store_test.go  # Cache tests
│   │   ├── dynamodb_state_store.go       # DynamoDB state adapter
│   │   ├── dynamodb_state_store_test.go  # Expression attribute name tests
│   │   ├── s3_document_store.go          # S3 storage adapter
│   │   ├── ses_email_sender.go           # SES email adapter (builds the MIME message)
│   │   ├── ses_email_sender_test.go      # MIME message tests
│   │   ├── sns_push_sender.go            # SNS mobile push adapter (FCM)
│   │   ├── sns_push_sender_test.go       # Push message tests
│   │   └── api_key_validator.go          # API key validation
│   │
│   ├── config/                           # Configuration model
│   │   ├── config.go                     # Config structure, parsing and validation
│   │   └── config_test.go                # Parsing and validation tests
│   │
│   ├── domain/                           # Core business logic
│   │   ├── service.go                    # Main service with business rules
│   │   └── service_test.go               # Domain logic tests
│   │
│   ├── handlers/                         # Lambda handlers
│   │   ├── alarm_push.go                 # Alarm push handler
│   │   ├── alarm_push_test.go            # Alarm push tests
│   │   ├── document_watch.go             # Document change check and upload handler
│   │   ├── document_watch_test.go        # Document change tests
│   │   ├── http_handler.go               # HTTP API handler
│   │   ├── http_handler_test.go          # HTTP handler tests
│   │   ├── push.go                       # Pushes owner notices, best-effort
│   │   ├── push_test.go                  # Push and push token tests
│   │   ├── scheduled_event_handler.go    # EventBridge handler
│   │   └── scheduled_event_handler_test.go  # Scheduled handler tests
│   │
│   ├── mocks/                            # Mock implementations for testing
│   │   └── mocks.go                      # Mocks of the ports
│   │
│   ├── ports/                            # Port interfaces (hexagonal architecture)
│   │   └── ports.go                      # Service interfaces
│   │
│   └── qrconfig/                         # The setup QR code's payload
│       ├── qrconfig.go                   # Endpoint from the state, key from the config, JSON
│       └── qrconfig_test.go              # Payload tests
│
├── android/                              # Android check-in app (Gradle project)
│   ├── app/
│   │   ├── build.gradle.kts              # App build: version from VERSION, signing, R8
│   │   ├── proguard-rules.pro            # R8 keep rules (ML Kit)
│   │   └── src/
│   │       ├── main/                     # Manifest, resources and Kotlin source
│   │       │   └── java/com/fireflycons/deadmanshandle/
│   │       │       ├── DeadmansHandleApp.kt  # Starts Firebase from the saved push settings
│   │       │       ├── MainActivity.kt   # Wires the screens to storage, prompt and API
│   │       │       ├── auth/             # BiometricPrompt: fingerprint or screen lock
│   │       │       ├── checkin/          # Check-in request and its result
│   │       │       ├── config/           # Config, Keystore storage, QR payload
│   │       │       ├── push/             # Firebase, push messages and notifications
│   │       │       └── ui/               # Compose screens
│   │       └── test/                     # JVM unit tests
│   ├── gradle/                           # Version catalogue, wrapper, daemon JDK
│   ├── build.gradle.kts                  # Root build
│   ├── settings.gradle.kts               # Repositories and modules
│   └── gradlew, gradlew.bat              # Gradle wrapper
│
├── terraform/                            # Infrastructure as Code
│   ├── provider.tf                       # AWS provider and Terraform version
│   ├── variables.tf                      # Input variables
│   ├── data.tf                           # Data sources
│   ├── iam.tf                            # IAM role and policies
│   ├── lambda.tf                         # Lambda functions and log groups
│   ├── api_gateway.tf                    # HTTP API Gateway
│   ├── s3.tf                             # S3 bucket configuration
│   ├── parameter_store.tf                # SSM parameter and seed-config checks
│   ├── dynamodb.tf                       # State table and its seeded item
│   ├── eventbridge.tf                    # EventBridge rules and DLQ
│   ├── ses.tf                            # SES domain identity and DKIM records
│   ├── alarms.tf                         # SNS topic and CloudWatch alarms
│   ├── push.tf                           # Push to the app (optional): SNS, alarmpush, alarms
│   ├── outputs.tf                        # Output values
│   ├── terraform.tfvars.example          # Example variables file
│   ├── .tflint.hcl                       # TFLint rules (AWS ruleset)
│   └── build/                            # Lambda zips (generated)
│
├── bin/                                  # Compiled binaries (generated)
│   ├── http/bootstrap                    # HTTP handler binary
│   ├── scheduled/bootstrap               # Scheduled handler binary
│   ├── docwatch/bootstrap                # Document watch handler binary
│   └── alarmpush/bootstrap               # Alarm push handler binary
│
├── docs/
│   └── architecture.png                  # AWS architecture diagram
│
├── tools/
│   └── architecture_diagram.py           # Draws docs/architecture.png (Python, diagrams)
│
├── go.mod                                # Go module definition
├── go.sum                                # Go dependencies checksums
├── Makefile                              # Build automation
├── VERSION                               # Release version (bumped in each PR)
├── .github/
│   ├── workflows/ci.yml                  # Lint and test on push and PR
│   ├── workflows/release.yml             # Build and release on merge to master
│   └── scripts/check-version.sh          # Checks VERSION is an unused, higher version
├── .gitattributes                        # LF line endings
├── .gitignore                            # Git ignore rules
│
├── LICENSE                               # MIT licence
├── README.md                             # Project documentation
├── QUICKSTART.md                         # Quick start guide
├── DEPLOYMENT.md                         # Deployment guide
├── DEVELOPMENT.md                        # Development guide
├── PROJECT_STRUCTURE.md                  # This file
├── spec.md                               # Original requirements
├── config.example.json                   # Example configuration
└── config.json                           # Configuration (git-ignored)
```

## Directory Descriptions

### `cmd/` - Command/Entry Points
- **http**: HTTP API Gateway Lambda handler
  - Handles check-in requests
  - Validates API keys
  - Sets the timeout in the DynamoDB state

- **scheduled**: EventBridge scheduled event handler
  - Processes daily checks
  - Sends warning emails
  - Distributes documents on timeout

- **docwatch**: S3 upload (EventBridge) handler
  - Reports changes to the document's content to the owner

- **alarmpush**: Alarm topic (SNS) handler, deployed only with push
  - Pushes each CloudWatch alarm to the app

- **qrconfig**: Command-line tool, not a Lambda
  - Writes a PNG QR code that sets up the Android app (`make qrcode`)
  - Reads the endpoint from the Terraform state and the API key from the
    config file, or both from Parameter Store with `-aws`
  - Adds the push settings from a Firebase `google-services.json` with
    `-firebase`

Each `main.go` sets up JSON logging, creates the AWS clients and adapters once
at init, and starts the Lambda runtime.

### `internal/adapters/` - AWS Service Adapters
Implements the port interfaces using AWS SDK:
- **SSMConfigStore**: Reads the configuration from a SecureString parameter
- **CachingConfigStore**: Wraps a ConfigStore and keeps the configuration for
  a set time; the HTTP Lambda uses it for 5 minutes, so a flood of requests
  does not cost an SSM and KMS call each
- **DynamoDBStateStore**: Reads and atomically updates the state item; the
  updates that record deliveries or an ETag are conditional, so concurrent
  Lambdas cannot lose each other's writes
- **S3DocumentStore**: Retrieves documents from S3
- **SESEmailSender**: Sends emails via SES, with the document as an attachment
- **SNSPushSender**: Pushes to the app through an SNS platform application for
  FCM, creating the token's endpoint on demand
- **SimpleAPIKeyValidator**: Validates API keys with constant-time comparison

### `internal/config/` - Configuration Model
- Defines the Config struct matching Parameter Store JSON, and the State
  struct for the timeout, delivery state and recorded ETag in DynamoDB
- Parses, validates (`Config.Validate`) and serializes the config

### `internal/domain/` - Core Business Logic
- **DeadmansHandleService**: Main service encapsulating all business logic
  - `CheckIn()`: Process owner check-in, returning now + `resetDays`
  - `ProcessScheduledEvent()`: Determine what emails to send
  - `DocumentChanged()`: Decide whether a document's ETag is a change to report

- Tests using time injection for deterministic testing

### `internal/handlers/` - Lambda Handlers
- **HTTPHandler**: Processes API Gateway (payload 2.0) requests
  - Validates API key header
  - Calls domain service
  - Sets the new timeout in the state
  - Returns JSON response

- **ScheduledEventHandler**: Processes EventBridge events
  - Fetches configuration
  - Calls domain service
  - Retrieves document if needed
  - Sends emails, recording each delivery as it succeeds; stops if the owner
    checks in during the run

- **DocumentWatcher** and **DocumentWatchHandler**: Compare the document's
  ETag with the recorded one, record it (and the time of a change) with a
  conditional swap, and email the owner of a change, retrying a notice that
  failed

- Owner notices are also pushed to the app's recorded token, whether or not
  the email was sent; a failed push is logged, never returned

- **AlarmPushHandler**: Pushes a CloudWatch alarm from the alarm topic

### `internal/mocks/` - Testing Mocks
Mock implementations of all port interfaces for unit testing:
- **MockConfigStore**: In-memory configuration storage, with injectable errors
- **MockStateStore**: In-memory state with the same conditions as DynamoDB,
  and a hook to simulate a concurrent writer
- **MockDocumentStore**: In-memory document storage
- **MockEmailSender**: Captures sent emails, and can fail for chosen recipients
- **MockPushSender**: Captures pushes, and can fail
- **MockAPIKeyValidator**: Plain comparison of the provided and expected keys

### `internal/ports/` - Port Interfaces (Hexagonal Architecture)
Defines contracts for external dependencies:
- **ConfigStore**: Get/set configuration
- **DocumentStore**: Retrieve documents
- **EmailSender**: Send an email, with optional attachments
- **PushSender**: Push a notice to the owner's phone
- **APIKeyValidator**: Validate API keys

### `internal/qrconfig/` - Setup QR Code
- Builds the QR code's JSON payload, `{"v":1,"endpoint":"…","apiKey":"…"}`,
  with an optional `push` object holding the Firebase client settings; the
  app rejects a version it doesn't know

### `android/` - Android Check-in App
Kotlin and Jetpack Compose, for Android 11 and later:
- **config/**: The endpoint and API key, the key encrypted with an Android
  Keystore key that needs the fingerprint or screen lock for every use; and
  the QR payload's parser
- **auth/**: Authorises the Keystore key with BiometricPrompt
- **checkin/**: `POST <endpoint>/checkin`, with the push token, and its result
- **push/**: Firebase, started at run time from the QR code's push settings;
  the messaging service, and the notifications
- **ui/**: The config entry screen (by hand or by QR scan) and the check-in
  screen

See [DEVELOPMENT.md](DEVELOPMENT.md#android-app) for building it.

### `terraform/` - Infrastructure as Code
Terraform files defining AWS resources:
- **provider.tf**: AWS provider configuration
- **iam.tf**: Lambda execution role and policies
- **lambda.tf**: Lambda function definitions, archiving and log groups
- **api_gateway.tf**: HTTP API and routes
- **s3.tf**: S3 bucket with security settings
- **parameter_store.tf**: Configuration parameter, and checks on the seed file
- **eventbridge.tf**: Scheduled rule, target and dead-letter queue
- **ses.tf**: SES domain identity, verified with DKIM
- **alarms.tf**: Alarms that email the owner if the daily run fails or does not run
- **push.tf**: With `firebase_service_account_file` set, the SNS platform
  application for FCM, the alarmpush Lambda and the push alarms
- **variables.tf**: Input variables
- **outputs.tf**: Output values
- **data.tf**: Data sources (AWS account ID)

## Architecture Pattern: Hexagonal Architecture

The application follows hexagonal architecture (ports & adapters). The domain
makes the decisions; the handlers carry them out through the ports:

```
┌─────────────────────────────────────┐
│       Lambda Entry Points           │
│  (cmd/http, cmd/scheduled,          │
│   cmd/docwatch)                     │
└──────────────┬──────────────────────┘
               │ wire up
┌──────────────▼──────────────────────┐
│        Handlers Layer               │
│   (internal/handlers)               │
└───────┬─────────────────────┬───────┘
        │ calls               │ calls
┌───────▼────────────┐ ┌──────▼──────────────┐
│ Core Domain Logic  │ │  Port Interfaces    │
│ (internal/domain)  │ │  (internal/ports)   │
└────────────────────┘ └──────▲──────────────┘
                              │ implement
                     ┌────────┴─────────┐
                     │                  │
                ┌────┴─────┐       ┌────┴────┐
                │ Adapters │       │  Mocks  │
                │  (AWS)   │       │ (Tests) │
                └──────────┘       └─────────┘
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
- HTTP and scheduled handler tests with mocks
- Configuration parsing and validation tests
- SES adapter tests that parse the built MIME message

### Test Execution
```bash
go test -v ./...           # Run all tests
go test -cover ./...       # With coverage
go test -run TestName ./internal/domain  # Specific test
```

### Mock Usage
Handler tests use the mocks from `internal/mocks/` for the ports, and the real
domain service with a fixed clock:
```go
configStore := mocks.NewMockConfigStore()
emailSender := mocks.NewMockEmailSender()
validator := mocks.NewMockAPIKeyValidator()
service := domain.NewDeadmansHandleServiceWithTime(now)
```

## Build Process

### Local Development
```bash
make build          # Builds the Lambda handlers
make test           # Runs tests
make clean          # Removes artifacts
make qrcode         # Writes the setup QR code, checkin-qr.png
make android-test   # Android unit tests and lint
make android        # Android release APK
```

### Production Build
- Compiles for Linux ARM64 (Lambda architecture), CGO off, `lambda.norpc`
- Names each binary `bootstrap`, as the `provided.al2023` runtime expects

### Terraform Deployment
- Zips each binary (with the executable bit set) into `terraform/build/`
- Uploads the zips directly as the Lambda functions' code
- Manages all AWS infrastructure

## Environment Variables

### Runtime (Lambda)
- `CONFIG_PARAMETER_NAME`: Parameter Store path (all functions)
- `STATE_TABLE_NAME`: DynamoDB state table (all functions)
- `SENDER_EMAIL`: Sender address, in the SES-verified domain
- `DOCUMENT_BUCKET`: S3 bucket name (scheduled and docwatch)
- `DOCUMENT_KEY`: S3 object key (scheduled and docwatch)
- `PUSH_PLATFORM_APPLICATION_ARN`: SNS platform application for push, empty
  when push is off (scheduled, docwatch and alarmpush)

### Build/Deployment
- `GOOS`: Operating system (linux for Lambda)
- `GOARCH`: Architecture (arm64 for Lambda)
- AWS credentials via standard AWS CLI

## Dependencies

### Go Dependencies (Standard Library Focus)
- `github.com/aws/aws-lambda-go`: Lambda runtime
- `github.com/aws/aws-sdk-go-v2`: AWS SDK
- `github.com/skip2/go-qrcode`: QR codes, in `cmd/qrconfig` only
- Standard library: encoding/json, time, context, log/slog, mime, etc.

### External Tools
- Go 1.26+ (see `go.mod`)
- Terraform 1.5+
- AWS CLI
- Make (optional)
- Android Studio, for its JDK and the Android SDK (the Android app only)

## Security Considerations

1. **API Key Validation**: Uses `subtle.ConstantTimeCompare` to prevent timing attacks
2. **S3 Security**: All public access blocked, optional versioning (`document_versioning`, off by default) with old versions expiring after 30 days
3. **Parameter Store**: The configuration, which holds the API key, is a SecureString
4. **IAM Least Privilege**: Lambda role only has necessary permissions
5. **SES Verification**: The sender domain is verified with DKIM by Terraform
6. **Input Validation**: `Config.Validate` checks every configuration field
7. **Logging**: The API key and the document are never logged

## Deployment Flow

1. Developer runs `make build` to compile binaries
2. Developer runs Terraform `init`, `plan`, `apply`
3. Terraform archives binaries and creates Lambda functions
4. API Gateway routes HTTP requests to HTTP handler
5. EventBridge triggers scheduled handler daily
6. Handlers use adapters to call AWS services
7. Core domain logic remains independent and testable
