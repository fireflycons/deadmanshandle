# Deployment Guide

## Prerequisites

1. **AWS Account**: With permissions for Lambda, API Gateway, S3, Parameter Store, DynamoDB, SES, EventBridge, SQS, SNS, CloudWatch, IAM and (for the DKIM records) Route 53
2. **Go 1.26+** (see `go.mod`): For building
3. **Terraform 1.5+**: For infrastructure deployment
4. **AWS CLI**: Configured with credentials
5. **Make**: For build automation (optional but recommended)

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

**Request SES production access.** A new account is in the SES sandbox, where
mail is only delivered to verified addresses (200 a day). Recipients such as an
executor or attorney cannot be expected to verify, so request production access
in the SES console (Account dashboard) before relying on the handle.

### 2. Create Configuration File

Create `config.json` in the project root:

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
    "deployment": {
        "senderEmail": "noreply@example.com",
        "documentBucket": "",
        "documentKey": "document.pdf"
    }
}
```

The `deployment` section holds infrastructure settings. Terraform reads it on
every plan, so changes to it take effect with `terraform apply`. It is not
stored in the parameter.

- `senderEmail`: Address the emails come from. Terraform verifies its domain
  in SES.
- `documentBucket`: (Optional) Bucket name. Empty or absent for a generated
  name.
- `documentKey`: (Optional) S3 key of the document, default `document.pdf`.

Rules, checked by `terraform plan` and again by both Lambdas on every run:

- `owner`, `apiKey` and every entry in `recipients` must be non-empty, and
  there must be at least one recipient.
- `resetDays` must be greater than 0, and `warnDays` must be from 0 to
  `resetDays - 1`.
- `deployment.senderEmail` must be an email address, and
  `deployment.documentKey`, if given, must be non-empty. The Lambdas do not
  check these.
- `timeout` must be an RFC 3339 time. Set it to your first deadline; each
  check-in then moves it to `resetDays` from now. It only seeds the DynamoDB
  state item when Terraform creates it. If it is already past then, the
  document is sent on the next daily run, so `terraform plan` warns about it.

If the stored config breaks these rules (for example after a `put-parameter`),
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
- `aws_region`: Your preferred region
- `document_versioning`: Versioning on the document bucket (default `false`).
  When `true`, old versions are deleted 30 days after being replaced or
  deleted. Setting it back to `false` suspends versioning

## Build the Application

```bash
# Build the Lambda binaries
make build

# Or manually:
mkdir -p bin/http bin/scheduled bin/docwatch
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/http/bootstrap ./cmd/http
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/scheduled/bootstrap ./cmd/scheduled
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/docwatch/bootstrap ./cmd/docwatch
```

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
- SNS topic, owner email subscription and CloudWatch alarms
- IAM role and policies

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

The owner then receives an email asking them to confirm the alarm subscription.

## Post-deployment Verification

### 1. Test the HTTP Endpoint

```bash
API_ENDPOINT="$(terraform output -raw api_endpoint)"
API_KEY="your-api-key"

curl -X POST "${API_ENDPOINT}/checkin" \
  -H "x-api-key: ${API_KEY}"
```

The API is throttled to about one request a minute, so a second call straight
away gets HTTP 429. The limit is shared by all callers, not per client.

Expected response:
```json
{
    "statusCode": 200,
    "message": "Check-in successful",
    "newTimeout": "2024-07-15T12:00:00Z"
}
```

### 2. Upload Document to S3

The key must match `deployment.documentKey` (default `document.pdf`). Recipients receive
the attachment under the key's file name. Until the document is uploaded, each
daily run emails the owner that it is missing.

The first upload is recorded silently. After that, the owner is emailed
whenever the content changes, including your own updates.

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

Both Lambdas log JSON lines: each check-in (accepted or rejected, with the
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

## Updating the Configuration

Terraform only uses `config_file_path` to create the parameter and the state
item, apart from the `deployment` section (see above). After that, Terraform
ignores changes to both, so re-running `terraform apply` will not reset the
configuration or the countdown.

To update the configuration after deployment, write it directly. This no
longer touches the timeout, which is in DynamoDB:

```bash
# Update config.json
vi config.json

# Upload to Parameter Store
aws ssm put-parameter \
  --name /deadmanshandle/config \
  --value file://config.json \
  --overwrite \
  --type SecureString
```

The Lambdas ignore the `deployment` section and `timeout`. To change the
`deployment` section, edit `config.json` and run `terraform apply`; the
timeout is changed by checking in.

### Checking in without the API

If the API is unavailable, set the timeout in DynamoDB directly. This does
what a check-in does, including clearing the delivery state:

```bash
# A new timeout, resetDays from now (GNU date; on macOS: date -u -v+30d ...)
TIMEOUT=$(date -u -d '+30 days' +%Y-%m-%dT%H:%M:%SZ)

aws dynamodb update-item \
  --table-name deadmanshandle-state \
  --key '{"id":{"S":"handle"}}' \
  --update-expression 'SET #t = :t REMOVE sentTo, ownerNotified ADD checkIns :one' \
  --expression-attribute-names '{"#t":"timeout"}' \
  --expression-attribute-values "{\":t\":{\"S\":\"$TIMEOUT\"},\":one\":{\"N\":\"1\"}}"
```

## Testing

Run tests before deployment:

```bash
# Run all tests
make test

# Or manually:
go test -v ./...
```

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

### Permission Denied Errors

Verify IAM role policies:

```bash
aws iam get-role-policy --role-name deadmanshandle-lambda-role \
  --policy-name deadmanshandle-lambda-policy
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
  While it lasts, check in with the AWS CLI (see
  [Checking in without the API](#checking-in-without-the-api)).

## Cost Estimation

Usage is tiny (one scheduled run a day, occasional check-ins, a handful of
emails), so most services cost nothing or fractions of a cent. The fixed costs
are:

- CloudWatch alarms: about $0.10 per alarm per month (4 alarms)
- Route 53: the hosted zone's own charge, if it is not already paid for

Lambda, API Gateway, S3, Parameter Store (standard parameter), DynamoDB
(on-demand, plus point-in-time recovery for one tiny item), SQS, SNS email
and SES (about $0.10 per 1,000 emails) are negligible at this volume. Expect
well under $1 a month; check current AWS pricing for your region.
