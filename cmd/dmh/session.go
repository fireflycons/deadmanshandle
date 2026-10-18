package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/fireflycons/deadmanshandle/internal/adapters"
	"github.com/fireflycons/deadmanshandle/internal/cli"
)

// Compile-time checks that the adapters have what the CLI needs
var (
	_ cli.ConfigReadWriter = (*adapters.SSMConfigStore)(nil)
	_ cli.StateEditor      = (*adapters.DynamoDBStateStore)(nil)
	_ cli.DocumentTransfer = (*adapters.S3DocumentStore)(nil)
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
