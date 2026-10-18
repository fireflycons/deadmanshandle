.PHONY: help build test clean deploy local qrcode android android-test

# Native Windows make runs recipes in cmd.exe, whether or not an sh.exe is on
# PATH. Elsewhere (macOS, Linux, MSYS2/Cygwin make) they run in sh.
ifeq ($(MAKE_HOST),Windows32)
SHELL := cmd.exe
.SHELLFLAGS := /c
rmdir = if exist $(subst /,\,$1) rmdir /s /q $(subst /,\,$1)
gradlew = cd android && .\gradlew.bat
# Android Studio's JDK and SDK; the machine's JAVA_HOME may be an older JDK.
# Override with ANDROID_JAVA_HOME=... or ANDROID_HOME=...
ANDROID_JAVA_HOME ?= C:\Program Files\Android\Android Studio\jbr
ANDROID_HOME ?= $(LOCALAPPDATA)\Android\Sdk
android android-test: export JAVA_HOME = $(ANDROID_JAVA_HOME)
android android-test: export ANDROID_HOME := $(ANDROID_HOME)
else
rmdir = rm -rf $1
gradlew = cd android && ./gradlew
endif

# $(info) drops leading spaces, so the help indent is made from this
indent := $(subst ,,  )

help:
	@$(info Available targets:)
	@$(info $(indent)build       - Build Go binaries)
	@$(info $(indent)test        - Run unit tests)
	@$(info $(indent)clean       - Clean build artifacts)
	@$(info $(indent)deploy      - Deploy with Terraform)
	@$(info $(indent)local       - Run local tests)
	@$(info $(indent)android     - Build the Android app: android/app/build/outputs/apk/release)
	@$(info $(indent)android-test - Run the Android unit tests and lint)
	@$(info $(indent)qrcode      - Write checkin-qr.png to set up the Android app (QRFLAGS=-aws reads AWS; -firebase adds push))
	@exit 0

# Exported so that go build sees them in either shell
build: export CGO_ENABLED = 0
build: export GOOS = linux
build: export GOARCH = arm64

# go build -o creates the bin/ directories
build:
	@echo Building HTTP handler...
	@go build -tags lambda.norpc -o bin/http/bootstrap ./cmd/http
	@echo Building scheduled handler...
	@go build -tags lambda.norpc -o bin/scheduled/bootstrap ./cmd/scheduled
	@echo Building document watch handler...
	@go build -tags lambda.norpc -o bin/docwatch/bootstrap ./cmd/docwatch
	@echo Building alarm push handler...
	@go build -tags lambda.norpc -o bin/alarmpush/bootstrap ./cmd/alarmpush

test:
	@echo Running tests...
	@go test -v ./...
	@echo Tests completed

clean:
	@echo Cleaning build artifacts...
	@$(call rmdir,bin)
	@$(call rmdir,terraform/build)
	@$(call rmdir,android/build)
	@$(call rmdir,android/app/build)
	@echo Clean completed

deploy: build
	@echo Deploying with Terraform...
	@terraform -chdir=terraform init
	@terraform -chdir=terraform apply

local:
	@echo Running local tests with code coverage...
	@go test -v -cover ./...

# Built for this machine, unlike the Lambdas. Reads terraform/terraform.tfstate
# and config.json, or with QRFLAGS=-aws the parameters in AWS. The PNG holds
# the API key, so delete it after scanning.
qrcode:
	@go run ./cmd/dmh qr $(QRFLAGS)

android:
	@$(gradlew) assembleRelease

android-test:
	@$(gradlew) testDebugUnitTest lintDebug
