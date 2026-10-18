# CLAUDE.md

## What this is

A "dead man's handle": the owner checks in periodically, from an Android app
or over the HTTP API. If they stop, a daily scheduled run emails a document
from S3 to a list of recipients. Requirements: [spec.md](spec.md) (service)
and [client-app.md](client-app.md) (app). User-facing docs: README,
DEPLOYMENT (including app setup), QUICKSTART, DEVELOPMENT, PROJECT_STRUCTURE.

## Layout

- Go module `github.com/fireflycons/deadmanshandle`. Standard library plus the
  AWS SDK only; the one exception is `github.com/skip2/go-qrcode`, used only
  by `cmd/dmh`.
- Four Lambdas (`provided.al2023`, arm64) from `cmd/http`, `cmd/scheduled`,
  `cmd/docwatch` and `cmd/alarmpush` (deployed only with push).
  `cmd/dmh` is the CLI. `dmh qr` writes the app's setup QR code
  (`make qrcode`; `QRFLAGS=-aws` reads Parameter Store, `-firebase` adds the
  push settings).
- Hexagonal `internal/`:
  - `domain`: business rules and the clock (injectable).
  - `ports`: interfaces. Ports must not import domain.
  - `adapters`: AWS implementations.
  - `handlers`: Lambda entry logic. They use the concrete
    `*domain.DeadmansHandleService` by design; ports are for the AWS side.
  - `mocks`: test doubles, with compile-time interface checks.
  - `config`: the config JSON type and `Config.Validate`.
  - `qrconfig`: the QR payload, `{"v":1,"endpoint":"https://…","apiKey":"…"}`,
    with an optional `push` object (`projectId`, `appId`, `apiKey`,
    `senderId`). Older apps ignore it, so it didn't need a new `v`.
- `android/`: the check-in app. Kotlin, Compose, `minSdk` 30. Its source is
  in `android/app/src/main/java/com/fireflycons/deadmanshandle/`:
  - `config/`: config, Keystore storage, QR payload.
  - `auth/`: BiometricPrompt.
  - `checkin/`: the HTTP request and its result.
  - `push/`: Firebase setup, the messaging service, notifications.
  - `ui/`: the screens.
- `terraform/`: AWS (API Gateway HTTP API, EventBridge, SSM, DynamoDB, S3,
  SES, SNS/CloudWatch alarms; `push.tf` when push is on). It ships in the
  release zip, so it must work for other deployers.

## Runtime behaviour (invariants to preserve)

- **Config:** one SecureString SSM parameter (`/deadmanshandle/config`)
  holding `owner`, `recipients`, `resetDays`, `warnDays`, `apiKey` and
  `documentKey`.
  - The Lambdas only read it, and validate it strictly.
  - The seed file (`config.json`) also has `timeout`, which seeds the state,
    and a `deployment` section (`senderEmail`, `documentBucket`). Terraform
    reads `deployment` and passes it to the Lambdas as env vars. Both are
    stripped from the stored parameter. Older seed files have `documentKey`
    in `deployment`; Terraform still accepts it there.
  - Terraform only creates the parameter and the state item
    (`ignore_changes`). Later changes go through `dmh config set`, which
    validates the result before writing it (DEPLOYMENT.md).
  - `/deadmanshandle/api-endpoint`, `document-bucket`, `state-table` and
    `sender-email` are plain parameters that Terraform keeps up to date, so
    the CLI can find the resources from the app name alone.
- **State:** one DynamoDB item (`id = "handle"` in `deadmanshandle-state`):
  `timeout`, `checkIns`, `sentTo`, `ownerNotified`, `documentETag`,
  `documentChangedAt`, `documentChangePending` and `pushToken`.
  - Every update is a single atomic `UpdateItem`.
  - A check-in is unconditional.
  - Recording a send or the owner notice is conditional on `checkIns` being
    unchanged since the run read the state.
  - The ETag is updated by compare-and-swap.
- **Check-in** (`POST /checkin`, header `x-api-key`): sets
  `timeout = now + resetDays`, clears the delivery state and increments
  `checkIns`.
  - It returns `newTimeout` in RFC 3339.
  - An optional body, `{"pushToken": "…"}`, records the token in the same
    `UpdateItem`. Without one the recorded token is kept; a malformed one is
    logged and ignored, and the check-in still succeeds.
  - The HTTP Lambda caches the config for 5 minutes.
  - The stage throttle (about 1 request a minute, burst 1) is shared by all
    callers.
