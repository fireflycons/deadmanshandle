.PHONY: help build test clean deploy local

help:
	@echo "Available targets:"
	@echo "  build       - Build Go binaries"
	@echo "  test        - Run unit tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  deploy      - Deploy with Terraform"
	@echo "  local       - Run local tests"

build:
	@mkdir -p bin/http bin/scheduled
	@echo "Building HTTP handler..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/http/bootstrap ./cmd/http
	@echo "Building scheduled handler..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bin/scheduled/bootstrap ./cmd/scheduled

test:
	@echo "Running tests..."
	@go test -v ./...
	@echo "Tests completed"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf terraform/build/
	@echo "Clean completed"

deploy: build
	@echo "Deploying with Terraform..."
	@cd terraform && terraform init
	@cd terraform && terraform apply

local:
	@echo "Running local tests with code coverage..."
	@go test -v -cover ./...
