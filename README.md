# Deadman's Handle Application

A Golang AWS Lambda application that implements a deadman's handle - a mechanism to send documents if the owner fails to check in periodically.

## Features

- **HTTP API Check-in**: Owner checks in via HTTP API to reset timeout
- **Android App**: Checks in with a fingerprint or the screen lock, and is set
  up by scanning a QR code; see
  [Setting Up the Android App](DEPLOYMENT.md#setting-up-the-android-app)
- **Scheduled Event Processing**: Daily EventBridge event checks timeout and sends warnings
- **Document Distribution**: On timeout, document from S3 is emailed to recipients. For additional security you should encrypt this document yourself prior to uploading to S3 and ensure the recipient(s) have the key and the knowledge to decrypt it upon receipt. A good choice is to put files into a [7-Zip](https://www.7-zip.org/) password protected archive which uses AES-256 encryption.
- **Warning Emails**: Owner is warned daily once the timeout is within `warnDays`
- **Alarms**: The owner is emailed if the daily run fails or does not run, or if
  the check-in API is flooded
- **Push Notifications** (optional): The owner's notices and the alarms are
  also pushed to the Android app, through SNS and Firebase Cloud Messaging; see
  [Push Notifications](DEPLOYMENT.md#5-push-notifications-optional)
- **CLI**: `dmh` manages a deployment with your AWS credentials: it checks in
  (`dmh checkin`), shows and changes the settings and state (`dmh config show`,
  `dmh config set`), downloads and replaces the document (`dmh document get`,
  `dmh document put`) and makes the app's setup QR code (`dmh qr`); see
  [Updating the Configuration](DEPLOYMENT.md#updating-the-configuration)

## How It Works

Once the timeout passes, the scheduled Lambda emails the document to each
recipient and sends the owner a notice that it has done so. Each send is
recorded as it succeeds, so the document goes out only once and a failed send
is retried on the next daily run without re-sending to anyone else. A
check-in resets the timeout and clears the record of sends. A send is only
recorded if no check-in has happened since the run started, so a check-in
during a run stops it after the email already being sent.

Every daily run also checks that the document is in S3. If it is missing, the
owner is emailed each day until it is uploaded. If the timeout has passed while
it is missing, the owner is told that nothing could be sent, and the run errors
(raising the alarm); once the document is uploaded, the next run delivers it.

The owner is also emailed whenever the document's content changes, so that an
unauthorised change can be put right before the document is sent. Each upload
is checked straight away, and every daily run checks again in case an upload
was missed. Only the first document ever seen is not reported, so deleting the
document and uploading a different one is reported too. Your own updates are
reported as well. If two checks see the same change at once, only one reports
it. If the email fails, it is sent again by a later check.

If the timeout passes within 24 hours of a change, delivery is held: the
owner is emailed that it is held, and the document is sent on the first daily
run after the 24 hours are up. This gives time to check in, or to restore the
document, if the change was not yours. The 24 hours run from when the change
was found, even if the owner could not be emailed about it, so delivery
cannot be held forever.
The daily run only sends the exact version it checked: if the document is
replaced while the run is in progress, nothing is sent and the run's retry
reports the change first.

For the architecture diagram, the AWS services used and where the state is
kept, see [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md#architecture).

## Deployment

You don't need to build anything: each
[release](https://github.com/fireflycons/deadmanshandle/releases) has the
pre-built Lambdas, the Terraform configuration, the example config, the
Android app and the `dmh` CLI. See:

- [QUICKSTART.md](QUICKSTART.md): up and running in about 10 minutes
- [DEPLOYMENT.md](DEPLOYMENT.md): the full guide, including the configuration
  rules, setting up the Android app, push notifications, updating the
  configuration and troubleshooting

## API Usage

### Check-in Endpoint

**POST** `/checkin`

Headers:
- `x-api-key`: Your API key from configuration

Body (optional): `{"pushToken": "…"}`, the app's push token, which the owner's
notices are then pushed to. Without one, the recorded token is kept.

The Android app makes this request for you.

Response:
```json
{
    "statusCode": 200,
    "message": "Check-in successful",
    "newTimeout": "2024-07-15T12:00:00Z"
}
```

## Security Considerations

- All S3 buckets have public access blocked
- The Parameter Store config is a SecureString, since it holds the API key
- Optional S3 bucket versioning (`document_versioning`); old versions expire after 30 days
- API key validation uses constant-time comparison
- The Lambdas' IAM role has only the permissions they need
- Every configuration field is validated, by `terraform plan` and by the Lambdas
- The API key and the document are never logged
- The SES sender domain is verified with DKIM by Terraform

## Documentation

- [QUICKSTART.md](QUICKSTART.md): a first deployment
- [DEPLOYMENT.md](DEPLOYMENT.md): deploying, configuring and running it
- [DEVELOPMENT.md](DEVELOPMENT.md): working with the source: building the
  Lambdas and the Android app, testing, CI and releases
- [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md): the architecture and the
  layout of the repository

## License

MIT; see [LICENSE](LICENSE).
