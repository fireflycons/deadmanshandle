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
