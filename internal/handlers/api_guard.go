package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/domain"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// APIGuardHandler closes the check-in API when its rejected-requests alarm
// fires, so that a flood stops at API Gateway, and reopens it once the alarm
// has cleared. It checks on a schedule, which runs only while it has the API
// closed: SNS notifies only on a change of state, so reopening into a flood
// that is still going on would not close the API again.
type APIGuardHandler struct {
	configStore ports.ConfigStore // Holds the owner's address
	stateStore  ports.StateStore  // Holds the push token
	emailSender ports.EmailSender
	pushSender  ports.PushSender // nil when push is not set up
	gate        ports.APIGate
	schedule    ports.CheckSchedule
	alarms      ports.AlarmStates
	service     *domain.DeadmansHandleService
	paramName   string
	alarmName   string         // the alarm that closes the API
	open        ports.Throttle // the throttle Terraform set
}

// NewAPIGuardHandler creates a new API guard handler
func NewAPIGuardHandler(
	configStore ports.ConfigStore,
	stateStore ports.StateStore,
	emailSender ports.EmailSender,
	pushSender ports.PushSender,
	gate ports.APIGate,
	schedule ports.CheckSchedule,
	alarms ports.AlarmStates,
	service *domain.DeadmansHandleService,
	parameterName string,
	alarmName string,
	open ports.Throttle,
) *APIGuardHandler {
	return &APIGuardHandler{
		configStore: configStore,
		stateStore:  stateStore,
		emailSender: emailSender,
		pushSender:  pushSender,
		gate:        gate,
		schedule:    schedule,
		alarms:      alarms,
		service:     service,
		paramName:   parameterName,
		alarmName:   alarmName,
		open:        open,
	}
}

// HandleAlarm closes the API if message, the body of an SNS notification
// from the alarms topic, is the rejected-requests alarm entering ALARM. An
// API that is already closed, by the guard or by hand, is left alone.
func (h *APIGuardHandler) HandleAlarm(ctx context.Context, message string) error {
	var alarm cloudWatchAlarm
	if err := json.Unmarshal([]byte(message), &alarm); err != nil {
		return fmt.Errorf("decoding the alarm: %w", err)
	}
	if alarm.AlarmName != h.alarmName || alarm.NewStateValue != "ALARM" {
		slog.Info("Alarm ignored", "alarm", alarm.AlarmName, "state", alarm.NewStateValue)
		return nil
	}

	throttle, err := h.gate.Throttle(ctx)
	if err != nil {
		return fmt.Errorf("reading the API's throttle: %w", err)
	}
	if throttle.Closed() {
		slog.Info("API already closed")
		return nil
	}

	// The check first: if closing then failed, the check finds the API open
	// and stops. The other way round, a failure to start the check would
	// leave the API closed, and the retry would see it closed and stop.
	if err := h.schedule.Enable(ctx); err != nil {
		return fmt.Errorf("starting the check: %w", err)
	}
	if err := h.gate.SetThrottle(ctx, ports.Throttle{}); err != nil {
		return fmt.Errorf("closing the API: %w", err)
	}
	slog.Warn("API closed", "alarm", alarm.AlarmName)

	minutes, _, err := h.schedule.Interval(ctx)
	if err != nil {
		slog.Warn("Reading the check interval failed", "error", err)
		minutes = 0
	}
	return h.notify(ctx, func(cfg *config.Config) domain.EmailAction {
		return h.service.APIClosedEmail(cfg, minutes)
	})
}

// HandleCheck reopens the API once the alarm has cleared, and stops the
// check. If the API was reopened another way (dmh api enable, or a
// terraform apply), it just stops the check.
func (h *APIGuardHandler) HandleCheck(ctx context.Context) error {
	throttle, err := h.gate.Throttle(ctx)
	if err != nil {
		return fmt.Errorf("reading the API's throttle: %w", err)
	}
	if !throttle.Closed() {
		slog.Info("API already open; stopping the check")
		return h.stopCheck(ctx)
	}

	flooded, err := h.alarms.InAlarm(ctx, h.alarmName)
	if err != nil {
		return fmt.Errorf("reading alarm %s: %w", h.alarmName, err)
	}
	if flooded {
		slog.Info("API kept closed: the alarm is still in ALARM", "alarm", h.alarmName)
		return nil
	}

	if err := h.gate.SetThrottle(ctx, h.open); err != nil {
		return fmt.Errorf("reopening the API: %w", err)
	}
	slog.Info("API reopened")
	if err := h.stopCheck(ctx); err != nil {
		return err
	}
	return h.notify(ctx, h.service.APIReopenedEmail)
}

func (h *APIGuardHandler) stopCheck(ctx context.Context) error {
	if err := h.schedule.Disable(ctx); err != nil {
		return fmt.Errorf("stopping the check: %w", err)
	}
	return nil
}

// notify emails the owner and pushes the notice, best-effort
func (h *APIGuardHandler) notify(ctx context.Context, notice func(*config.Config) domain.EmailAction) error {
	configData, err := h.configStore.GetConfig(ctx, h.paramName)
	if err != nil {
		return fmt.Errorf("reading the config: %w", err)
	}
	cfg, err := config.ParseConfig(configData)
	if err != nil {
		return err
	}
	email := notice(cfg)

	if state, err := h.stateStore.GetState(ctx); err != nil {
		slog.Warn("Push skipped: reading the push token failed", "error", err)
	} else {
		pushToOwner(ctx, h.pushSender, state.PushToken, email)
	}
	if err := h.emailSender.SendEmail(ctx, email.To, email.Subject, email.Body, nil); err != nil {
		return fmt.Errorf("sending %s notice to %s: %w", email.Kind, email.To, err)
	}
	slog.Info("Email sent", "to", email.To, "subject", email.Subject)
	return nil
}
