package config

import (
	"strings"
	"testing"
)

const validJSON = `{
	"owner": "owner@example.com",
	"recipients": ["a@example.com", "b@example.com"],
	"resetDays": 30,
	"warnDays": 7,
	"apiKey": "key"
}`

func TestParseConfigValid(t *testing.T) {
	cfg, err := ParseConfig([]byte(validJSON))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if cfg.Owner != "owner@example.com" || len(cfg.Recipients) != 2 || cfg.ResetDays != 30 || cfg.WarnDays != 7 || cfg.APIKey != "key" {
		t.Errorf("Unexpected config: %+v", cfg)
	}
}

func TestParseConfigInvalid(t *testing.T) {
	tests := []struct {
		name    string
		replace [2]string // applied to validJSON
		wantErr string
	}{
		{"empty owner", [2]string{`"owner@example.com"`, `" "`}, "owner is empty"},
		{"no recipients", [2]string{`["a@example.com", "b@example.com"]`, `[]`}, "recipients is empty"},
		{"missing recipients", [2]string{`"recipients": ["a@example.com", "b@example.com"],`, ``}, "recipients is empty"},
		{"blank recipient", [2]string{`"b@example.com"`, `""`}, "blank entry"},
		{"zero resetDays", [2]string{`"resetDays": 30`, `"resetDays": 0`}, "resetDays must be greater than 0"},
		{"negative warnDays", [2]string{`"warnDays": 7`, `"warnDays": -1`}, "warnDays must be"},
		{"warnDays equals resetDays", [2]string{`"warnDays": 7`, `"warnDays": 30`}, "warnDays must be"},
		{"empty apiKey", [2]string{`"key"`, `""`}, "apiKey is empty"},
		{"bad JSON", [2]string{`{`, `[`}, "invalid character"},
		{"empty data", [2]string{validJSON, ``}, "unexpected end of JSON input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := strings.Replace(validJSON, tt.replace[0], tt.replace[1], 1)
			if data == validJSON {
				t.Fatalf("replacement %q did not apply", tt.replace[0])
			}
			_, err := ParseConfig([]byte(data))
			if err == nil {
				t.Fatal("Expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %q", tt.wantErr, err)
			}
		})
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	err := (&Config{}).Validate()
	if err == nil {
		t.Fatal("Expected an error")
	}
	for _, want := range []string{"owner", "recipients", "resetDays", "apiKey"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Expected error to mention %s, got %q", want, err)
		}
	}
}
