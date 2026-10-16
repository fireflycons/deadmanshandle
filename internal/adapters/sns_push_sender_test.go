package adapters

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPushMessage(t *testing.T) {
	message, err := pushMessage("warning", "Check-In Required", "Body with \"quotes\"\nand lines")
	if err != nil {
		t.Fatalf("pushMessage failed: %v", err)
	}

	// SNS takes the FCM message as a JSON string under "GCM"
	var outer map[string]string
	if err := json.Unmarshal([]byte(message), &outer); err != nil {
		t.Fatalf("SNS message is not a JSON object of strings: %v", err)
	}
	var fcm struct {
		FCMV1Message struct {
			Message struct {
				Android map[string]string `json:"android"`
				Data    map[string]string `json:"data"`
			} `json:"message"`
		} `json:"fcmV1Message"`
	}
	if err := json.Unmarshal([]byte(outer["GCM"]), &fcm); err != nil {
		t.Fatalf("GCM value is not an FCM message: %v", err)
	}

	got := fcm.FCMV1Message.Message
	if got.Android["priority"] != "high" {
		t.Errorf("Expected high priority, got %v", got.Android)
	}
	want := map[string]string{
		"v":     "1",
		"kind":  "warning",
		"title": "Check-In Required",
		"body":  "Body with \"quotes\"\nand lines",
	}
	if !reflect.DeepEqual(got.Data, want) {
		t.Errorf("Expected data %v, got %v", want, got.Data)
	}
}