- **Daily run:**
  - Within `warnDays` of the timeout, it warns the owner.
  - After the timeout, it sends the document to each recipient not in
    `sentTo`, recording each success, plus a one-off "triggered" notice to
    the owner. It stops if a record is refused because of a check-in, and
    errors if any send failed.
  - It always checks the document exists (`HeadObject`). If it's missing, the
    owner is emailed and, after the timeout, nothing is sent and the run
    errors.
  - `GetObject` uses `IfMatch` with the checked ETag.
- **Document changes:** S3 upload events reach the docwatch Lambda through
  EventBridge, and the daily run repeats the check.
  - The rule matches every upload to the bucket, since the key is in the
    config; docwatch ignores events for other keys.
  - The first ETag seen is recorded silently.
  - A new ETag sets `documentChangedAt` and `documentChangePending`, then
    emails the owner. A failed email stays pending and is retried 2 minutes
    or more later.
  - A deleted document keeps its recorded ETag.
- **Delivery hold:** after the timeout, if the document changed less than 24
  hours ago, the owner gets a "delivery held" email instead, and the run
  doesn't error. The hold runs from when the change was found.
- **Push (optional):** on when Terraform's `firebase_service_account_file` is
  set, which creates the SNS platform application (FCM HTTP v1) and passes
  `PUSH_PLATFORM_APPLICATION_ARN` to the Lambdas.
  - Every owner notice (any `EmailKind` but the document) is pushed to
    `pushToken` as well as emailed, as an FCM data message
    `{v, kind, title, body}` with the email's subject and text.
  - Push is best-effort: it's sent whether or not the email was, and a
    failure is only logged (counted by the `push-failed` alarm), so it never
    fails a run or changes the state.
  - The endpoint is created at send time (`CreatePlatformEndpoint` is
    idempotent); only the token is stored. SNS authorises
    `GetEndpointAttributes` against the application's ARN, so the IAM policy
    grants the actions on the application and its endpoints.
  - `alarmpush` takes the alarm topic's notifications and pushes those in
    ALARM. Unlike the notices, it returns a failed push, which raises
    `alarmpush-errors`.
- Deadlines are UTC in emails, as `Monday 2 January 2006 at 15:04 MST`. The
  app shows the same format in the phone's time zone.

## Android app decisions

- The API key is encrypted with an AES-GCM Android Keystore key that needs a
  strong biometric or the device credential for **every** use (timeout 0).
  Saving and checking in both prompt, through a BiometricPrompt CryptoObject.
  The endpoint is stored in clear.
- Removing the screen lock invalidates the key: the app clears the config, at
  startup or at check-in, and asks for it again. Enrolling a fingerprint does
  **not** invalidate it, because the key also accepts the device credential;
  the user accepted this.
- Without a screen lock no key can be created; the app checks
  `KeyguardManager.isDeviceSecure`.
- Backups and device transfers are off, so uninstalling erases the config.
- QR scanning uses Google's code scanner (`play-services-code-scanner`), a
  Play services module downloaded on demand; no camera permission is needed.
  Scanning fills the fields, and Save then prompts.
- Release builds are minified with R8. `proguard-rules.pro` keeps ML Kit
  whole: without it, creating the scanner crashes in the minified build.
- Push: Firebase is started at run time (`FirebaseApp.initializeApp` in
  `DeadmansHandleApp`) from the push settings stored with the config, so the
  APK holds no `google-services.json` and must not use the Google services
  plugin. `FirebaseInitProvider` is removed in the manifest.
  - The push settings are stored in clear. Manual edits keep the scanned
    ones; a code without them turns push off. Changing or removing them
    deletes the old token first.
  - Each check-in sends the current token. The app records the token sent and
    the latest one from `onNewToken`, and shows "check in to start them" while
    they differ.
  - Firebase Messaging 26 deprecates the token API for an opt-in
    `register()`/`onRegistered`. The token API is kept (with the deprecation
    suppressed) until SNS is known to accept the new tokens.
  - Warnings go to the Reminders channel, everything else to the high
    importance Alerts channel. A notice replaces the last of its kind;
    different alarms are kept apart.
