package qrconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEndpointFromState(t *testing.T) {
	tests := []struct {
		name    string
		state   string
		want    string
		wantErr string
	}{
		{
			name:  "valid",
			state: `{"version": 4, "outputs": {"api_endpoint": {"value": "https://abc.execute-api.eu-west-1.amazonaws.com", "type": "string"}}}`,
			want:  "https://abc.execute-api.eu-west-1.amazonaws.com",
		},
		{"no outputs", `{"version": 4}`, "", "no api_endpoint output"},
		{"other outputs only", `{"outputs": {"state_table_name": {"value": "t"}}}`, "", "no api_endpoint output"},
		{"not a string", `{"outputs": {"api_endpoint": {"value": 1}}}`, "", "not a string"},
		{"http", `{"outputs": {"api_endpoint": {"value": "http://abc.example.com"}}}`, "", "not an https URL"},
		{"not a URL", `{"outputs": {"api_endpoint": {"value": "abc.example.com"}}}`, "", "not an https URL"},
		{"no host", `{"outputs": {"api_endpoint": {"value": "https://"}}}`, "", "not an https URL"},
		{"bad JSON", `{`, "", "parsing the Terraform state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EndpointFromState([]byte(tt.state))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("endpoint = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAPIKeyFromConfig(t *testing.T) {
	// The seed file's extra fields are ignored
	valid := `{
		"owner": "owner@example.com",
		"recipients": ["a@example.com"],
		"resetDays": 30,
		"warnDays": 7,
		"timeout": "2030-01-01T00:00:00Z",
		"apiKey": "secret",
		"documentKey": "document.pdf",
		"deployment": {"senderEmail": "noreply@example.com", "documentBucket": ""}
	}`
	key, err := APIKeyFromConfig([]byte(valid))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "secret" {
		t.Errorf("apiKey = %q, want %q", key, "secret")
	}

	if _, err := APIKeyFromConfig([]byte(strings.Replace(valid, `"secret"`, `""`, 1))); err == nil || !strings.Contains(err.Error(), "apiKey is empty") {
		t.Errorf("error = %v, want one containing %q", err, "apiKey is empty")
	}
}

func TestEncode(t *testing.T) {
	data, err := Encode("https://abc.example.com", `k"ey`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `{"v":1,"endpoint":"https://abc.example.com","apiKey":"k\"ey"}`; string(data) != want {
		t.Errorf("payload = %s, want %s", data, want)
	}
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if p != (Payload{Version: 1, Endpoint: "https://abc.example.com", APIKey: `k"ey`}) {
		t.Errorf("round trip = %+v", p)
	}
}

func TestEncodeWithPush(t *testing.T) {
	push := &Push{ProjectID: "proj", AppID: "1:42:android:ab", APIKey: "AIza", SenderID: "42"}
	data, err := Encode("https://abc.example.com", "key", push)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `{"v":1,"endpoint":"https://abc.example.com","apiKey":"key",` +
		`"push":{"projectId":"proj","appId":"1:42:android:ab","apiKey":"AIza","senderId":"42"}}`
	if string(data) != want {
		t.Errorf("payload = %s, want %s", data, want)
	}
}

// googleServices is a google-services.json with another app before this one
const googleServices = `{
  "project_info": {"project_number": "123456789012", "project_id": "dmh-test", "storage_bucket": "dmh-test.appspot.com"},
  "client": [
    {
      "client_info": {"mobilesdk_app_id": "1:123456789012:android:other", "android_client_info": {"package_name": "com.example.other"}},
      "api_key": [{"current_key": "AIzaOther"}]
    },
    {
      "client_info": {"mobilesdk_app_id": "1:123456789012:android:abc123", "android_client_info": {"package_name": "com.fireflycons.deadmanshandle"}},
      "oauth_client": [],
      "api_key": [{"current_key": "AIzaThisApp"}],
      "services": {"appinvite_service": {"other_platform_oauth_client": []}}
    }
  ],
  "configuration_version": "1"
}`

func TestPushFromGoogleServices(t *testing.T) {
	push, err := PushFromGoogleServices([]byte(googleServices))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := Push{ProjectID: "dmh-test", AppID: "1:123456789012:android:abc123", APIKey: "AIzaThisApp", SenderID: "123456789012"}
	if *push != want {
		t.Errorf("push = %+v, want %+v", *push, want)
	}

	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{"bad JSON", `{`, "parsing google-services.json"},
		{"no such app", strings.Replace(googleServices, "com.fireflycons.deadmanshandle", "com.example.x", 1), "has no Android app"},
		{"no API key", strings.Replace(googleServices, `"AIzaThisApp"`, `""`, 1), "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := PushFromGoogleServices([]byte(tt.data)); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}
