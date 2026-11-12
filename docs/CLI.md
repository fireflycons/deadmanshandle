# dmh CLI Reference

`dmh` manages a deployment of the dead man's handle. It works on AWS
directly, not through the Lambdas or the check-in API, using the AWS
credentials in the environment. With it you can:

- check in
- show and change the settings and the state
- get and replace the document, and set the password that encrypts it
- close and reopen the check-in API
- make the Android app's setup QR code
- check that email will reach your recipients

For where it fits in a deployment, see [DEPLOYMENT.md](DEPLOYMENT.md).

- [Getting dmh](#getting-dmh)
- [Usage](#usage)
- [Commands](#commands)
  - [checkin](#dmh-checkin)
  - [config show](#dmh-config-show)
  - [config set](#dmh-config-set)
  - [document get](#dmh-document-get)
  - [document put](#dmh-document-put)
  - [document password set](#dmh-document-password-set)
  - [document password show](#dmh-document-password-show)
  - [api status](#dmh-api-status)
  - [api disable](#dmh-api-disable)
  - [api enable](#dmh-api-enable)
  - [api check-interval](#dmh-api-check-interval)
  - [qr](#dmh-qr)
  - [recipients status](#dmh-recipients-status)
  - [recipients test](#dmh-recipients-test)
- [Where dmh finds things](#where-dmh-finds-things)
- [AWS access](#aws-access)

## Getting dmh

Each release has `dmh` in `dmh/`, built for each platform:

| Computer | Program |
| --- | --- |
| Windows (most PCs) | `dmh-windows-amd64.exe` |
| Windows on Arm | `dmh-windows-arm64.exe` |
| Mac with Apple silicon (M1 or later) | `dmh-darwin-arm64` |
| Mac with an Intel processor | `dmh-darwin-amd64` |
| Linux | `dmh-linux-amd64`, or `dmh-linux-arm64` on Arm |

The programs aren't signed. On a Mac, if it says the program can't be opened,
run `xattr -d com.apple.quarantine dmh/dmh-darwin-*` once. If Windows blocks
it, run `Unblock-File .\dmh\*.exe` in PowerShell.

From a clone of the repository, `make dmh` builds `bin/dmh/dmh` (`dmh.exe` on
Windows) for your computer; see [DEVELOPMENT.md](DEVELOPMENT.md#building).

The examples below call it `dmh`. Use the program's path, such as
`./dmh/dmh-linux-amd64` or `.\dmh\dmh-windows-amd64.exe`, or rename it.

## Usage

```
dmh [-app name] [-region region] <command> [subcommand] [flags] [arguments]
```

**Global flags** come before the command:

| Flag | Default | Meaning |
| --- | --- | --- |
| `-app name` | `deadmanshandle` | Terraform's `application_name`, which prefixes the parameter names (`/<app>/config` and so on) |
| `-region region` | from the environment | AWS region of the deployment |

The AWS credentials and the default region come from the environment, as for
the AWS CLI: `AWS_PROFILE`, `AWS_REGION`, `AWS_ACCESS_KEY_ID` and so on, or
`~/.aws/config` and `~/.aws/credentials`.

**Flags** follow Go's conventions:

- `-flag value` and `-flag=value` both work, and so does `--flag`.
- A true/false flag is given on its own (`-force`); to give it a value, use
  `=` (`-force=false`).
- Flags come before a command's arguments. `dmh document put -key a.pdf
  file.pdf` works; `dmh document put file.pdf -key a.pdf` doesn't.

**Help:** `dmh -h` lists the commands, `dmh <command> -h` lists the
subcommands (for `config`, `document`, `document password`, `api` and
`recipients`), and `dmh <command> <subcommand> -h` lists the flags.

**Output and exit status:** results go to standard output, and an error is
printed to standard error as `dmh: <message>`, with exit status 1. Asking for
help exits with 0.

**Times** are shown in UTC, as in the emails, for example
`Monday 2 January 2006 at 15:04 UTC`, followed by your local time in brackets
when that differs.

## Commands

| Command | What it does |
| --- | --- |
| `checkin` | check in, resetting the deadline |
| `config show` | show the settings, what Terraform fixed, and the state |
| `config set` | change the settings, the timeout or the push token |
| `document get` | download the document to the current directory |
| `document put` | upload a new document, and point the settings at it |
| `document password set` | make a new password (or choose one), re-encrypting the document |
| `document password show` | show the password |
| `api status` | show whether the check-in API is open, and the flood guard |
| `api disable` | close the API: every request is refused with 429 |
| `api enable` | reopen the API with the throttle Terraform set |
| `api check-interval` | set how often the flood guard checks whether to reopen the API |
| `qr` | write the app's setup QR code |
| `recipients status` | show whether SES will deliver to the owner and each recipient |
| `recipients test` | send the owner and the recipients a test email, with a test document |

---

### dmh checkin

```
dmh checkin
```

Checks in, as the app does: the deadline moves to `resetDays` from now, the
delivery state is cleared (`sentTo`, `ownerNotified`), and `checkIns` goes up
by one. The app's push token is kept.

It writes the state directly, so it needs neither the API key nor the API, and
isn't throttled. Use it when the API is closed or flooded (see
[Closing the API in a Flood](DEPLOYMENT.md#closing-the-api-in-a-flood)).

It reads and validates the stored settings first, and fails if they are
invalid.

```
$ dmh checkin
Checked in. The new deadline is Saturday 12 December 2026 at 09:30 UTC
```

---

### dmh config show

```
dmh config show [-show-secrets] [-json]
```

Shows four things:

| Section | What it holds |
| --- | --- |
| Settings | the stored config: `owner`, `recipients`, `resetDays`, `warnDays`, `apiKey`, `documentKey`; and whether a document password is set |
| Fixed by Terraform | the API endpoint, the document bucket, the state table and the sender address |
| Check-in API | whether the API is open, with its throttle, and the flood guard's state |
| State (DynamoDB) | the deadline and how long is left, `checkIns`, `sentTo`, `ownerNotified`, the document's ETag, when it last changed and whether that change's notice is pending, and the push token |

| Flag | Meaning |
| --- | --- |
| `-show-secrets` | show the API key, the document password and the push token in full. Otherwise the API key is cut to its first 4 characters (or `********`), the password shows only `set` or `not set`, and the token is cut to 16 characters |
| `-json` | print the report as JSON instead |

An invalid config is still shown, since `dmh config set` is how it gets fixed.
Anything that can't be read is listed under **Problems** rather than stopping
the command. For example, the parameters a newer release creates are missing
until its `terraform apply`.

```
$ dmh config show
Settings (Parameter Store /deadmanshandle/config)
  owner             owner@example.com
  recipients        executor@example.com, attorney@example.com
  resetDays         30
  warnDays          7
  apiKey            k3Yz…
  documentKey       will.pdf.dmh
  documentPassword  set

Fixed by Terraform
  endpoint        https://abc123.execute-api.eu-west-2.amazonaws.com
  documentBucket  deadmanshandle-documents-123456789012
  stateTable      deadmanshandle-state
  senderEmail     dmh@example.com

Check-in API
  status       open (rate 0.017/s, burst 1)
  flood guard  on; closes the API in a flood, then checks every 15 minutes whether to reopen it

State (DynamoDB)
  timeout                Saturday 12 December 2026 at 09:30 UTC, in 30 days
  checkIns               42
  sentTo                 (none)
  ownerNotified          false
  documentETag           "0f343b0931126a20f133d67c2b018a3b"
  documentChangedAt      Tuesday 3 November 2026 at 18:02 UTC
  documentChangePending  false
  pushToken              dGhpcyBpcyBub3Qg…
```

The values are illustrative.

---

### dmh config set

```
dmh config set [flags]
```

Changes the settings, the deadline or the push token. Give at least one flag.

**Settings** (the `/<app>/config` parameter):

| Flag | Setting | Notes |
| --- | --- | --- |
| `-owner address` | `owner` | the owner's email address |
| `-recipients a@x,b@y` | `recipients` | comma-separated; **replaces** the whole list. Spaces around the commas are trimmed |
| `-reset-days n` | `resetDays` | days from a check-in to the deadline |
| `-warn-days n` | `warnDays` | days before the deadline that the daily warnings start |
| `-api-key key` | `apiKey` | the key the app checks in with. The app then needs a new QR code (`dmh qr -aws`) |
| `-document-key key` | `documentKey` | the document's S3 key. The document must be at that key; `dmh document put` sets this for you |

The settings are read, changed and checked against the same rules the Lambdas
apply, and written only if the result is valid. If it isn't, nothing is
written and every problem is listed. The rules:

- `owner` is an email address
- `recipients` has at least one entry, each an email address
- `resetDays` is greater than 0
- `warnDays` is at least 0 and less than `resetDays`
- `apiKey` and `documentKey` aren't empty

Settings it doesn't know are kept as they are. Check-ins can take up to 5
minutes to see a change, because the check-in Lambda caches the settings; the
daily run and the document watch see it straight away.

The settings need only the config parameter, so they can be changed before an
upgrade's `terraform apply`.

**State** (the DynamoDB item):

| Flag | Meaning |
| --- | --- |
| `-timeout time` | set the deadline to an RFC 3339 time, such as `2030-01-01T00:00:00Z`. Like a check-in, it clears the delivery state, so a handle that has triggered starts over. `checkIns` goes up by one, and the push token is kept |
| `-force` | allow a `-timeout` that has passed. The document is then sent on the next daily run |
| `-clear-push-token` | forget the app's push token, to stop pushes to a lost phone. The next check-in from the app records a new one |

The settings are written first, then the state, so if one flag fails, the
changes before it have still been made.

```bash
dmh config set -recipients executor@example.com,attorney@example.com -warn-days 5
dmh config set -timeout 2027-06-30T12:00:00Z
dmh config set -clear-push-token
```

---

### dmh document get

```
dmh document get [-force] [-raw]
```

Downloads the document named by `documentKey` to the current directory.

An encrypted document is decrypted with the stored document password and
saved under its original name. If the password can't be read, or doesn't open
it, `dmh` asks for it on the terminal, up to three times. A document that
isn't encrypted is saved under the key's file name.

| Flag | Meaning |
| --- | --- |
| `-raw` | save the document as stored (the `.dmh` file, for example), without decrypting it. This is what recipients receive |
| `-force` | replace a file of the same name. Without it, an existing file is left alone and the command fails |

The file is created readable by you only. The name from inside an encrypted
document is reduced to a plain file name, so it can't write outside the
current directory, and characters Windows doesn't allow become `_`.

```
$ dmh document get
Wrote will.pdf
```

---

### dmh document put

```
dmh document put [-key key] [-plain] <file>
```

Encrypts `<file>` with the stored document password and uploads it to the
document bucket. If the key it uploads to isn't the current `documentKey`,
`documentKey` is then changed to it.

| Flag | Meaning |
| --- | --- |
| `-key key` | the S3 key to upload to. The default is the file's name plus `.dmh` (`will.pdf` → `will.pdf.dmh`), or the file's name alone with `-plain` |
| `-plain` | upload the file as it is, without encrypting it |

- The file's original name is stored inside the encrypted document and
  restored when it's opened. Recipients receive the attachment under the
  key's file name.
- Without a stored password, `put` refuses unless given `-plain`; run
  `dmh document password set` first.
- If the password parameter can't be read (with credentials that may not
  read it, say), `dmh` asks for the password, twice. It must be the stored
  one, or recipients couldn't open the document.
- The upload happens before `documentKey` changes, so the settings never name
  a document that isn't there. If changing `documentKey` then fails, the
  error says so and the upload stays.
- A document at the old key is left in the bucket. Delete it if you no longer
  need it.

Every upload counts as a change of the document. The owner is emailed about
it, and if the deadline passes within 24 hours of the change, delivery is held
until the 24 hours are up. An upload under a new key may be reported by the
next daily run rather than straight away.

```
$ dmh document put will.pdf
Encrypted and uploaded will.pdf to s3://deadmanshandle-documents-123456789012/will.pdf.dmh
You'll be emailed that the document changed. If the deadline passes within
24 hours of the change, delivery is held until the 24 hours are up.
```

---

### dmh document password set

```
dmh document password set [-words n] [-prompt]
```

Sets the password that encrypts the document, and stores it in the
SecureString parameter `/<app>/document-password`. Only `dmh` creates or
reads this parameter: it is never in Terraform's state, and no Lambda can read
it. It is never emailed: give it to your recipients yourself (see
[Preparing Your Recipients](DEPLOYMENT.md#preparing-your-recipients)).

| Flag | Default | Meaning |
| --- | --- | --- |
| `-words n` | `6` | the number of words in a generated passphrase |
| `-prompt` | | type your own password instead, twice |

By default it generates a passphrase of random words from the EFF short word
list, such as `acid-corn-acre-zone-zoom-kiwi`, and prints it before changing
anything, so it isn't lost if a later step fails.

A password you choose with `-prompt` must be at least 12 characters, using
only letters, digits, spaces and the punctuation on an English keyboard (no
accented letters). That way it can be typed on any computer: a Windows console
may pass other characters to a program differently.

What happens to the stored document:

| The document is | Then |
| --- | --- |
| encrypted with the old password | it is re-encrypted with the new one and uploaded **before** the password is saved, so the stored password always opens it. This counts as a change of the document: you're emailed, and delivery is held for 24 hours |
| already encrypted with the new password | left alone |
| encrypted with neither password | nothing is changed, and the command fails. Set the password it was encrypted with (`-prompt`), or replace it with `dmh document put` |
| not encrypted | left alone; `dmh document put` encrypts it |
| missing | nothing to do; `dmh document put` encrypts one with the new password |

If the document was re-encrypted but saving the password then fails, the
error says so. Run `dmh document password set -prompt` and enter that
password.

If the new password replaces an older one, your recipients need the new one.

---

### dmh document password show

```
dmh document password show
```

Prints the stored document password, and nothing else, so it can be used in
a script. It fails if no password is set.

---

### dmh api status

```
dmh api status
```

Shows whether the check-in API is open, with its throttle, and the flood
guard's state:

```
$ dmh api status
The check-in API is open (rate 0.017/s, burst 1)
Flood guard: on; closes the API in a flood, then checks every 15 minutes whether to reopen it
```

The flood guard is one of:

- `off`: Terraform's `api_flood_guard` is `false`
- `on; closes the API in a flood, then checks every N minutes whether to
  reopen it`: waiting for a flood
- `on; it closed the API and checks every N minutes whether to reopen it`:
  the guard has closed the API

`dmh config show` shows the same.

---

### dmh api disable

```
dmh api disable
```

Closes the check-in API. It sets the stage's throttle to rate 0 and burst 0,
so API Gateway refuses every request with HTTP 429 without invoking the
Lambda, even one with a valid API key. `dmh checkin` still works.

It stops the flood guard's check first, so the guard never reopens an API you
closed yourself.

The throttle is changed outside Terraform, so a `terraform apply` reopens the
API.

---

### dmh api enable

```
dmh api enable
```

Reopens the check-in API with the throttle Terraform set (published in
`/<app>/api-stage`), and stops the flood guard's check.

During a flood, this leaves the API open. The alarm is already in ALARM, and
SNS only notifies on a change of state, so the guard isn't told again until
the alarm clears and fires once more.

---

### dmh api check-interval

```
dmh api check-interval <minutes>
```

Sets how often the flood guard checks, once it has closed the API, whether to
reopen it: from 1 to 1440 minutes. It works while the guard has the API
closed, and doesn't start or stop the check. It fails if the flood guard is
off.

Terraform sets the first interval (`api_guard_check_minutes`, 15 by default)
and ignores it after that, so this setting lasts across applies.

```
$ dmh api check-interval 5
Flood guard: on; closes the API in a flood, then checks every 5 minutes whether to reopen it
```

See [Closing the API in a Flood](DEPLOYMENT.md#closing-the-api-in-a-flood)
for how the flood guard works.

---

### dmh qr

```
dmh qr [-state file] [-config file] [-aws] [-firebase file] [-out file] [-size n]
```

Writes a PNG QR code that sets up the Android app: it holds the API endpoint
and the API key, and with `-firebase` the push settings.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-state file` | `terraform/terraform.tfstate` | Terraform state to read the endpoint from |
| `-config file` | `config.json` | config file to read the `apiKey` from |
| `-aws` | | read the endpoint and API key from Parameter Store instead, the values the Lambdas use. Not with `-state` or `-config` |
| `-firebase file` | | the Firebase project's `google-services.json`, to turn on push notifications (see [Push Notifications](DEPLOYMENT.md#5-push-notifications-optional)) |
| `-out file` | `checkin-qr.png` | the PNG file to write |
| `-size n` | `1024` | width and height of the PNG, in pixels |

Without `-aws`, run it from the directory you deployed from, so the default
paths are found. It needs no AWS credentials then, but it reads local state,
so with an S3 backend use `-aws`. Use `-aws` too after changing the
`apiKey` with `dmh config set`. With `-aws`, it works from any directory.

The file is created readable by you only. It holds the API key: delete it once
the app has scanned it.

```
$ dmh qr -aws -firebase google-services.json
Wrote checkin-qr.png for https://abc123.execute-api.eu-west-2.amazonaws.com
Push notifications: Firebase project my-project
It contains the API key: delete it once the app has scanned it.
```

From a clone of the repository, `make qrcode` runs it (with flags in
`QRFLAGS=`); see [DEVELOPMENT.md](DEVELOPMENT.md#the-setup-qr-code).

---

### dmh recipients status

```
dmh recipients status
```

Shows whether SES will deliver email to the owner and to each recipient. It
changes nothing.

Out of the SES sandbox, SES delivers to any address. In the sandbox, it only
delivers to addresses that are verified identities, or are in a verified
domain. Each address is one of:

| Status | Meaning |
| --- | --- |
| `can receive` | SES is out of the sandbox |
| `verified` | the address is a verified identity |
| `verified (domain example.com)` | the address's domain is a verified identity |
| `not verified: waiting for its SES verification link to be followed` | SES has emailed it a verification link |
| `not verified: dmh recipients test asks SES to verify it` | it has no identity yet, or its link expired |

```
$ dmh recipients status
SES is in the sandbox: it delivers only to verified addresses, and addresses in
verified domains. dmh recipients test asks SES to verify the others; to leave the
sandbox, request production access in the SES console (Account dashboard).

owner      owner@example.com                   verified (domain example.com)
recipient  executor@example.com                verified (domain example.com)
recipient  attorney@example.org                not verified: dmh recipients test asks SES to verify it
```

To leave the sandbox, see
[SES Sender Domain and Production Access](DEPLOYMENT.md#1-ses-sender-domain-and-production-access).

---

### dmh recipients test

```
dmh recipients test [recipient...]
```

Sends a test email from the sender address, with an attached test document,
`dmh-test.txt.dmh`, encrypted with the document password. It goes to the
owner and to every recipient, or to the owner and only the recipients named.
Each one named must be in `recipients` (case doesn't matter).

Use it to check that the email reaches each inbox rather than the junk folder,
and that each recipient can open the attachment with the password, using
`open-document` or `decrypt.html` (see
[Preparing Your Recipients](DEPLOYMENT.md#preparing-your-recipients)).

- It changes nothing in the dead man's handle: not the deadline, not the
  delivery state, not the document.
- It needs a document password; set one with `dmh document password set`.
- In the SES sandbox, an address that isn't verified isn't sent the test.
  SES is asked to email it a verification link (from
  `no-reply-aws@amazon.com`) instead, and an expired request is replaced.
  An address still waiting for its link to be followed is skipped. Run the
  test again once the link has been followed.
- Emails are sent no faster than the account's SES sending rate.
- If any address fails, the others are still tried, and the command then
  fails, saying how many did.

Each address shows `sent`, `not sent: not verified, so SES has sent it a
verification link`, `not sent: waiting for its SES verification link to be
followed`, or `failed: <reason>`.

```
$ dmh recipients test executor@example.com
owner      owner@example.com                   sent
recipient  executor@example.com                sent

The test changes nothing in the deadman's handle. Ask the recipients whether it
reached their inbox, and whether the attachment opened with the password.
```

## Where dmh finds things

`dmh` needs only the application name (`-app`) and the region. It finds
everything else in Parameter Store, under `/<app>/`:

| Parameter | Kept by | Holds | Used by |
| --- | --- | --- | --- |
| `config` | created by Terraform, then changed only by `dmh config set` and `dmh document put` | the settings (SecureString) | most commands |
| `document-password` | `dmh document password set` only | the document password (SecureString) | `document`, `recipients test`, `config show` |
| `api-endpoint` | Terraform | the check-in API's URL | `qr -aws`, `config show` |
| `document-bucket` | Terraform | the document bucket's name | `document`, `config show` |
| `state-table` | Terraform | the DynamoDB state table's name | `checkin`, `config set -timeout`/`-clear-push-token`, `config show` |
| `sender-email` | Terraform | the address emails are sent from | `recipients test`, `config show` |
| `api-stage` | Terraform | the API's ID, stage, open throttle and flood guard rule | `api`, `config show` |

Each command looks up only the parameters it needs. If one is missing, the
error names it: check `-app` and `-region`, and that the release's Terraform
has been applied.

## AWS access

`dmh` assumes the owner's credentials, with access to the deployment's
resources. By command, it uses:

| Command | AWS services |
| --- | --- |
| `checkin` | Parameter Store, DynamoDB |
| `config show` | Parameter Store, DynamoDB, API Gateway, EventBridge |
| `config set` | Parameter Store; DynamoDB for `-timeout` and `-clear-push-token` |
| `document get`, `document put` | Parameter Store, S3 |
| `document password set` | Parameter Store, S3 |
| `document password show` | Parameter Store |
| `api` | Parameter Store, API Gateway, EventBridge |
| `qr` | none; Parameter Store with `-aws` |
| `recipients status` | Parameter Store, SES |
| `recipients test` | Parameter Store, SES |

Reading the SecureString parameters needs KMS decryption with the AWS-managed
key `alias/aws/ssm`.

Recipients don't use `dmh`. They open the document with `open-document` or
`decrypt.html`. A recipient with AWS access can be given the
`DMH-<app>-Recipient` policy, which reads only the document password.
