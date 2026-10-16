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
		"deployment": {"senderEmail": "noreply@example.com", "documentBucket": "", "documentKey": "document.pdf"}
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
	data, err := Encode("https://abc.example.com", `k"ey`)
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
