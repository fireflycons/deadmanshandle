package config

import (
	"os"
	"strings"
	"testing"
)

const validJSON = `{
	"owner": "owner@example.com",
	"recipients": ["a@example.com", "b@example.com"],
	"resetDays": 30,
	"warnDays": 7,
	"apiKey": "key",
	"documentKey": "docs/will.pdf"
}`

func TestParseConfigValid(t *testing.T) {
	cfg, err := ParseConfig([]byte(validJSON))
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if cfg.Owner != "owner@example.com" || len(cfg.Recipients) != 2 || cfg.ResetDays != 30 || cfg.WarnDays != 7 || cfg.APIKey != "key" || cfg.DocumentKey != "docs/will.pdf" {
		t.Errorf("Unexpected config: %+v", cfg)
	}
}

func TestParseConfigUnusualAddress(t *testing.T) {
	data := strings.Replace(validJSON, `"b@example.com"`, `"first.o'brien+tag@mail.example-site.co.uk"`, 1)
	if _, err := ParseConfig([]byte(data)); err != nil {
		t.Errorf("Expected the address accepted, got %v", err)
	}
}

// The seed file's preconditions must accept no address the Lambdas reject
func TestEmailAddressPatternMatchesTerraform(t *testing.T) {
	tf, err := os.ReadFile("../../terraform/parameter_store.tf")
	if err != nil {
		t.Fatal(err)
	}
	want := `email_address_pattern = "` + emailAddressPattern.String() + `"`
	if !strings.Contains(string(tf), want) {
		t.Errorf("terraform/parameter_store.tf does not contain %s", want)
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
		{"owner without @", [2]string{`"owner@example.com"`, `"owner.example.com"`}, `owner "owner.example.com" is not an email address`},
		{"owner with a display name", [2]string{`"owner@example.com"`, `"Owner <owner@example.com>"`}, "owner"},
		{"owner with spaces", [2]string{`"owner@example.com"`, `" owner@example.com"`}, "not an email address"},
		{"recipient domain without a dot", [2]string{`"b@example.com"`, `"b@example"`}, `recipient "b@example" is not an email address`},
		{"recipient with two @", [2]string{`"b@example.com"`, `"b@@example.com"`}, "not an email address"},
		{"owner with a display name and no space", [2]string{`"owner@example.com"`, `"Owner<owner@example.com>"`}, "not an email address"},
		{"recipient with a tab", [2]string{`"b@example.com"`, `"b@example.com\t"`}, "not an email address"},
		{"recipient with a comma", [2]string{`"b@example.com"`, `"b@example.com,c@example.com"`}, "not an email address"},
		{"recipient with a double dot", [2]string{`"b@example.com"`, `"b..c@example.com"`}, "not an email address"},
		{"zero resetDays", [2]string{`"resetDays": 30`, `"resetDays": 0`}, "resetDays must be greater than 0"},
		{"negative warnDays", [2]string{`"warnDays": 7`, `"warnDays": -1`}, "warnDays must be"},
		{"warnDays equals resetDays", [2]string{`"warnDays": 7`, `"warnDays": 30`}, "warnDays must be"},
		{"empty apiKey", [2]string{`"key"`, `""`}, "apiKey is empty"},
		{"blank documentKey", [2]string{`"docs/will.pdf"`, `" "`}, "documentKey is empty"},
		{"missing documentKey", [2]string{`,
	"documentKey": "docs/will.pdf"`, ``}, "documentKey is empty"},
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
	for _, want := range []string{"owner", "recipients", "resetDays", "apiKey", "documentKey"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Expected error to mention %s, got %q", want, err)
		}
	}
}
