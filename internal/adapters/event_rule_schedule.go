package adapters

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
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

// Interval reads the rule's rate and state
func (s *EventRuleSchedule) Interval(ctx context.Context) (int, bool, error) {
	rule, err := s.client.DescribeRule(ctx, &eventbridge.DescribeRuleInput{Name: &s.rule})
	if err != nil {
		return 0, false, err
	}
	minutes, err := parseRate(rule.ScheduleExpression)
	if err != nil {
		return 0, false, fmt.Errorf("rule %s: %w", s.rule, err)
	}
	return minutes, rule.State == types.RuleStateEnabled, nil
}

// SetInterval changes the rule's rate. PutRule replaces the whole rule, so
// its state and description are passed back as read.
func (s *EventRuleSchedule) SetInterval(ctx context.Context, minutes int) error {
	rule, err := s.client.DescribeRule(ctx, &eventbridge.DescribeRuleInput{Name: &s.rule})
	if err != nil {
		return err
	}
	expression := rateExpression(minutes)
	_, err = s.client.PutRule(ctx, &eventbridge.PutRuleInput{
		Name:               &s.rule,
		ScheduleExpression: &expression,
		State:              rule.State,
		Description:        rule.Description,
	})
	return err
}

// rateExpression is the schedule expression for a check every minutes
// minutes; EventBridge wants the singular for one
func rateExpression(minutes int) string {
	if minutes == 1 {
		return "rate(1 minute)"
	}
	return "rate(" + strconv.Itoa(minutes) + " minutes)"
}

var ratePattern = regexp.MustCompile(`^rate\((\d+) minutes?\)$`)

// parseRate returns the minutes of a rate expression in minutes
func parseRate(expression *string) (int, error) {
	if expression == nil {
		return 0, fmt.Errorf("not a scheduled rule")
	}
	match := ratePattern.FindStringSubmatch(*expression)
	if match == nil {
		return 0, fmt.Errorf("schedule %q is not a rate in minutes", *expression)
	}
	return strconv.Atoi(match[1])
}