- Signing: Gradle uses the release key when `ANDROID_KEYSTORE_FILE`,
  `ANDROID_KEYSTORE_PASSWORD` and `ANDROID_KEY_ALIAS` are set (PKCS12, so the
  key's password is the store's), and the debug key otherwise. A
  debug-signed APK can't update a release install.

## Commands

The environment is Windows; two workstations share this OneDrive folder.
`make` (GNU make 4.x) works from any shell; the Makefile uses cmd.exe under
native Windows make and sh elsewhere. Use Git Bash for POSIX commands.

```bash
go build ./... && go vet ./... && go test ./internal/...
make build          # bin/{http,scheduled,docwatch,alarmpush}/bootstrap (linux/arm64, CGO off, lambda.norpc)
make qrcode         # checkin-qr.png; QRFLAGS="-aws -firebase google-services.json" (state is in S3)
cd terraform && terraform init -backend=false && terraform validate
make android-test   # Android JVM unit tests and lint
make android        # Android release APK (debug-signed locally)
```

- **Android toolchain:** Android Studio only, with no Gradle or
  cmdline-tools on PATH.
  - JDK: Android Studio's JBR (JDK 25), which the Gradle daemon is pinned to.
    On Windows the Makefile sets `JAVA_HOME` to it, because the machines'
    `JAVA_HOME` is Java 8 or unset.
  - SDK: `%LOCALAPPDATA%\Android\Sdk`.
  - Build only through make.
- **Run make through the PowerShell tool, with output redirected to a file**
  (`make android-test *> <scratchpad>\build.log`), then read the log. Never
  pipe the output, and never run make via `powershell -Command` from Bash:
  the Gradle daemon keeps the handle open, so the call hangs.
- `AccessDenied`, or "Unable to delete directory" / "Cannot snapshot … not a
  regular file", under `android/app/build`: OneDrive is holding the files.
  Delete `android/app/build` and `android/build`, and build again.
- The local tflint is too old (0.34.1) for `precondition`/`check` blocks; CI's
  v0.64.0 is authoritative.

## Testing constraints

- **Go:** unit tests with the mocks and the real domain service, using the
  standard `testing` package, with tables where useful. There is no local
  Lambda environment.
- **Android JVM tests** cover the config (including the push settings and
  token registration), the QR payload, the check-in result and body, and the
  push message. Android's `org.json` is a stub on the JVM, so the tests use the real
  `org.json` (a `testImplementation`).
- The Keystore, the prompt, the scanner and HTTP need an emulator. Test the
  **minified release APK** (`make android`, then `adb install -r`), because
  R8 problems only show there.
- **Emulators** (adb is in `%LOCALAPPDATA%\Android\Sdk\platform-tools`):
  - Scanning needs a Google Play image signed in to a Google account. A Google
    APIs image has a stub Play Store and can't download the scanner.
  - On this machine: `Medium_Phone` (Play Store, signed in) and `S23_Ultra`
    (Google APIs).
  - Show a QR code to the scanner with Extended controls > Camera > Virtual
    scene images > Table, and look down at the table. The wall poster is
    hidden by furniture.
  - Enrol a fingerprint in settings, then `adb emu finger touch <id>`. Set a
    PIN with `adb shell locksettings set-pin 1234`.
  - After a cold boot, unlock the device before launching the app.
  - Prefer screenshots to `uiautomator dump`, which can miss text that is on
    screen. In Git Bash, set `MSYS_NO_PATHCONV=1` for `adb shell` paths.
  - The BiometricPrompt is a secure window, so screenshots of it are black;
    `uiautomator dump` shows its PIN pad, and `adb shell input text 1234`
    then Enter answers it. If the screen sleeps, `adb shell svc power stayon
    true`.
  - Push needs the Play image and a real Firebase project. Send a test push
    straight to the token's endpoint (`aws sns create-platform-endpoint`,
    then `aws sns publish --message-structure json`); it uses your
    credentials, not the Lambdas' role, so test the role by setting off an
    alarm (`aws cloudwatch set-alarm-state`). `adb shell dumpsys notification
    --noredact` lists what was posted.
- Testing against production changes live state (a check-in resets the
  deadline). A 401 or 429 can set off the `api-rejected-requests` alarm. Ask
  first.

## CI and releases

