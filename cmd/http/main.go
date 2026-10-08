package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/handlers"
)

// configCacheTTL is how long a config change can take to reach check-ins
const configCacheTTL = 5 * time.Minute

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
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Initialize adapters. The config is cached on a warm instance, so a flood
	// of requests does not cost an SSM and KMS call each.
	configStore := adapters.NewCachingConfigStore(adapters.NewSSMConfigStore(ssmClient), configCacheTTL)
	stateStore := adapters.NewDynamoDBStateStore(dynamoClient, os.Getenv("STATE_TABLE_NAME"))
	keyValidator := adapters.NewSimpleAPIKeyValidator()

	// Initialize service
	service := domain.NewDeadmansHandleService()

	// Initialize handler
	paramName := os.Getenv("CONFIG_PARAMETER_NAME")
	httpHandler = handlers.NewHTTPHandler(configStore, stateStore, service, keyValidator, paramName)
}

// HandleHTTPRequest handles API Gateway requests
func HandleHTTPRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return httpHandler.Handle(ctx, request)
}

func main() {
	lambda.Start(HandleHTTPRequest)
}
