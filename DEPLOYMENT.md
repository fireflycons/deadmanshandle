# Deployment Guide

You don't need to build anything to deploy. Each
[release](https://github.com/fireflycons/deadmanshandle/releases) has the
pre-built Lambda binaries, the Terraform configuration, the example config
and the Android check-in app, in a single zip; see
[Deploying from a Release](#deploying-from-a-release). To build
from source instead, clone the repository and see
[DEVELOPMENT.md](DEVELOPMENT.md#building), then follow this guide from
[Pre-deployment Setup](#pre-deployment-setup).

## Prerequisites

1. **AWS Account**: With permissions for Lambda, API Gateway, S3, Parameter Store, DynamoDB, SES, EventBridge, SQS, SNS, CloudWatch, IAM and (for the DKIM records) Route 53
2. **Terraform 1.5+**: For infrastructure deployment
3. **AWS CLI**: Configured with credentials
4. **Go 1.26+** (see `go.mod`) and **Make**: Only to build from source
   (see [DEVELOPMENT.md](DEVELOPMENT.md))
5. **A Firebase project** (free): Only for push notifications to the Android
   app; see [Push Notifications](#5-push-notifications-optional)

## Deploying from a Release

### 1. Download and Verify

Pick a version from the
[releases page](https://github.com/fireflycons/deadmanshandle/releases) and
download its two files: `deadmanshandle-<version>.zip`, which holds
everything, and `SHA256SUMS`. (Releases before v0.3.0 have separate files;
follow the DEPLOYMENT.md linked from their notes.)

Bash (Linux, macOS, Git Bash):

```bash
VERSION=v0.5.0   # the release you want
BASE="https://github.com/fireflycons/deadmanshandle/releases/download/$VERSION"
curl -fsSLO "$BASE/deadmanshandle-$VERSION.zip"
curl -fsSLO "$BASE/SHA256SUMS"
sha256sum -c SHA256SUMS   # macOS: shasum -a 256 -c SHA256SUMS
```

PowerShell:

```powershell
$Version = 'v0.5.0'   # the release you want
$Base = "https://github.com/fireflycons/deadmanshandle/releases/download/$Version"
Invoke-WebRequest "$Base/deadmanshandle-$Version.zip" -OutFile "deadmanshandle-$Version.zip"
Invoke-WebRequest "$Base/SHA256SUMS" -OutFile SHA256SUMS
$hash = (-split (Get-Content SHA256SUMS))[0]
if ((Get-FileHash "deadmanshandle-$Version.zip" -Algorithm SHA256).Hash -ne $hash) { throw 'Checksum mismatch' }
```

With the GitHub CLI, `gh release download v0.5.0 -R fireflycons/deadmanshandle`
downloads both.

### 2. Unzip

```bash
unzip -q "deadmanshandle-$VERSION.zip"
cd "deadmanshandle-$VERSION"
```

```powershell
Expand-Archive "deadmanshandle-$Version.zip" -DestinationPath .
Set-Location "deadmanshandle-$Version"
```

The result is laid out like the repository, with the Lambdas already where
Terraform expects them:

```
deadmanshandle-v0.5.0/
├── app/deadmanshandle-v0.5.0.apk     # the Android app; see Setting Up the Android App
├── bin/{http,scheduled,docwatch,alarmpush}/bootstrap   # the Lambdas, linux/arm64
├── dmh/                              # the CLI that manages the deployment, for each platform
├── terraform/
├── config.example.json
└── LICENSE
```

`alarmpush` is deployed only with push notifications. On Windows the
`bootstrap` files lose their executable bit; that doesn't matter, because
Terraform sets it when it zips them for Lambda.

### 3. Configure and Deploy

Follow [Pre-deployment Setup](#pre-deployment-setup) and
[Deploy Infrastructure](#deploy-infrastructure) below, working in
`deadmanshandle-<version>/`: `config.json` goes next to `terraform/`, and you
run Terraform from `terraform/`.

### Upgrading to a Later Release

Terraform keeps its state in `terraform/terraform.tfstate`, in the directory
you deployed from. Keep that directory, and back the state file up: without
it, Terraform can't update or destroy what it created.

To upgrade, download and unzip the new release as above, then copy across
your `config.json`, `terraform/terraform.tfvars` and
`terraform/terraform.tfstate` (and the Firebase key, if you use push) from the
old directory, and run
`terraform init`, `terraform plan` and `terraform apply` from the new
`terraform/`. The plan shows the Lambdas updating; the configuration and the
countdown are not reset (see
[Updating the Configuration](#updating-the-configuration)).

**Upgrading from 0.5.x:** each Lambda now has its own IAM role. The plan
creates a `deadmanshandle-<function>-role` for each, moves the Lambdas to
them, and destroys `deadmanshandle-lambda-role`; there is nothing to do by
hand.

**Upgrading from 0.3.x:** the document's key has moved from the Lambdas'
settings into the stored configuration, as `documentKey`, and the Lambdas now
require it. Add it to the stored configuration **before** `terraform apply`;
the old Lambdas ignore it, but the new ones reject a configuration without
it, and check-ins would fail. Use the key from your `config.json`'s
`deployment` section, or `document.pdf` if it has none, with the new release's
`dmh` (see [Make the Setup QR Code](#2-make-the-setup-qr-code) for the program
for your computer):

```bash
./dmh/dmh-linux-amd64 config set -document-key document.pdf
```

Then move `documentKey` in `config.json` from the `deployment` section to the
top level, where new deployments have it, and apply. Until the apply,
`dmh config show` reports the parameters the new Terraform creates as not
found, and the other commands that need them refuse to run.

## Pre-deployment Setup

### 1. SES Sender Domain and Production Access

Terraform creates an SES identity for the domain of `deployment.senderEmail`
in the config file (in
`aws_region`) and verifies it with DKIM, which also makes the emails less
likely to be treated as spam. It needs three DKIM CNAME records:

- With `manage_dkim_dns_records = true` (the default), Terraform creates them
  in the Route 53 public hosted zone named after the domain, in the same
  account. Verification completes a few minutes after `terraform apply`.
- With `manage_dkim_dns_records = false`, add the records from the
  `ses_dkim_dns_records` output at your DNS provider. SES verifies the domain
  once they resolve (up to 72 hours).

If the domain is already a verified SES identity in that region (for example,
used by other applications), set `create_ses_identity = false`. Terraform then
uses it as it is and never creates, changes or deletes it or its DKIM records,
so `terraform destroy` leaves it alone. `manage_dkim_dns_records` is ignored
and `ses_dkim_dns_records` is empty.

**DMARC.** Set `dmarc_record` to the value of the domain's DMARC record, e.g.
`"v=DMARC1; p=none; rua=mailto:dmarc@example.com"`, to have Terraform create
it at `_dmarc.<domain>` in the same Route 53 zone. With a DMARC record, the
DKIM-signed mail passes DMARC, even when forwarded, which helps keep it out of
spam. The record covers all mail from the domain, so it is managed whatever
`create_ses_identity` is, and `terraform destroy` deletes it. Leave it empty
(the default) if the domain already has one or its DNS is elsewhere. To manage
an existing record, import it first:
`terraform import 'aws_route53_record.dmarc[0]' <zone-id>__dmarc.<domain>_TXT`.

**Request SES production access.** A new account is in the SES sandbox, where
mail is only delivered to verified addresses (200 a day). Recipients such as an
executor or attorney cannot be expected to verify, so request production access
in the SES console (Account dashboard) before relying on the handle.

### 2. Create Configuration File

Create `config.json` in the top directory (the repository root, or
`deadmanshandle-<version>/` for a release):

```bash
cp config.example.json config.json
# Edit config.json with your settings
```

Example configuration:
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
    "apiKey": "your-secure-random-api-key",
    "documentKey": "document.pdf",
    "deployment": {
        "senderEmail": "noreply@example.com",
        "documentBucket": ""
    }
}
```

`documentKey` is the S3 key of the document, default `document.pdf`.
Recipients receive the attachment under the key's file name. Like the rest of
the configuration, it can be changed after deployment without Terraform.

The `deployment` section holds infrastructure settings. Terraform reads it on
every plan, so changes to it take effect with `terraform apply`. It is not
stored in the parameter.

- `senderEmail`: Address the emails come from. Terraform verifies its domain
  in SES.
- `documentBucket`: (Optional) Bucket name. Empty or absent for a generated
  name.

Rules, checked by `terraform plan` and again by the Lambdas on every run:

- `owner`, `apiKey`, `documentKey` and every entry in `recipients` must be
  non-empty, and there must be at least one recipient.
- `resetDays` must be greater than 0, and `warnDays` must be from 0 to
  `resetDays - 1`.
- `deployment.senderEmail` must be an email address. The Lambdas do not
  check it.
- `timeout` must be an RFC 3339 time. Set it to your first deadline; each
  check-in then moves it to `resetDays` from now. It only seeds the DynamoDB
  state item when Terraform creates it. If it is already past then, the
  document is sent on the next daily run, so `terraform plan` warns about it.

If the stored config breaks these rules (for example after an
`aws ssm put-parameter`; `dmh config set` refuses to write an invalid one),
check-ins fail with HTTP 500 and the daily run errors, which raises the
`scheduled-errors` alarm. Nothing is sent until the config is fixed.

### 3. Prepare Terraform Variables

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
```

Key variables:
- `config_file_path`: Path to your config.json (default `../config.json`, the
  repo root when running from `terraform/`)
- `create_ses_identity`: `false` to use an existing verified SES identity for
  the domain instead of creating one
- `manage_dkim_dns_records`: Whether Terraform creates the DKIM records in Route 53
- `dmarc_record`: The domain's DMARC record value, created in Route 53; empty
  (the default) to leave DMARC alone
- `aws_region`: Your preferred region
- `document_versioning`: Versioning on the document bucket (default `false`).
  When `true`, old versions are deleted 30 days after being replaced or
  deleted. Setting it back to `false` suspends versioning
- `firebase_service_account_file`: Path to the Firebase project's service
  account key, to push the owner's notices to the Android app; empty (the
  default) leaves push off. See
  [Push Notifications](#5-push-notifications-optional)
- `api_flood_guard`: Close the check-in API automatically in a flood (default
  `true`); see [Closing the API in a Flood](#closing-the-api-in-a-flood)
- `api_guard_check_minutes`: How often the flood guard checks whether to
  reopen the API (default 15, from 1 to 1440). It only seeds the setting:
  after the first apply, change it with `dmh api check-interval`

## Deploy Infrastructure

### Step 1: Initialize Terraform

```bash
cd terraform
terraform init
```

### Step 2: Plan the Deployment

```bash
terraform plan -out=tfplan
```

Review the plan to ensure it will create the expected resources:
- API Gateway HTTP API with the `POST /checkin` route
- Lambda functions (HTTP, scheduled and document watch) and their log groups
- S3 bucket for the document
- EventBridge rules for the daily run and for uploads of the document, with an
  SQS dead-letter queue
- Parameter Store configuration (SecureString)
- DynamoDB table holding the timeout and delivery state, with its item seeded
  from the config file's `timeout`
- SES domain identity, unless `create_ses_identity` is false, and its DKIM
  records if `manage_dkim_dns_records` is true
- The domain's DMARC record, if `dmarc_record` is set
- SNS topic, owner email subscription and CloudWatch alarms
- An IAM role for each Lambda, with only the permissions it uses
- With `firebase_service_account_file` set: the SNS platform application for
  push, the `alarmpush` Lambda subscribed to the alarm topic, and the push
  alarms

The plan fails if the config file breaks the rules above, and warns if its
`timeout` is not in the future.

### Step 3: Apply the Deployment

```bash
terraform apply tfplan
```

This will create all resources and output:
- `api_endpoint`: Your check-in API endpoint
- `document_bucket_name`: S3 bucket name
- `config_parameter_name`: Parameter Store path
- `state_table_name`: DynamoDB table holding the state
- `http_lambda_function_name`, `scheduled_lambda_function_name`,
  `docwatch_lambda_function_name`: Lambda names
- `eventbridge_rule_name`: EventBridge rule name
- `alarm_topic_arn`: SNS topic for alarms
- `ses_dkim_dns_records`: DKIM records for the sender domain
- `push_platform_application_arn`, `alarmpush_lambda_function_name`: the push
  resources (null when push is off)

The owner then receives an email asking them to confirm the alarm subscription.

## Post-deployment Verification

### 1. Test the HTTP Endpoint

```bash
API_ENDPOINT="$(terraform output -raw api_endpoint)"
API_KEY="your-api-key"

curl -X POST "${API_ENDPOINT}/checkin" \
  -H "x-api-key: ${API_KEY}"
```

The API is throttled to about one request a minute, shared by all callers, not
per client. API Gateway throttles on a best-effort basis, so rapid calls are
mostly, but not always, rejected with HTTP 429.

The response holds the new deadline; see
[API Usage](README.md#check-in-endpoint) for its format.

### 2. Upload Document to S3

Upload it with `dmh document put` (see
[Make the Setup QR Code](#2-make-the-setup-qr-code) for the program for your
computer):

```bash
./dmh/dmh-linux-amd64 document put your-document.pdf
```

It uploads to a key named after the file, or to the one given with `-key`,
and if that isn't the configured `documentKey`, it then sets `documentKey` to
it. Recipients receive the attachment under the key's file name. A document
already at the old key is left there; delete it if you no longer need it.
`dmh document get` downloads the current document to the current directory.

Until the document is uploaded, each daily run emails the owner that it is
missing.

The first upload is recorded silently. After that, the owner is emailed
whenever the content changes, including your own updates. If the handle
triggers within 24 hours of a change, delivery is held until the 24 hours are
up; avoid replacing the document just before a deadline you might miss. An
upload under a new key may be reported by the next daily run rather than
straight away.

To upload with the AWS CLI instead, use the configured key:

```bash
aws s3 cp your-document.pdf "s3://$(terraform output -raw document_bucket_name)/document.pdf"
```

### 3. Test EventBridge Rule

Manually trigger the scheduled Lambda. This is a real run: if the timeout has
passed it sends the document, and if it is within `warnDays` it warns the
owner. An error also raises the `scheduled-errors` alarm.

```bash
aws lambda invoke \
  --function-name deadmanshandle-scheduled \
  response.json
cat response.json
```

### 4. Monitor CloudWatch Logs

```bash
# View HTTP Lambda logs
aws logs tail /aws/lambda/deadmanshandle-http --follow

# View Scheduled Lambda logs
aws logs tail /aws/lambda/deadmanshandle-scheduled --follow

# View API Gateway logs
aws logs tail /aws/apigateway/deadmanshandle --follow
```

The Lambdas log JSON lines: each check-in (accepted or rejected, with the
caller's IP), each email sent, and every failure with its error. The API key
and the document are never logged. Lambda logs are kept for 14 days and API
Gateway logs for 7.

Terraform creates the Lambda log groups. If the Lambdas ran before Terraform
managed them, AWS has already created the groups and `terraform apply` fails
with `ResourceAlreadyExistsException`. Import them once:

```bash
terraform import aws_cloudwatch_log_group.http_handler /aws/lambda/deadmanshandle-http
terraform import aws_cloudwatch_log_group.scheduled_handler /aws/lambda/deadmanshandle-scheduled
terraform import aws_cloudwatch_log_group.docwatch_handler /aws/lambda/deadmanshandle-docwatch
```

## Setting Up the Android App

The owner checks in with the Android app (source in `android/`). It keeps the
API endpoint and API key on the phone, with the key encrypted by an Android
Keystore key, and asks for a fingerprint or the screen lock each time it uses
the key. `curl`, as above, works too.

### Requirements

- Android 11 or later, with a screen lock (PIN, pattern or password). The app
  can't store the API key without one.
- Google Play services, to scan the setup QR code. Without it, enter the
  config by hand. Play services downloads the scanner when the app is
  installed, or on the first scan; until it has, the app says so.

### 1. Install the App

The app isn't in Google Play, so you install (sideload) the APK from the
release: `app/deadmanshandle-<version>.apk` in the unzipped release. It is
signed with the project's release key.

**From the phone's files:**

1. Copy the APK to the phone: over a USB cable (choose "File transfer" on the
   phone, then copy it into the phone's `Download` folder), or through a cloud
   drive such as Google Drive, or by emailing it to yourself.
2. On the phone, open the APK from the Files app (Downloads), the Drive app or
   the email.
3. Android says that app isn't allowed to install unknown apps. Tap
   **Settings**, turn on **Allow from this source**, and go back.
4. Tap **Install**. If Google Play Protect warns about an unknown app, tap
   **More details**, then **Install anyway** (the wording varies by phone).
5. Optionally, turn **Allow from this source** off again: Settings > Apps >
   Special app access > Install unknown apps, then the app you installed from.

**Or with adb**, from a computer with the Android SDK platform tools: on the
phone, turn on Developer options (tap Settings > About phone > Build number
seven times), then USB debugging in them, connect the cable and accept the
prompt on the phone. Then, in the unzipped release:

```bash
adb install -r app/deadmanshandle-v0.5.0.apk
```

**To update**, install a later release's APK the same way; the config is
kept. Every release is signed with the same key, so it installs over the last
one. A build signed with another key can't: Android refuses it rather than
replace the app, and uninstalling erases the app's config.

### 2. Make the Setup QR Code

The release's `dmh` CLI writes `checkin-qr.png`, a QR code holding the API
endpoint and the API key, and with `-firebase` the push settings (see
[Push Notifications](#5-push-notifications-optional)). It is in `dmh/`, built
for each platform: pick the one for your computer.

| Computer | Program |
| --- | --- |
| Windows (most PCs) | `dmh-windows-amd64.exe` |
| Windows on Arm | `dmh-windows-arm64.exe` |
| Mac with Apple silicon (M1 or later) | `dmh-darwin-arm64` |
| Mac with an Intel processor | `dmh-darwin-amd64` |
| Linux | `dmh-linux-amd64`, or `dmh-linux-arm64` on Arm |

Run `dmh qr` from `deadmanshandle-<version>/`, where you deployed from. By
default it reads the endpoint from `terraform/terraform.tfstate` and the key
from `config.json`:

```bash
./dmh/dmh-linux-amd64 qr
```

```powershell
.\dmh\dmh-windows-amd64.exe qr
```

To read the values the Lambdas use from Parameter Store instead, with the AWS
credentials in the environment (for example after changing the `apiKey`), add
`-aws`. This works from any directory.

`-out` and `-size` set the file and its width in pixels (default 1024), and
`-firebase` the Firebase project's `google-services.json`, for push.
`dmh qr -help` lists them. If the Terraform `application_name` isn't
`deadmanshandle`, give it before the command: `dmh -app <name> qr -aws`.

The programs aren't signed. On a Mac, if it says the program can't be opened,
run `xattr -d com.apple.quarantine dmh/dmh-darwin-*` once. If Windows blocks
it, run `Unblock-File .\dmh\*.exe` in PowerShell.

From a clone of the repository, `make qrcode` runs it instead (add the flags
with `QRFLAGS=`); see [DEVELOPMENT.md](DEVELOPMENT.md#the-setup-qr-code).

**The PNG holds the API key.** Show it on a screen rather than printing it, and
delete it once the app has scanned it. Git ignores `checkin-qr.png`.

### 3. Configure the App

Open the app and either tap **Scan QR code** and scan the code, or enter the
endpoint (`terraform output -raw api_endpoint`) and the API key by hand. Tap
**Save** and confirm with your fingerprint or screen lock. If the code has
push settings, the app then asks to show notifications.

### 4. Check In

Tap **Check in** and confirm with your fingerprint or screen lock. The app
shows the new deadline in the phone's time zone, or why the check-in failed.
The API allows about one request a minute.

### 5. Push Notifications (Optional)

The app can also receive the owner's notices as push notifications: the
warnings to check in, the missing document, delivery held or blocked, the
document changed, the handle triggered, and the CloudWatch alarms. Email stays
the record. A push is sent whether or not the email was, and a failed push
never fails a run. Pushes go through SNS mobile push to Firebase Cloud
Messaging (FCM). Each deployment uses its own Firebase project, so the
released app works with any deployment.

1. **Create a Firebase project.** In the
   [Firebase console](https://console.firebase.google.com), create a project
   (Google Analytics isn't needed) and add an **Android** app with the package
   name `com.fireflycons.deadmanshandle`. Download its `google-services.json`.
   Skip the remaining "add the Firebase SDK" steps: the app is already built,
   and gets the settings from the setup QR code instead of the Gradle plugin.
   Under **Project settings > Cloud Messaging**, check that the Firebase Cloud
   Messaging API (V1) is enabled.
2. **Download the server key.** Under **Project settings > Service accounts**,
   choose **Generate new private key**. **The key is a secret**: anyone with
   it can push to the app. Keep it, and `google-services.json`, in the top
   directory, as `firebase-service-account.json`; git ignores both names. The
   key is also kept in the Terraform state.
3. **Turn push on.** Set `firebase_service_account_file =
   "../firebase-service-account.json"` in `terraform.tfvars`, then run
   `terraform plan` and `terraform apply`.
4. **Make a QR code with the push settings**, adding `-firebase`:

   ```bash
   ./dmh/dmh-linux-amd64 qr -aws -firebase google-services.json
   ```

   with the program for your computer (see
   [Make the Setup QR Code](#2-make-the-setup-qr-code)).

5. **In the app**, tap **Change config**, scan the new code, tap **Save** and
   allow notifications. Then check in: each check-in sends the phone's push
   token, and the app shows "Notifications: on".

Good to know about push:

- **One phone**: pushes go to the last phone that checked in with push set up.
  Check-ins with `curl` don't change it.
- **New tokens**: Google can give the app a new token, for example after
  reinstalling or clearing its data. The app then shows "Notifications: check
  in to start them" until the next check-in sends it; meanwhile pushes fail,
  which raises the `push-failed` alarm.
- **Channels**: warnings go to the **Reminders** channel, and everything else
  to **Alerts**, which makes a sound. Change them in the app's notification
  settings.
- **Privacy**: a push carries the same text as the email (the recipients'
  addresses, the document's location), through Google.
- **Turning push off**: scan a code made without `-firebase`, or remove the
  variable and run `terraform apply`.

### Good to Know

- **Changing the API key:** after updating `apiKey` in Parameter Store (see
  below), tap **Change config** and scan a new code or enter the new key. The
  check-in Lambda may accept the old key for up to 5 minutes.
- **Removing the screen lock** invalidates the app's Keystore key, so the app
  clears its config and asks for it again. Enrolling a new fingerprint does
  not: the key also accepts the screen lock, which enrolling needs anyway.
- **The config is never backed up** or copied to a new phone, and uninstalling
  the app erases it. Set the app up again from a QR code.

## Updating the Configuration

Terraform only uses `config_file_path` to create the parameter and the state
item, apart from the `deployment` section (see above). After that, Terraform
ignores changes to both, so re-running `terraform apply` will not reset the
configuration or the countdown.

To see or change the configuration after deployment, use `dmh config` (see
[Make the Setup QR Code](#2-make-the-setup-qr-code) for the program for your
computer). It uses the AWS credentials in the environment.

`dmh config show` prints the settings, what Terraform fixed (the endpoint,
bucket, state table and sender), and the state: the deadline, the delivery
state, the document's last change and the push token. The API key and push
token are shortened unless you add `-show-secrets`, and `-json` prints JSON.

`dmh config set` changes the settings, with any of `-owner`,
`-recipients a@example.com,b@example.com` (which replaces the list),
`-reset-days`, `-warn-days`, `-api-key` and `-document-key`. It checks the
result against the rules above and writes nothing if it breaks them. For
example:

```bash
./dmh/dmh-linux-amd64 config set -recipients executor@example.com,attorney@example.com -warn-days 5
```

A new `-api-key` stops the app checking in until it scans a new QR code
(`dmh qr -aws`). `dmh document put` sets `-document-key` for you when it
uploads a document under a new name (see
[Upload Document to S3](#2-upload-document-to-s3)).

It can also change the state:

- `-timeout 2030-01-01T00:00:00Z` sets the deadline to that time. Like a
  check-in, it clears the delivery state, so a triggered handle starts over.
  It refuses a time that has passed, which would send the document on the
  next daily run, unless you add `-force`.
- `-clear-push-token` forgets the app's push token, to stop pushes to a lost
  phone. The next check-in from the app records a new one.

The check-in Lambda caches the configuration for 5 minutes on a warm instance,
so that a flood of requests does not cost an SSM and KMS call each. For up to
5 minutes after an update, check-ins may still use the old configuration,
including the old `apiKey`. The daily run and the document watch always read
the current configuration.

To change the `deployment` section, edit `config.json` and run
`terraform apply`.

### Checking in without the API

If the API is unavailable, or throttled by a flood of requests, check in with
`dmh checkin` (see [Make the Setup QR Code](#2-make-the-setup-qr-code) for
the program for your computer). It uses the AWS credentials in the
environment rather than the API key, and does what a check-in does: the
deadline moves to `resetDays` from now and the delivery state is cleared. The
app's push token is kept.

```bash
./dmh/dmh-linux-amd64 checkin
```

### Closing the API in a Flood

`dmh api disable` closes the check-in API: it sets the stage's throttle to
rate 0 and burst 0, so API Gateway refuses every request with HTTP 429
without invoking the Lambda. Requests still reach API Gateway, but nothing
gets through, not even with a valid API key. `dmh checkin` still works.
`dmh api enable` reopens it with the throttle Terraform set, and
`dmh api status` (or `dmh config show`) says whether it is open.

**The flood guard** does this for you (unless `api_flood_guard = false`).
When the `api-rejected-requests` alarm fires, the `apiguard` Lambda closes
the API and emails you (and pushes, with push on) that it has. Requests
refused while it is closed still count, so the alarm stays in ALARM for as
long as the flood goes on. Every `api_guard_check_minutes` (15 by default),
the guard checks the alarm, and once it is back to OK, which takes 5 to 10
minutes after the flood stops, reopens the API and tells you so. If the
closed notice fails, Lambda's retry sends it again; a failed reopened notice
isn't retried, but `apiguard-errors` tells you.

- `dmh api check-interval <minutes>` changes how often it checks, from 1 to
  1440 minutes, even while it has the API closed.
- `dmh api disable` and `dmh api enable` stop the guard's check, so it never
  reopens an API you closed yourself.
- `dmh api enable` during a flood leaves the API open: the alarm is already
  in ALARM, and SNS only notifies on a change of state, so the guard isn't
  told again until the alarm has cleared and fires once more.

The throttle is changed outside Terraform, so a `terraform apply` while the
API is closed reopens it; the guard's next check then stops.

## Cleanup

To remove all infrastructure:

```bash
cd terraform
terraform destroy
```

**Warning**: This deletes all the resources, including the Parameter Store
configuration, the DynamoDB state (and with it the current timeout), the SES
identity and its DKIM
records. The S3 bucket is only deleted when empty, so `terraform destroy`
fails until every object (and, if it is versioned, every object version) is
removed.

## Troubleshooting

### Lambda Execution Failures

Check CloudWatch Logs for detailed error messages:

```bash
aws logs tail /aws/lambda/deadmanshandle-http --follow
aws logs tail /aws/lambda/deadmanshandle-scheduled --follow
```

Common issues:
- Missing IAM permissions
- Invalid configuration (see the rules under "Create Configuration File")
- SES sender domain not verified
- S3 document not found

### Check-in Rejected

- HTTP 401 `Missing API key` or `Invalid API key`: send the config's `apiKey`
  in the `x-api-key` header. A new key can take 5 minutes to work.
- HTTP 500 `Failed to retrieve configuration` or `Failed to parse
  configuration`: the HTTP Lambda's logs give the cause. Check the parameter
  exists and follows the rules under "Create Configuration File".

### Permission Denied Errors

Verify the IAM policy of the Lambda that was refused (`http`, `scheduled`,
`docwatch`, `apiguard` or `alarmpush`):

```bash
aws iam get-role-policy --role-name deadmanshandle-scheduled-role \
  --policy-name deadmanshandle-scheduled-policy
```

### SES Email Not Sending

1. Check the sender domain is verified:
   `aws sesv2 get-email-identity --email-identity <domain>` should show
   `"VerifiedForSendingStatus": true`
2. In the SES sandbox, only verified recipients receive mail (200 emails a day);
   request production access
3. Verify recipient addresses are valid

### EventBridge Not Triggering

1. Verify rule is enabled: `aws events describe-rule --name deadmanshandle-daily-check`
2. Check Lambda permission: `aws lambda list-permissions --function-name deadmanshandle-scheduled`
3. Check the DLQ (`deadmanshandle-dlq`) and the `scheduled-not-run` alarm

### Push Notifications Not Arriving

1. The app should show "Notifications: on". If it asks you to check in, do;
   if notifications are blocked, allow them in its notification settings.
2. Search the scheduled and docwatch Lambdas' logs for `Push failed`, and the
   `alarmpush` Lambda's for `Alarm push failed`. "push token no longer valid"
   means Google dropped the token: check in from the app.
3. Check the platform application:
   `aws sns get-platform-application-attributes --platform-application-arn "$(terraform output -raw push_platform_application_arn)"`
   should show `"Enabled": "true"`.

## Monitoring

### Key Metrics to Monitor

- Lambda execution duration
- Lambda errors and throttles
- API Gateway 4xx and 5xx errors
- SES send statistics
- EventBridge rule triggers

### CloudWatch Alarms

Terraform creates these alarms (`terraform/alarms.tf`). Each one emails the
config file's `owner` through the SNS topic in the `alarm_topic_arn` output.
After `terraform apply`, the owner must confirm the subscription from the email
AWS sends. If you change the owner, update `config_file_path` as well as the
parameter, then run `terraform apply` so the subscription follows.

- `deadmanshandle-scheduled-errors`: the daily run returned an error. This
  includes failing to send to any recipient, and the document being missing
  after the timeout.
- `deadmanshandle-scheduled-not-run`: the daily run was not invoked in the last
  24 hours. This alarm fires once after the first deploy, until the first
  scheduled run happens.
- `deadmanshandle-docwatch-errors`: an upload of the document could not be
  checked, or the change could not be reported. The daily run checks again.
- `deadmanshandle-dlq-messages`: EventBridge could not invoke a Lambda.
- `deadmanshandle-api-rejected-requests`: the check-in API rejected 10 or more
  requests in 5 minutes (throttled, bad API key or unknown route). The
  throttle is shared by all callers, so a flood also blocks your check-ins.
  While it lasts, check in with `dmh checkin` (see
  [Checking in without the API](#checking-in-without-the-api)). The flood
  guard, if on, closes the API until the alarm clears (see
  [Closing the API in a Flood](#closing-the-api-in-a-flood)).
- `deadmanshandle-apiguard-errors` (with the flood guard on): the guard could
  not close or reopen the API, or tell you. Check `dmh api status`.

With push notifications on, the `alarmpush` Lambda also pushes each alarm to
the app, and there are three more alarms:

- `deadmanshandle-push-failed`: a notice could not be pushed (it was still
  emailed), most likely because the push token is no longer valid.
- `deadmanshandle-push-delivery-failed`: FCM refused a push that SNS sent.
- `deadmanshandle-alarmpush-errors`: an alarm could not be pushed.

## Cost Estimation

Usage is tiny (one scheduled run a day, occasional check-ins, a handful of
emails), so most services cost nothing or fractions of a cent. The fixed costs
are:

- CloudWatch alarms: about $0.10 per alarm per month (5 alarms, 8 with push)
- Route 53: the hosted zone's own charge, if it is not already paid for

Lambda, API Gateway, S3, Parameter Store (standard parameter), DynamoDB
(on-demand, plus point-in-time recovery for one tiny item), SQS, SNS email
and mobile push (the first million pushes a month are free), and SES (about
$0.10 per 1,000 emails) are negligible at this volume. Firebase Cloud
Messaging is free. Expect
well under $1 a month; check current AWS pricing for your region.
