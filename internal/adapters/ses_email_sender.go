package adapters

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/fireflycons/deadmanshandle/internal/domain"
)

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

// SendBatchEmail sends multiple emails
func (s *SESEmailSender) SendBatchEmail(ctx context.Context, emails []domain.EmailAction, attachments map[string][]byte) error {
	for _, email := range emails {
		if err := s.SendEmail(ctx, email.To, email.Subject, email.Body, attachments); err != nil {
			return err
		}
	}
	return nil
}

func (s *SESEmailSender) buildMessage(to, subject, body string, attachments map[string][]byte) string {
	boundary := "===============boundary==============="
	var message strings.Builder

	fmt.Fprintf(&message, "From: %s\r\n", s.sender)
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
		fmt.Fprintf(&message, "Content-Type: application/octet-stream; name=\"%s\"\r\n", filename)
		message.WriteString("Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&message, "Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filename)
		message.WriteString(base64.StdEncoding.EncodeToString(data))
		message.WriteString("\r\n")
	}

	fmt.Fprintf(&message, "\r\n--%s--\r\n", boundary)
	return message.String()
}
