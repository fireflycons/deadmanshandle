// Package qrconfig builds the payload of the QR code that sets up the
// Android check-in app: the API endpoint, the API key and, for push
// notifications, the Firebase project's client settings.
package qrconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/fireflycons/deadmanshandle/internal/config"
)

// PayloadVersion is the "v" field; the app rejects versions it doesn't know.
// Push was added without a new version: older apps ignore it.
const PayloadVersion = 1

// AppPackage is the Android app's package name, as registered in Firebase
const AppPackage = "com.fireflycons.deadmanshandle"

// Payload is the JSON encoded in the QR code
type Payload struct {
	Version  int    `json:"v"`
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"apiKey"`
	Push     *Push  `json:"push,omitempty"` // nil when push is not set up
}

// Push is the Firebase project's client settings, with which the app
// registers for push notifications. They identify the project but are not
// secrets.
type Push struct {
	ProjectID string `json:"projectId"`
	AppID     string `json:"appId"`
	APIKey    string `json:"apiKey"`
	SenderID  string `json:"senderId"`
}

// PushFromGoogleServices returns the client settings for the app from a
// Firebase google-services.json file
func PushFromGoogleServices(data []byte) (*Push, error) {
	var gs struct {
		ProjectInfo struct {
			ProjectNumber string `json:"project_number"`
			ProjectID     string `json:"project_id"`
		} `json:"project_info"`
		Client []struct {
			ClientInfo struct {
				MobileSDKAppID    string `json:"mobilesdk_app_id"`
				AndroidClientInfo struct {
					PackageName string `json:"package_name"`
				} `json:"android_client_info"`
			} `json:"client_info"`
			APIKey []struct {
				CurrentKey string `json:"current_key"`
			} `json:"api_key"`
		} `json:"client"`
	}
	if err := json.Unmarshal(data, &gs); err != nil {
		return nil, fmt.Errorf("parsing google-services.json: %w", err)
	}

	for _, client := range gs.Client {
		if client.ClientInfo.AndroidClientInfo.PackageName != AppPackage {
			continue
		}
		push := &Push{
			ProjectID: gs.ProjectInfo.ProjectID,
			AppID:     client.ClientInfo.MobileSDKAppID,
			SenderID:  gs.ProjectInfo.ProjectNumber,
		}
		if len(client.APIKey) > 0 {
			push.APIKey = client.APIKey[0].CurrentKey
		}
		if push.ProjectID == "" || push.AppID == "" || push.APIKey == "" || push.SenderID == "" {
			return nil, errors.New("google-services.json is missing the project ID, project number, app ID or API key")
		}
		return push, nil
	}
	return nil, fmt.Errorf("google-services.json has no Android app %s; add it to the Firebase project", AppPackage)
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

// Encode returns the QR payload for an endpoint and API key, and the push
// settings if push is set up
func Encode(endpoint, apiKey string, push *Push) ([]byte, error) {
	return json.Marshal(Payload{Version: PayloadVersion, Endpoint: endpoint, APIKey: apiKey, Push: push})
}
