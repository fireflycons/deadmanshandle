package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/handlers"
)

var scheduledHandler *handlers.ScheduledEventHandler

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
	s3Client := s3.NewFromConfig(cfg)
	sesClient := ses.NewFromConfig(cfg)

	// Initialize adapters
	configStore := adapters.NewSSMConfigStore(ssmClient)
	stateStore := adapters.NewDynamoDBStateStore(dynamoClient, os.Getenv("STATE_TABLE_NAME"))
	documentStore := adapters.NewS3DocumentStore(s3Client)
	emailSender := adapters.NewSESEmailSender(sesClient, os.Getenv("SENDER_EMAIL"))

	// Initialize service
	service := domain.NewDeadmansHandleService()

	// Initialize handler
	watcher := handlers.NewDocumentWatcher(stateStore, emailSender, service)
	paramName := os.Getenv("CONFIG_PARAMETER_NAME")
	docBucket := os.Getenv("DOCUMENT_BUCKET")
	docKey := os.Getenv("DOCUMENT_KEY")
	scheduledHandler = handlers.NewScheduledEventHandler(
		configStore,
		stateStore,
		documentStore,
		emailSender,
		service,
		watcher,
		paramName,
		docBucket,
		docKey,
	)
}

// HandleScheduledEvent handles EventBridge scheduled events
func HandleScheduledEvent(ctx context.Context, event interface{}) error {
	err := scheduledHandler.Handle(ctx)
	if err != nil {
		slog.Error("Scheduled run failed", "error", err)
	}
	return err
}

func main() {
	lambda.Start(HandleScheduledEvent)
}
