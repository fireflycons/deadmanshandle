package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// AlarmKind is the push kind of a CloudWatch alarm
const AlarmKind = "alarm"

// cloudWatchAlarm is the part of a CloudWatch alarm notification used here
type cloudWatchAlarm struct {
	AlarmName        string `json:"AlarmName"`
	AlarmDescription string `json:"AlarmDescription"`
	NewStateValue    string `json:"NewStateValue"`
	NewStateReason   string `json:"NewStateReason"`
}

// AlarmPushHandler pushes the CloudWatch alarms, which reach the owner by
// email through the alarms SNS topic, to the owner's phone as well
type AlarmPushHandler struct {
	stateStore ports.StateStore // Holds the push token
	pushSender ports.PushSender
}

// NewAlarmPushHandler creates a new alarm push handler
func NewAlarmPushHandler(stateStore ports.StateStore, pushSender ports.PushSender) *AlarmPushHandler {
	return &AlarmPushHandler{
		stateStore: stateStore,
		pushSender: pushSender,
	}
}

// Handle pushes the alarm in message, the body of the SNS notification.
// Unlike the owner notices, a failed push is returned, since pushing is all
// this does; the failure shows in the Lambda's Errors metric.
func (h *AlarmPushHandler) Handle(ctx context.Context, message string) error {
	var alarm cloudWatchAlarm
	if err := json.Unmarshal([]byte(message), &alarm); err != nil {
		return fmt.Errorf("decoding the alarm: %w", err)
	}
	// The alarms only notify on entering ALARM, but don't push a recovery as
	// if it were a problem
	if alarm.NewStateValue != "ALARM" {
		slog.Info("Alarm not pushed: not in ALARM", "alarm", alarm.AlarmName, "state", alarm.NewStateValue)
		return nil
	}

	state, err := h.stateStore.GetState(ctx)
	if err != nil {
		return fmt.Errorf("reading the push token: %w", err)
	}
	if state.PushToken == "" {
		slog.Info("Alarm not pushed: the app has not registered a push token", "alarm", alarm.AlarmName)
		return nil
	}

	title := "Deadman's Handle Alarm: " + alarm.AlarmName
	body := alarm.AlarmDescription + "\n\n" + alarm.NewStateReason
	if err := h.pushSender.Push(ctx, state.PushToken, AlarmKind, title, body); err != nil {
		return fmt.Errorf("pushing alarm %s: %w", alarm.AlarmName, err)
	}
	slog.Info("Alarm pushed", "alarm", alarm.AlarmName)
	return nil
}
