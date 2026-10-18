package adapters

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

// EventRuleSchedule implements the CheckSchedule interface with an
// EventBridge scheduled rule, which Terraform creates disabled
type EventRuleSchedule struct {
	client *eventbridge.Client
	rule   string
}

// NewEventRuleSchedule creates a schedule for the named rule
func NewEventRuleSchedule(client *eventbridge.Client, rule string) *EventRuleSchedule {
	return &EventRuleSchedule{
		client: client,
		rule:   rule,
	}
}

// Enable starts the periodic check
func (s *EventRuleSchedule) Enable(ctx context.Context) error {
	_, err := s.client.EnableRule(ctx, &eventbridge.EnableRuleInput{Name: &s.rule})
	return err
}

// Disable stops the periodic check
func (s *EventRuleSchedule) Disable(ctx context.Context) error {
	_, err := s.client.DisableRule(ctx, &eventbridge.DisableRuleInput{Name: &s.rule})
	return err
}
