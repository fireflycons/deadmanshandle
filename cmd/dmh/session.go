package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/adapters/sesaccount"
	"github.com/fireflycons/deadmanshandle/internal/cli"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// Compile-time checks that the adapters have what the CLI needs
var (
	_ cli.ConfigReadWriter = (*adapters.SSMConfigStore)(nil)
	_ cli.StateEditor      = (*adapters.DynamoDBStateStore)(nil)
	_ cli.DocumentTransfer = (*adapters.S3DocumentStore)(nil)
	_ cli.SESAccount       = (*sesaccount.Account)(nil)
)

// session holds the AWS clients of one command. Each command looks up only
// the parameters it needs, so that settings can be changed before an
// upgrade's terraform apply has created the newer parameters.
type session struct {
	globals
	aws aws.Config
	ssm *adapters.SSMConfigStore
}

func newSession(ctx context.Context, g globals) (*session, error) {
	awsCfg, err := g.awsConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &session{
		globals: g,
		aws:     awsCfg,
		ssm:     adapters.NewSSMConfigStore(ssm.NewFromConfig(awsCfg)),
	}, nil
}

// configParameter is the name of the stored config
func (s *session) configParameter() string {
	return s.parameter("config")
}

// passwordParameter is the name of the document password, which dmh creates;
// Terraform doesn't, so that the password is never in its state
func (s *session) passwordParameter() string {
	return s.parameter("document-password")
}

// lookup reads one of the plain parameters that Terraform keeps up to date
// (see terraform/parameter_store.tf)
func (s *session) lookup(ctx context.Context, name string) (string, error) {
	param := s.parameter(name)
	value, err := s.ssm.GetConfig(ctx, param)
	var notFound *types.ParameterNotFound
	if errors.As(err, &notFound) {
		return "", fmt.Errorf("%s not found: check -app and -region, and that this release's Terraform has been applied", param)
	}
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", param, err)
	}
	return string(value), nil
}

// stateStore returns the handle's state, in the table Terraform created
func (s *session) stateStore(ctx context.Context) (*adapters.DynamoDBStateStore, error) {
	table, err := s.lookup(ctx, "state-table")
	if err != nil {
		return nil, err
	}
	return adapters.NewDynamoDBStateStore(dynamodb.NewFromConfig(s.aws), table), nil
}

// documentStore returns the document store and the bucket Terraform created
func (s *session) documentStore(ctx context.Context) (*adapters.S3DocumentStore, string, error) {
	bucket, err := s.lookup(ctx, "document-bucket")
	if err != nil {
		return nil, "", err
	}
	return adapters.NewS3DocumentStore(s3.NewFromConfig(s.aws)), bucket, nil
}

// api returns the check-in API's published stage settings, its gate, and the
// flood guard's check schedule, which is nil when the guard is off
func (s *session) api(ctx context.Context) (*cli.APIStage, *adapters.APIGatewayGate, ports.CheckSchedule, error) {
	value, err := s.lookup(ctx, "api-stage")
	if err != nil {
		return nil, nil, nil, err
	}
	stage, err := cli.ParseAPIStage([]byte(value))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", s.parameter("api-stage"), err)
	}
	gate := adapters.NewAPIGatewayGate(apigatewayv2.NewFromConfig(s.aws), stage.APIID, stage.Stage)
	var schedule ports.CheckSchedule
	if stage.GuardRule != "" {
		schedule = adapters.NewEventRuleSchedule(eventbridge.NewFromConfig(s.aws), stage.GuardRule)
	}
	return stage, gate, schedule, nil
}

// sesAccount returns the SES account's sandbox state and identities
func (s *session) sesAccount() *sesaccount.Account {
	return sesaccount.New(sesv2.NewFromConfig(s.aws))
}

// emailSender sends from the sender address Terraform set, as the Lambdas do
func (s *session) emailSender(ctx context.Context) (*adapters.SESEmailSender, error) {
	sender, err := s.lookup(ctx, "sender-email")
	if err != nil {
		return nil, err
	}
	return adapters.NewSESEmailSender(ses.NewFromConfig(s.aws), sender), nil
}
