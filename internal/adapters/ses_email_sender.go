package adapters

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// senderName is the display name on every email; mail from a bare address
// looks more like spam.
const senderName = "Deadman's Handle"

// SESEmailSender implements the EmailSender interface using AWS SES
type SESEmailSender struct {
	client *ses.Client
	sender string
}

// NewSESEmailSender creates a new SES-based email sender
func NewSESEmailSender(client *ses.Client, senderEmail string) *SESEmailSender {
	return &SESEmailSender{
		client: client,
		sender: senderEmail,
	}
}

// SendEmail sends a single email
func (s *SESEmailSender) SendEmail(ctx context.Context, to, subject, body string, attachments map[string][]byte) error {
	message := s.buildMessage(to, subject, body, attachments)

	_, err := s.client.SendRawEmail(ctx, &ses.SendRawEmailInput{
		Destinations: []string{to},
		RawMessage: &types.RawMessage{
			Data: []byte(message),
		},
	})
	return err
}

func (s *SESEmailSender) buildMessage(to, subject, body string, attachments map[string][]byte) string {
	// A new boundary for each message: a fixed one is a spam fingerprint, and
	// a document containing it would break the message
	boundary := "=_" + rand.Text()
	var message strings.Builder

	fmt.Fprintf(&message, "From: %s\r\n", (&mail.Address{Name: senderName, Address: s.sender}).String())
	fmt.Fprintf(&message, "To: %s\r\n", to)
	fmt.Fprintf(&message, "Subject: %s\r\n", subject)
	fmt.Fprintf(&message, "MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&message, "\r\n--%s\r\n", boundary)
	message.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	message.WriteString(body)
	message.WriteString("\r\n")

	for filename, data := range attachments {
		fmt.Fprintf(&message, "\r\n--%s\r\n", boundary)
		fmt.Fprintf(&message, "Content-Type: %s\r\n", mime.FormatMediaType(attachmentType(filename), map[string]string{"name": filename}))
		message.WriteString("Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&message, "Content-Disposition: %s\r\n\r\n", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
		// RFC 2045 limits base64 lines to 76 characters
		encoded := base64.StdEncoding.EncodeToString(data)
		for len(encoded) > 76 {
			message.WriteString(encoded[:76] + "\r\n")
			encoded = encoded[76:]
		}
		message.WriteString(encoded)
		message.WriteString("\r\n")
	}

	fmt.Fprintf(&message, "\r\n--%s--\r\n", boundary)
	return message.String()
}

// attachmentType guesses the MIME type from the file extension, using Go's
// built-in table plus the system's. Unknown types are sent as
// application/octet-stream.
func attachmentType(filename string) string {
	if t := mime.TypeByExtension(path.Ext(filename)); t != "" {
		return t
	}
	return "application/octet-stream"
}