- **CI** (`.github/workflows/ci.yml`), on pushes to branches other than
  `master`, and on pull requests:
  - golangci-lint and `go test`
  - `terraform fmt`, `validate` and TFLint
  - Android unit tests, lint and a release build, on JDK 25
  - the `VERSION` check, on pull requests
  - `CI gate`, the job required by `master`'s ruleset; add new jobs to its
    `needs`
- **Release** (`release.yml`), on a merge to `master`: it creates
  `v<VERSION>` with `SHA256SUMS` and one zip,
  `deadmanshandle-v<VERSION>.zip`: Terraform, the example config, the Lambdas
  in `bin/`, the signed APK in `app/` and `dmh` for each desktop platform
  in `dmh/`, laid out ready to deploy.
  - The APK is signed from the repository secrets `ANDROID_KEYSTORE_BASE64`,
    `ANDROID_KEYSTORE_PASSWORD` and `ANDROID_KEY_ALIAS`, and the release fails
    without them.
  - Every release must use the same key, and the user keeps the keystore.
- **`VERSION`** (semver) must be bumped in each pull request to `master`; CI
  fails if the tag exists or isn't the highest. The app's `versionName` is
  `VERSION`, and its `versionCode` is `MAJOR*10000 + MINOR*100 + PATCH`.

## Deployment specifics (this owner's)

Identifying details are deliberately kept out of this file: the AWS account
ID, region, state bucket name, email addresses and domains. **At the start of
a session, check local memory for a `deployment-details` note. If it's
missing, ask the user for these details and save them to local memory**
before doing anything that needs them:
- AWS account ID and region
- Terraform state bucket name
- the owner, recipient and sender email addresses, and the sender domain
- the email path (forwarder and mailbox provider) and the custom MAIL FROM
  domain

Never write them into files in the repository.

- **Terraform state** is in S3, in the owner's state bucket. The key is
  `deadmanshandle/terraform.tfstate`, with `use_lockfile`.
  - The backend is set in `terraform/backend_override.tf`, which git ignores
    (`*_override.tf`); the committed configuration keeps local state for other
    deployers. **Never commit a backend block.**
  - A workstation without the override file needs a copy of it, then
    `terraform init`.
- The `make build` binaries are what `terraform apply` deploys. Go records
  the commit in each binary, so every rebuild shows as a code change.
- **Push** is on: `terraform.tfvars` (git-ignored) sets
  `firebase_service_account_file = "../firebase-service-account.json"`. That
  key and `google-services.json` are in the repository root, git-ignored. The
  Firebase project ID is in local memory.
  - The key is in the Terraform state, and in any saved plan: keep plans out
    of the repository (`*.tfplan` is ignored).

## Conventions

- All text files use LF line endings (`.gitattributes`); in Python, write
  with `newline='\n'`.
- Comment density as in the existing files. Terraform comments use `#`.
- Escape apostrophes in Android XML resources as `\'`; an unescaped one fails
  the build. Check after writing strings from a script, whose own escaping
  can drop the backslash.

## How we work

- One item at a time, then pause for the user to review. Commit only after
  they approve, one commit per item with a descriptive message. **Never
  push**; the user pushes and opens pull requests to `master`.
- Ask before behaviour decisions that are the user's call.
- CLAUDE.md is committed, so it must never contain identifying details; keep
  those in local memory (see Deployment specifics).

## Accepted limitations

- A flood exhausts the shared throttle and blocks check-ins too. HTTP APIs
  have no WAF or per-key usage plans, and a REST API was declined. The
  `api-rejected-requests` alarm tells the owner, who can check in with
  `dmh checkin`.
- Overlapping runs, or a send whose record fails, can deliver a document
  twice; a duplicate is preferred to a recipient missing out. A check-in
  can't recall an email already being sent.
- Lambda retries failed async runs twice, so notices can arrive three times.
- ETag change detection reports a multipart re-upload of the same bytes. Two
  uploads before any baseline are both "first". The requester is shown only
  as an account ID.
- A config change, including a new `apiKey`, takes up to 5 minutes to reach
  check-ins. Throttling is best-effort.
- Warnings round the day count down; the exact deadline is always shown.
- Push goes through Google and carries the email's text. It reaches one
  phone, the last to check in with a token. A new token isn't registered
  until the next check-in, and until then pushes fail (the app says so, and
  `push-failed` fires). Pushes repeat with Lambda's retries, as emails do.
