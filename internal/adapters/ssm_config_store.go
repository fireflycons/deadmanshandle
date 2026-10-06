package adapters

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
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
		return nil, nil
	}

	return []byte(*output.Parameter.Value), nil
}

// SetConfig stores configuration in Parameter Store
func (s *SSMConfigStore) SetConfig(ctx context.Context, parameterName string, data []byte) error {
	_, err := s.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      &parameterName,
		Value:     stringPtr(string(data)),
		Overwrite: boolPtr(true),
		Type:      types.ParameterTypeSecureString,
	})
	return err
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
