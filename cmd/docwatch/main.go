package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
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

var documentWatchHandler *handlers.DocumentWatchHandler

// objectCreated is the part of an S3 "Object Created" event's detail used here
type objectCreated struct {
	Object struct {
		Key string `json:"key"`
	} `json:"object"`
	Requester string `json:"requester"`
	SourceIP  string `json:"source-ip-address"`
}

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
	documentWatchHandler = handlers.NewDocumentWatchHandler(
		configStore,
		documentStore,
		watcher,
		os.Getenv("CONFIG_PARAMETER_NAME"),
		os.Getenv("DOCUMENT_BUCKET"),
		os.Getenv("DOCUMENT_KEY"),
	)
}

// HandleDocumentEvent handles the EventBridge events for uploads of the document
func HandleDocumentEvent(ctx context.Context, event events.CloudWatchEvent) error {
	var detail objectCreated
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		err = fmt.Errorf("decoding %s event: %w", event.DetailType, err)
		slog.Error("Document check failed", "error", err)
		return err
	}
	slog.Info("Document uploaded", "key", detail.Object.Key, "requester", detail.Requester, "sourceIP", detail.SourceIP)

	err := documentWatchHandler.Handle(ctx, &domain.ChangeOrigin{
		Requester: detail.Requester,
		SourceIP:  detail.SourceIP,
	})
	if err != nil {
		slog.Error("Document check failed", "error", err)
	}
	return err
}

func main() {
	lambda.Start(HandleDocumentEvent)
}
