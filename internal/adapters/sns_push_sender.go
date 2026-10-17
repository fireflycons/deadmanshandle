package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// pushMessageVersion is the "v" field of the data the app receives
const pushMessageVersion = "1"

// ErrPushTokenInvalid is returned when FCM has rejected the token, so SNS
// has disabled its endpoint. The app registers a new token when it next
// checks in.
var ErrPushTokenInvalid = errors.New("push token no longer valid; check in from the app to register it again")

// SNSPushSender implements PushSender with SNS mobile push, which forwards to
// Firebase Cloud Messaging
type SNSPushSender struct {
	client         *sns.Client
	applicationARN string // The SNS platform application for FCM
}

// NewSNSPushSender creates a new SNS-based push sender
func NewSNSPushSender(client *sns.Client, applicationARN string) *SNSPushSender {
	return &SNSPushSender{
		client:         client,
		applicationARN: applicationARN,
	}
}

// Push sends a data message to the app. The endpoint is created on demand:
// CreatePlatformEndpoint returns the existing one for a token it already
// knows, so the token is all that needs to be stored.
func (p *SNSPushSender) Push(ctx context.Context, token, kind, title, body string) error {
	endpoint, err := p.client.CreatePlatformEndpoint(ctx, &sns.CreatePlatformEndpointInput{
		PlatformApplicationArn: &p.applicationARN,
		Token:                  &token,
	})
	if err != nil {
		return fmt.Errorf("creating the push endpoint: %w", err)
	}

	attributes, err := p.client.GetEndpointAttributes(ctx, &sns.GetEndpointAttributesInput{
		EndpointArn: endpoint.EndpointArn,
	})
	if err != nil {
		return fmt.Errorf("reading the push endpoint: %w", err)
	}
	if attributes.Attributes["Enabled"] == "false" {
		return ErrPushTokenInvalid
	}

	message, err := pushMessage(kind, title, body)
	if err != nil {
		return err
	}
	_, err = p.client.Publish(ctx, &sns.PublishInput{
		TargetArn:        endpoint.EndpointArn,
		MessageStructure: aws.String("json"),
		Message:          &message,
	})
	if err != nil {
		return fmt.Errorf("publishing the push: %w", err)
	}
	return nil
}

// pushMessage builds the SNS message: an FCM HTTP v1 data-only message at
// high priority, so the app chooses how to show it, even while closed. SNS
// takes the FCM message as a JSON string under the "GCM" key.
func pushMessage(kind, title, body string) (string, error) {
	fcm, err := json.Marshal(map[string]any{
		"fcmV1Message": map[string]any{
			"message": map[string]any{
				"android": map[string]string{"priority": "high"},
				"data": map[string]string{
					"v":     pushMessageVersion,
					"kind":  kind,
					"title": title,
					"body":  body,
				},
			},
		},
	})
	if err != nil {
		return "", err
	}
	message, err := json.Marshal(map[string]string{"GCM": string(fcm)})
	return string(message), err
}
