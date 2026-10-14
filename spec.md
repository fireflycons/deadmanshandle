# Project Spec: Deadman's handle app
Build a golang lambda using a hexagonal architecture. The core logic should be separated from the AWS services in such a way that it can be unit tested with mock interfaces for AWS.
The lambda will have 2 invocation methods:
1. Via HTTP API gateway, requiring an API key that will be stored in parameter store
2. Via daily Scheduled Event Bridge event

## Core Features
The purpose of the application is to email a document from S3 to recipient addresses in the event of the application owner's death.

The application owner must periodically check in via the HTTP API to reset a timeout. If the current time is after the timeout, then the document is mailed to the recipient addresses.
If the time till timeout is less than a configurable days, then the owner address is emailed to warn that a check-in is needed.
The above logic for managing count-downs and emails occurs when the lambda is invoked via Event Bridge.

Configuration is stored in parameter store as JSON
```json
{
    "owner": "owner@example.com",
    "recipients": [
        "recipient1@example.com",
        "recipient2@example.com",
    ],
    "resetDays": 30,
    "warnDays": 7,
    "timeout": "1985-04-12T23:20:50.52Z",
    "apiKey": "random string"
}
```

On successful checkin by the owner over the HTTP API, the JSON in the parameter store will be written back with the updated timeout calculated as `time of call to API (UTC) + resetDays`. The stored timeout value is ISO 3339 format.


## Tech Stack
- Golang (Lambda code), AWS Lambda, AWS API Gateway, AWS S3, AWS Parameter Store, AWS SES, AWS EventBridge, Terraform

## Golang project features
* Package name `github.com/fireflycons/deadmanshandle`
* With the exception of AWS imports, try to keep dependencies to standard library only.

## Deployment (Terraform)
* Generate terraform to build the infrastructure
* Create a bucket with no public access to store the document
* Create HTTP API and EventBridge event wired to lambda
* Expect initial configuration as a path to a JSON file and write to parameter store
* Create variables for unknowns
