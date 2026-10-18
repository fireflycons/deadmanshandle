package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/handlers"
)

var alarmPushHandler *handlers.AlarmPushHandler

func init() {
	ctx := context.Background()

	// JSON log lines on stdout, which Lambda sends to CloudWatch Logs
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Terraform deploys this Lambda only when push is set up
	applicationARN := os.Getenv("PUSH_PLATFORM_APPLICATION_ARN")
	if applicationARN == "" {
		panic("PUSH_PLATFORM_APPLICATION_ARN is not set")
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	// Initialize adapters
	stateStore := adapters.NewDynamoDBStateStore(dynamodb.NewFromConfig(cfg), os.Getenv("STATE_TABLE_NAME"))
	pushSender := adapters.NewSNSPushSender(sns.NewFromConfig(cfg), applicationARN)

	// Initialize handler
	alarmPushHandler = handlers.NewAlarmPushHandler(stateStore, pushSender)
}

// HandleAlarm handles the alarm notifications from the alarms SNS topic
func HandleAlarm(ctx context.Context, event events.SNSEvent) error {
	var errs []error
	for _, record := range event.Records {
		if err := alarmPushHandler.Handle(ctx, record.SNS.Message); err != nil {
			slog.Error("Alarm push failed", "error", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func main() {
	lambda.Start(HandleAlarm)
}
