// Package qrconfig builds the payload of the QR code that sets up the
// Android check-in app: the API endpoint and the API key.
package qrconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// PayloadVersion is the "v" field; the app rejects versions it doesn't know
const PayloadVersion = 1

// Payload is the JSON encoded in the QR code
type Payload struct {
	Version  int    `json:"v"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"apiKey"`
}

// EndpointFromState returns the api_endpoint output of a Terraform state file
func EndpointFromState(state []byte) (string, error) {
	var s struct {
		Outputs map[string]struct {
			Value any `json:"value"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(state, &s); err != nil {
		return "", fmt.Errorf("parsing the Terraform state: %w", err)
	}
	out, ok := s.Outputs["api_endpoint"]
	if !ok {
		return "", errors.New("the Terraform state has no api_endpoint output; has it been applied?")
	}
	endpoint, ok := out.Value.(string)
	if !ok {
		return "", errors.New("the api_endpoint output is not a string")
	}
	if err := CheckEndpoint(endpoint); err != nil {
		return "", err
	}
	return endpoint, nil
}

// CheckEndpoint reports whether an endpoint is an https URL
func CheckEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("the API endpoint %q is not an https URL", endpoint)
	}
	return nil
}

// APIKeyFromConfig returns the apiKey of a config file or of the stored
// config parameter, which must be valid
func APIKeyFromConfig(cfg []byte) (string, error) {
	c, err := config.ParseConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("parsing the config: %w", err)
	}
	return c.APIKey, nil
}

// Encode returns the QR payload for an endpoint and API key
func Encode(endpoint, apiKey string) ([]byte, error) {
	return json.Marshal(Payload{Version: PayloadVersion, Endpoint: endpoint, APIKey: apiKey})
}
