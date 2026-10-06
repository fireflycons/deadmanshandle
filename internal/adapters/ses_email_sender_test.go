package adapters

import (
	"strings"
	"testing"
)

func TestBuildMessageAddressesRecipient(t *testing.T) {
	sender := &SESEmailSender{sender: "sender@example.com"}

	message := sender.buildMessage("recipient@example.com", "Subject", "Body", nil)

	if !strings.Contains(message, "From: sender@example.com\r\n") {
		t.Errorf("Expected From header, got:\n%s", message)
	}
	if !strings.Contains(message, "To: recipient@example.com\r\n") {
		t.Errorf("Expected To header, got:\n%s", message)
	}
}

func TestBuildMessageAttachmentHeaders(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
		disposition string
	}{
		{"will.pdf", `Content-Type: application/pdf; name=will.pdf`, `Content-Disposition: attachment; filename=will.pdf`},
		{"my will.unknownext", `Content-Type: application/octet-stream; name="my will.unknownext"`, `Content-Disposition: attachment; filename="my will.unknownext"`},
		{"noextension", `Content-Type: application/octet-stream; name=noextension`, `Content-Disposition: attachment; filename=noextension`},
		{"testamenté.pdf", `Content-Type: application/pdf; name*=utf-8''testament%C3%A9.pdf`, `Content-Disposition: attachment; filename*=utf-8''testament%C3%A9.pdf`},
	}

	sender := &SESEmailSender{sender: "sender@example.com"}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			message := sender.buildMessage("recipient@example.com", "Subject", "Body", map[string][]byte{tt.filename: []byte("data")})

			for _, want := range []string{tt.contentType + "\r\n", tt.disposition + "\r\n"} {
				if !strings.Contains(message, want) {
					t.Errorf("Expected %q, got:\n%s", want, message)
				}
			}
		})
	}
}
