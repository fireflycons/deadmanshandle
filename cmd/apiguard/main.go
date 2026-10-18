package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/handlers"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

var apiGuardHandler *handlers.APIGuardHandler

func init() {
	ctx := context.Background()

	// JSON log lines on stdout, which Lambda sends to CloudWatch Logs
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		panic(err)
	}

	// The throttle Terraform set, which reopening restores
	rate, err := strconv.ParseFloat(os.Getenv("API_RATE_LIMIT"), 64)
	if err != nil {
		panic(fmt.Errorf("API_RATE_LIMIT: %w", err))
	}
	burst, err := strconv.ParseInt(os.Getenv("API_BURST_LIMIT"), 10, 32)
	if err != nil {
		panic(fmt.Errorf("API_BURST_LIMIT: %w", err))
	}

	// Initialize adapters
	configStore := adapters.NewSSMConfigStore(ssm.NewFromConfig(cfg))
	stateStore := adapters.NewDynamoDBStateStore(dynamodb.NewFromConfig(cfg), os.Getenv("STATE_TABLE_NAME"))
	emailSender := adapters.NewSESEmailSender(ses.NewFromConfig(cfg), os.Getenv("SENDER_EMAIL"))
	gate := adapters.NewAPIGatewayGate(apigatewayv2.NewFromConfig(cfg), os.Getenv("API_ID"), os.Getenv("API_STAGE"))
	schedule := adapters.NewEventRuleSchedule(eventbridge.NewFromConfig(cfg), os.Getenv("CHECK_RULE_NAME"))
	alarms := adapters.NewCloudWatchAlarmStates(cloudwatch.NewFromConfig(cfg))

	// Push is set up only when Terraform has created the platform application
	var pushSender ports.PushSender
	if arn := os.Getenv("PUSH_PLATFORM_APPLICATION_ARN"); arn != "" {
		pushSender = adapters.NewSNSPushSender(sns.NewFromConfig(cfg), arn)
	}

	// Initialize handler
	apiGuardHandler = handlers.NewAPIGuardHandler(
		configStore,
		stateStore,
		emailSender,
		pushSender,
		gate,
		schedule,
		alarms,
		domain.NewDeadmansHandleService(),
		os.Getenv("CONFIG_PARAMETER_NAME"),
		os.Getenv("ALARM_NAME"),
		ports.Throttle{RateLimit: rate, BurstLimit: int32(burst)},
	)
}

// event holds the fields that tell the two kinds of invocation apart: an
// SNS notification from the alarms topic, and the check rule's scheduled
// event
type event struct {
	Records []events.SNSEventRecord `json:"Records"`
	Source  string                  `json:"source"`
}

// HandleEvent closes the API on the alarm, and checks whether to reopen it on
// the schedule
func HandleEvent(ctx context.Context, raw json.RawMessage) error {
	var e event
	if err := json.Unmarshal(raw, &e); err != nil {
		return fmt.Errorf("decoding the event: %w", err)
	}

	var err error
	switch {
	case len(e.Records) > 0:
		var errs []error
		for _, record := range e.Records {
			errs = append(errs, apiGuardHandler.HandleAlarm(ctx, record.SNS.Message))
		}
		err = errors.Join(errs...)
	case e.Source == "aws.events":
		err = apiGuardHandler.HandleCheck(ctx)
	default:
		err = errors.New("unexpected event: neither an SNS notification nor a scheduled event")
	}
	if err != nil {
		slog.Error("API guard failed", "error", err)
	}
	return err
}

func main() {
	lambda.Start(HandleEvent)
}
