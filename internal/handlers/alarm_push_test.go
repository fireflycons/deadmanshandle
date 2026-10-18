package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/fireflycons/deadmanshandle/internal/config"
	"github.com/fireflycons/deadmanshandle/internal/mocks"
)

// testAlarm is a CloudWatch alarm notification, as published to the topic
const testAlarm = `{
  "AlarmName": "deadmanshandle-scheduled-errors",
  "AlarmDescription": "The daily dead man's handle run returned an error. Check its CloudWatch logs.",
  "AWSAccountId": "123456789012",
  "NewStateValue": "ALARM",
  "NewStateReason": "Threshold Crossed: 1 datapoint [1.0 (01/06/24 12:00:00)] was greater than or equal to the threshold (1.0).",
  "StateChangeTime": "2024-06-01T12:05:00.000+0000",
  "Region": "EU (Ireland)",
  "OldStateValue": "OK",
  "Trigger": {"MetricName": "Errors", "Namespace": "AWS/Lambda"}
}`

func TestAlarmPushHandler(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		token     string
		pushErr   error
		wantPush  bool
		wantError bool
	}{
		{"alarm pushed", testAlarm, testPushToken, nil, true, false},
		{"no token", testAlarm, "", nil, false, false},
		{"push fails", testAlarm, testPushToken, errors.New("sns down"), true, true},
		{"not in ALARM", `{"AlarmName": "a", "NewStateValue": "OK"}`, testPushToken, nil, false, false},
		{"not an alarm", `not JSON`, testPushToken, nil, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pushSender := mocks.NewMockPushSender()
			pushSender.Err = tt.pushErr
			handler := NewAlarmPushHandler(mocks.NewMockStateStore(config.State{PushToken: tt.token}), pushSender)

			err := handler.Handle(context.Background(), tt.message)
			if (err != nil) != tt.wantError {
				t.Errorf("Expected error %v, got %v", tt.wantError, err)
			}
			if (len(pushSender.Pushes) == 1) != tt.wantPush {
				t.Fatalf("Expected push %v, got %+v", tt.wantPush, pushSender.Pushes)
			}
			if !tt.wantPush {
				return
			}

			push := pushSender.Pushes[0]
			wantBody := "The daily dead man's handle run returned an error. Check its CloudWatch logs.\n\n" +
				"Threshold Crossed: 1 datapoint [1.0 (01/06/24 12:00:00)] was greater than or equal to the threshold (1.0)."
			if push.Token != testPushToken || push.Kind != AlarmKind ||
				push.Title != "Deadman's Handle Alarm: deadmanshandle-scheduled-errors" || push.Body != wantBody {
				t.Errorf("Unexpected push %+v", push)
			}
		})
	}
}
