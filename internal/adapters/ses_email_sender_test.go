package adapters

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
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

func TestBuildMessageParses(t *testing.T) {
	sender := &SESEmailSender{sender: "sender@example.com"}
	body := "Line one\r\nLine two"
	document := bytes.Repeat([]byte("0123456789abcdef"), 100) // long enough to need wrapping

	raw := sender.buildMessage("recipient@example.com", "The subject", body, map[string][]byte{"will.pdf": document})

	// RFC 5322: CRLF line endings, lines at most 998 characters
	for i, line := range strings.Split(strings.TrimSuffix(raw, "\r\n"), "\r\n") {
		if strings.Contains(line, "\n") {
			t.Fatalf("Line %d has a bare LF", i+1)
		}
		if len(line) > 998 {
			t.Fatalf("Line %d is %d characters long", i+1, len(line))
		}
	}

	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	for header, want := range map[string]string{
		"From":         "sender@example.com",
		"To":           "recipient@example.com",
		"Subject":      "The subject",
		"Mime-Version": "1.0",
	} {
		if got := msg.Header.Get(header); got != want {
			t.Errorf("Header %s: expected %q, got %q", header, want, got)
		}
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("Expected multipart/mixed, got %q (%v)", mediaType, err)
	}

	parts := multipart.NewReader(msg.Body, params["boundary"])

	text, err := parts.NextPart()
	if err != nil {
		t.Fatalf("Reading text part failed: %v", err)
	}
	if ct := text.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Text part: unexpected Content-Type %q", ct)
	}
	textBody, _ := io.ReadAll(text)
	if got := strings.TrimSuffix(string(textBody), "\r\n"); got != body {
		t.Errorf("Text part: expected %q, got %q", body, got)
	}

	attachment, err := parts.NextPart()
	if err != nil {
		t.Fatalf("Reading attachment part failed: %v", err)
	}
	if got := attachment.FileName(); got != "will.pdf" {
		t.Errorf("Attachment: expected filename will.pdf, got %q", got)
	}
	if enc := attachment.Header.Get("Content-Transfer-Encoding"); enc != "base64" {
		t.Fatalf("Attachment: expected base64 encoding, got %q", enc)
	}
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, attachment))
	if err != nil {
		t.Fatalf("Attachment: decoding failed: %v", err)
	}
	if !bytes.Equal(decoded, document) {
		t.Errorf("Attachment: decoded %d bytes, which do not match the %d-byte document", len(decoded), len(document))
	}

	if _, err := parts.NextPart(); err != io.EOF {
		t.Errorf("Expected exactly two parts, got another (err %v)", err)
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
