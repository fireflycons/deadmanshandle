# Quick Start Guide

Get up and running with the Deadman's Handle application in about 10 minutes.
SES production access, which real recipients need, is a separate request to
AWS that can take a day; see step 4. Each step links to
[DEPLOYMENT.md](DEPLOYMENT.md) for the details.

## 1. Prerequisites (5 minutes)

Install [Terraform 1.5+](https://www.terraform.io/downloads.html) and the
[AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html),
then configure your AWS credentials:

```bash
aws configure
```

You need a domain you control for the sender address; see
[Prerequisites](DEPLOYMENT.md#prerequisites).

## 2. Download a Release (2 minutes)

From a [release](https://github.com/fireflycons/deadmanshandle/releases),
download `deadmanshandle-<version>.zip` and `SHA256SUMS`, check the zip's
checksum and unzip it; see
[Deploying from a Release](DEPLOYMENT.md#deploying-from-a-release) for the
commands. Work in the unzipped `deadmanshandle-<version>/` directory from here
on. It also holds the Android app, in `app/`.

To build from source instead, see [DEVELOPMENT.md](DEVELOPMENT.md#building)
and work in the repository root.

## 3. Prepare Configuration (1 minute)

```bash
cp config.example.json config.json
```

Edit `config.json`: your address as `owner` (it also receives the alarms),
the `recipients`, your first deadline as `timeout`, a random `apiKey` (e.g.
`openssl rand -hex 32`) and `deployment.senderEmail`, in your domain. See
[Create Configuration File](DEPLOYMENT.md#2-create-configuration-file) for
every field and the rules.

## 4. Deploy (2 minutes)

```bash
cd terraform
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`; see
[Prepare Terraform Variables](DEPLOYMENT.md#3-prepare-terraform-variables)
and, for the domain's DKIM records,
[SES Sender Domain](DEPLOYMENT.md#1-ses-sender-domain-and-production-access).
Request SES production access in the SES console; until then, mail only
reaches verified addresses.

```bash
terraform init
terraform apply
```

The owner receives an email asking them to confirm the alarm subscription;
confirm it so that failures reach you.

## 5. Test the API (< 1 minute)

```bash
API_ENDPOINT="$(terraform output -raw api_endpoint)"
API_KEY="your-api-key"

curl -X POST "${API_ENDPOINT}/checkin" -H "x-api-key: ${API_KEY}"
```

The response holds the new deadline, `newTimeout`. The API allows about one
request a minute, so quick repeated calls usually get HTTP 429; see
[Post-deployment Verification](DEPLOYMENT.md#post-deployment-verification).

## 6. Upload Your Document

From `deadmanshandle-<version>/`, with the `dmh` program for your computer
(see [Make the Setup QR Code](DEPLOYMENT.md#2-make-the-setup-qr-code)):

```bash
./dmh/dmh-linux-amd64 document password set
./dmh/dmh-linux-amd64 document put important-document.pdf
```

The first command makes the password that encrypts the document, and prints
it. The second encrypts the document with it, uploads it and makes it the one
that is sent. Give the password and the release's `recipients/` folder to
your recipients yourself, never by email; see
[Preparing Your Recipients](DEPLOYMENT.md#preparing-your-recipients).
`dmh config show` then shows the settings and the deadline.

The handle cannot send anything until the document is uploaded, and you are
emailed whenever it changes; see
[Upload Document to S3](DEPLOYMENT.md#2-upload-document-to-s3).

## What's Next?

- **The Android app**: sideload the APK from `app/`, check in with a
  fingerprint or the screen lock, and optionally get your notices and alarms
  as push notifications; see
  [Setting Up the Android App](DEPLOYMENT.md#setting-up-the-android-app)
- **Changing the configuration**: see
  [Updating the Configuration](DEPLOYMENT.md#updating-the-configuration)
- **Problems**: see [Troubleshooting](DEPLOYMENT.md#troubleshooting)
- **Removing it**: see [Cleanup](DEPLOYMENT.md#cleanup)
- **Cost**: well under $1 a month; see
  [Cost Estimation](DEPLOYMENT.md#cost-estimation)
- **How it works**: see [README.md](README.md#how-it-works) and
  [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md#architecture)
