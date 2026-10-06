package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/handlers"
)

var httpHandler *handlers.HTTPHandler

func init() {
	ctx := context.Background()

	// JSON log lines on stdout, which Lambda sends to CloudWatch Logs
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	// Initialize AWS clients
	ssmClient := ssm.NewFromConfig(cfg)

	// Initialize adapters
	configStore := adapters.NewSSMConfigStore(ssmClient)
	keyValidator := adapters.NewSimpleAPIKeyValidator()

	// Initialize service
	service := domain.NewDeadmansHandleService()

	// Initialize handler
	paramName := os.Getenv("CONFIG_PARAMETER_NAME")
	httpHandler = handlers.NewHTTPHandler(configStore, service, keyValidator, paramName)
}

// HandleHTTPRequest handles API Gateway requests
func HandleHTTPRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return httpHandler.Handle(ctx, request)
}

func main() {
	lambda.Start(HandleHTTPRequest)
}
