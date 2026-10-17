# Client check in app for Android

## Stack

* Kotlin application for Android
* Android Studio is installed. Locate tooling and note in CLAUDE.md how you will use it.

## Development Strategy

* Break work into manageable pieces. Each completed piece to be human reviewed then committed separately.
* Do not push commits.

## App project

* Project should be loadable and buildable in Android Studio. Create necessary project files for this.
* Project should be able to be built from Makefile

## Functionality

* Needs to be configured with details of API gateway endpoint and API key
* Config should be stored securely on device
* Config should be erased if app is uninstalled.
* Config should be able to be input either manually or by scanning a QR which contains all the required information.
    * QR should be able to be generated as a PNG bitmap via a Go command line utility that reads the terraform state and local config. PNG can be generated as a file of sufficient resolution.
* Check-in functionality should be triggered via fingerprint (if configured on device), or request device PIN.

## Github Actions

* Adjust CI script to build and test andriod code
* Adjust Release script to add APK to release artifacts

## Push notifications

* The app can receive push notifications for all the same owner messages sent via email, and the alarms.
* Implemented with Firebase Cloud Messaging through SNS mobile push. Each deployment has its own Firebase project, whose client settings come from the setup QR code, so the released APK stays generic. Push is optional.
