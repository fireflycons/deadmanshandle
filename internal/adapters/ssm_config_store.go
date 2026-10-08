package adapters

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SSMConfigStore implements the ConfigStore interface using AWS Parameter Store
type SSMConfigStore struct {
	client *ssm.Client
}

// NewSSMConfigStore creates a new SSM-based config store
func NewSSMConfigStore(client *ssm.Client) *SSMConfigStore {
	return &SSMConfigStore{
		client: client,
	}
}

// GetConfig retrieves configuration from Parameter Store
func (s *SSMConfigStore) GetConfig(ctx context.Context, parameterName string) ([]byte, error) {
	output, err := s.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &parameterName,
		WithDecryption: boolPtr(true),
	})
	if err != nil {
		return nil, err
	}

	if output.Parameter == nil || output.Parameter.Value == nil {
		return nil, fmt.Errorf("parameter %s has no value", parameterName)
	}

	return []byte(*output.Parameter.Value), nil
}

func boolPtr(b bool) *bool {
	return &b
}
