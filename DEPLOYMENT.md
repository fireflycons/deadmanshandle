# Deployment Guide

## Prerequisites

1. **AWS Account**: With appropriate permissions for Lambda, API Gateway, S3, Parameter Store, SES, and EventBridge
2. **Go 1.21+**: For building
3. **Terraform 1.0+**: For infrastructure deployment
4. **AWS CLI**: Configured with credentials
5. **Make**: For build automation (optional but recommended)

## Pre-deployment Setup

### 1. Verify SES Configuration

Email sending requires SES to be configured:

```bash
# Add and verify sender email in SES (Production Access or Sandbox)
# https://console.aws.amazon.com/ses/
```

For development/testing, request production access or use sandbox verified addresses.

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
    "timeout": "2024-12-31T23:59:59Z",
    "apiKey": "your-secure-random-api-key"
}
```

### 3. Prepare Terraform Variables

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
```

Key variables:
- `config_file_path`: Path to your config.json
- `sender_email`: Verified SES email address
- `aws_region`: Your preferred region
- `document_bucket_name`: (Optional) Custom bucket name

## Build the Application

```bash
# Build the Lambda binaries
make build

# Or manually:
mkdir -p bin/http bin/scheduled
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/http/bootstrap ./cmd/http
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/scheduled/bootstrap ./cmd/scheduled
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
- API Gateway with HTTP endpoint
- Lambda functions (HTTP and Scheduled)
- S3 bucket
- EventBridge rule
- Parameter Store configuration
- IAM roles and policies

### Step 3: Apply the Deployment

```bash
terraform apply tfplan
```

This will create all resources and output:
- `api_endpoint`: Your check-in API endpoint
- `document_bucket_name`: S3 bucket name
- `config_parameter_name`: Parameter Store path
- `eventbridge_rule_name`: EventBridge rule name

## Post-deployment Verification

### 1. Test the HTTP Endpoint

```bash
API_ENDPOINT="https://your-api-endpoint-id.execute-api.region.amazonaws.com/dev"
API_KEY="your-api-key"

curl -X POST "${API_ENDPOINT}/checkin" \
  -H "x-api-key: ${API_KEY}" \
  -H "Content-Type: application/json"
```

Expected response:
```json
{
    "statusCode": 200,
    "message": "Check-in successful",
    "newTimeout": "2024-07-15T12:00:00Z"
}
```

### 2. Upload Document to S3

```bash
aws s3 cp your-document.pdf s3://your-bucket-name/document.pdf
```

### 3. Test EventBridge Rule

Manually trigger the scheduled Lambda:

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

## Updating the Configuration

Terraform only uses `config_file_path` to create the parameter. After that, the
Lambda rewrites the value on every check-in, so Terraform ignores changes to it
and re-running `terraform apply` will not reset the countdown.

To update the configuration after deployment, write it directly. The `timeout`
in the file replaces the stored one, so set it to a future date (or check in
straight afterwards):

```bash
# Update config.json
vi config.json

# Upload to Parameter Store
aws ssm put-parameter \
  --name /deadmanshandle/dev/config \
  --value file://config.json \
  --overwrite \
  --type String
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

**Warning**: This will delete all resources including:
- Lambda functions
- API Gateway
- S3 bucket (if not protected)
- EventBridge rules
- Parameter Store configuration

## Troubleshooting

### Lambda Execution Failures

Check CloudWatch Logs for detailed error messages:

```bash
aws logs tail /aws/lambda/deadmanshandle-http --follow
aws logs tail /aws/lambda/deadmanshandle-scheduled --follow
```

Common issues:
- Missing IAM permissions
- Invalid configuration JSON
- SES sender not verified
- S3 document not found

### Permission Denied Errors

Verify IAM role policies:

```bash
aws iam get-role-policy --role-name deadmanshandle-lambda-role \
  --policy-name deadmanshandle-lambda-policy
```

### SES Email Not Sending

1. Verify sender email is SES verified
2. Check SES sending limits (100 emails/day in sandbox)
3. Verify recipient addresses are valid

### EventBridge Not Triggering

1. Verify rule is enabled: `aws events describe-rule --name deadmanshandle-daily-check`
2. Check Lambda permission: `aws lambda list-permissions --function-name deadmanshandle-scheduled`
3. Review CloudWatch Events history

## Monitoring

### Key Metrics to Monitor

- Lambda execution duration
- Lambda errors and throttles
- API Gateway 4xx and 5xx errors
- SES send statistics
- EventBridge rule triggers

### Set Up CloudWatch Alarms

```bash
# Create alarm for Lambda errors
aws cloudwatch put-metric-alarm \
  --alarm-name deadmanshandle-lambda-errors \
  --alarm-description "Alert on Lambda errors" \
  --metric-name Errors \
  --namespace AWS/Lambda \
  --statistic Sum \
  --period 300 \
  --threshold 1 \
  --comparison-operator GreaterThanOrEqualToThreshold \
  --dimensions Name=FunctionName,Value=deadmanshandle-http
```

## Cost Estimation

Typical monthly costs:
- Lambda invocations: $0.20 per million (minimal)
- API Gateway: $3.50 per million requests
- S3 storage: Minimal (single small file)
- Parameter Store: Minimal (single parameter)
- SES: $0.10 per email sent
- EventBridge: $0.35 per million events

Total for typical usage: ~$5-10/month
