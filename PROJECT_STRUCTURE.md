# Project Structure

## Architecture

![AWS architecture: check-in through API Gateway, a daily EventBridge run, S3 upload events, config in SSM, state in DynamoDB, email through SES, alarms to the owner, and optional push notifications to the app through SNS and Firebase Cloud Messaging](docs/architecture.png)

The diagram is drawn by `tools/architecture_diagram.py`; see
[DEVELOPMENT.md](DEVELOPMENT.md#architecture-diagram).

### AWS Services Used

- **AWS Lambda**: Compute (3 functions, 4 with push)
- **API Gateway**: HTTP endpoint for check-ins
- **EventBridge**: Scheduled daily checks, and S3 upload events
- **Parameter Store**: Configuration storage
- **DynamoDB**: Timeout and delivery state
- **S3**: Document storage
- **SES**: Email delivery
- **CloudWatch**: Logs (14-day retention) and alarms
- **SNS**: Alarm emails to the owner, and push notifications to the app
  through Firebase Cloud Messaging

### Configuration and State

The configuration is stored in Parameter Store as JSON (see
[DEPLOYMENT.md](DEPLOYMENT.md#2-create-configuration-file)). The Lambdas only
read it.

What changes at run time is kept in one item of a DynamoDB table
(`deadmanshandle-state`), whose updates are atomic, so Lambdas running at the
same time cannot undo each other's writes:

- `timeout`: seeded from the config file, then set by each check-in
- `checkIns`: counts check-ins
- `sentTo`: recipients who have already been sent the document
- `ownerNotified`: whether the owner has been told that it was sent
- `documentETag`: the document's ETag when last seen
- `documentChangedAt`, `documentChangePending`: when its content last changed,
  and whether the owner has yet to be told
- `pushToken`: the app's push token, sent with its last check-in

A check-in clears `sentTo` and `ownerNotified`.

## Layout

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
  - Writes a PNG QR code that sets up the Android app (`make qrcode`); each
    release includes it for Windows, macOS and Linux
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

## Building, Testing and Deploying

See [DEVELOPMENT.md](DEVELOPMENT.md): [Building](DEVELOPMENT.md#building),
[Testing Guidelines](DEVELOPMENT.md#testing-guidelines) and
[Deploying Changes](DEVELOPMENT.md#deploying-changes).

## Environment Variables

### Runtime (Lambda)
- `CONFIG_PARAMETER_NAME`: Parameter Store path (all functions)
- `STATE_TABLE_NAME`: DynamoDB state table (all functions)
- `SENDER_EMAIL`: Sender address, in the SES-verified domain
- `DOCUMENT_BUCKET`: S3 bucket name (scheduled and docwatch)
- `DOCUMENT_KEY`: S3 object key (scheduled and docwatch)
- `PUSH_PLATFORM_APPLICATION_ARN`: SNS platform application for push, empty
  when push is off (scheduled, docwatch and alarmpush)

## Dependencies

- `github.com/aws/aws-lambda-go`: Lambda runtime
- `github.com/aws/aws-sdk-go-v2`: AWS SDK
- `github.com/skip2/go-qrcode`: QR codes, in `cmd/qrconfig` only
- Standard library: encoding/json, time, context, log/slog, mime, etc.

The tools needed to build are in
[DEVELOPMENT.md](DEVELOPMENT.md#1-install-dependencies).

## Security Considerations

See [README.md](README.md#security-considerations).
