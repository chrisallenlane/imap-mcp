package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	imap "github.com/emersion/go-imap/v2"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// emailSender is a narrow interface for sending email via SMTP.
// *smtp.Manager satisfies this implicitly.
type emailSender interface {
	Send(
		account, from string,
		to []string,
		msg io.Reader,
	) error
	Config() *config.Config
}

// sentSaver is a narrow interface for saving sent messages
// via IMAP APPEND.
// *imapmanager.ConnectionManager satisfies this implicitly.
type sentSaver interface {
	FindSentMailbox(account string) (string, error)
	AppendMessage(
		account, mailbox string,
		msg []byte,
		flags []imap.Flag,
	) error
}

// SendMessage is an MCP tool that sends an email via SMTP.
type SendMessage struct {
	sender emailSender
	saver  sentSaver
}

// NewSendMessage creates a new SendMessage tool.
func NewSendMessage(
	sender emailSender,
	saver sentSaver,
) *SendMessage {
	return &SendMessage{sender: sender, saver: saver}
}

// Description returns a description of what the tool does.
func (t *SendMessage) Description() string {
	return "Send an email message via SMTP"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *SendMessage) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from " +
					"config (e.g., 'gmail')",
			},
			"to": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Recipient email " +
					"addresses",
			},
			"cc": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "CC recipient " +
					"addresses",
			},
			"bcc": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "BCC recipient " +
					"addresses",
			},
			"subject": map[string]interface{}{
				"type":        "string",
				"description": "Message subject",
			},
			"body": map[string]interface{}{
				"type": "string",
				"description": "Plain text " +
					"message body",
			},
			"attachments": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "File paths to " +
					"attach",
			},
		},
		"required": []string{
			"account",
			"to",
			"subject",
			"body",
		},
	}
}

// Execute sends an email message.
func (t *SendMessage) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account     string   `json:"account"`
		To          []string `json:"to"`
		CC          []string `json:"cc"`
		BCC         []string `json:"bcc"`
		Subject     string   `json:"subject"`
		Body        string   `json:"body"`
		Attachments []string `json:"attachments"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf(
			"failed to parse arguments: %w",
			err,
		)
	}

	if params.Account == "" {
		return "", fmt.Errorf("account is required")
	}
	if len(params.To) == 0 {
		return "", fmt.Errorf(
			"at least one recipient (to) is required",
		)
	}
	if params.Subject == "" {
		return "", fmt.Errorf("subject is required")
	}
	if params.Body == "" {
		return "", fmt.Errorf("body is required")
	}

	cfg := t.sender.Config()
	acct, ok := cfg.Accounts[params.Account]
	if !ok {
		return "", fmt.Errorf(
			"unknown account: %q",
			params.Account,
		)
	}

	if !acct.SMTPEnabled {
		return "", fmt.Errorf(
			"SMTP is not enabled for account %q. "+
				"Set smtp_enabled = true in your "+
				"config file.",
			params.Account,
		)
	}

	from := acct.SMTPFrom
	if from == "" {
		from = acct.Username
	}

	msgBytes, err := composeMessage(composeParams{
		From:        from,
		To:          params.To,
		CC:          params.CC,
		BCC:         params.BCC,
		Subject:     params.Subject,
		Body:        params.Body,
		Attachments: params.Attachments,
	})
	if err != nil {
		return "", fmt.Errorf(
			"failed to compose message: %w",
			err,
		)
	}

	// Collect all envelope recipients.
	allRecipients := make(
		[]string,
		0,
		len(params.To)+len(params.CC)+len(params.BCC),
	)
	allRecipients = append(allRecipients, params.To...)
	allRecipients = append(allRecipients, params.CC...)
	allRecipients = append(allRecipients, params.BCC...)

	if err := t.sender.Send(
		params.Account,
		from,
		allRecipients,
		bytes.NewReader(msgBytes),
	); err != nil {
		return "", fmt.Errorf(
			"failed to send message: %w",
			err,
		)
	}

	// Save to Sent folder if configured.
	savedToSent := false
	if acct.SaveSent {
		sentMailbox, err := t.saver.FindSentMailbox(
			params.Account,
		)
		if err == nil {
			if appendErr := t.saver.AppendMessage(
				params.Account,
				sentMailbox,
				msgBytes,
				[]imap.Flag{imap.FlagSeen},
			); appendErr == nil {
				savedToSent = true
			}
		}
	}

	return formatSendResult(
		params.To,
		params.CC,
		params.BCC,
		params.Subject,
		params.Attachments,
		savedToSent,
	), nil
}

// formatSendResult formats the send confirmation message.
func formatSendResult(
	to, cc, bcc []string,
	subject string,
	attachments []string,
	savedToSent bool,
) string {
	var b strings.Builder
	b.WriteString("Message sent successfully.\n")

	fmt.Fprintf(
		&b,
		"\n  To:      %s\n",
		strings.Join(to, ", "),
	)
	if len(cc) > 0 {
		fmt.Fprintf(
			&b,
			"  CC:      %s\n",
			strings.Join(cc, ", "),
		)
	}
	if len(bcc) > 0 {
		fmt.Fprintf(
			&b,
			"  BCC:     %s\n",
			strings.Join(bcc, ", "),
		)
	}
	fmt.Fprintf(&b, "  Subject: %s\n", subject)

	if len(attachments) > 0 {
		fmt.Fprintf(
			&b,
			"  Attachments: %d\n",
			len(attachments),
		)
	}

	if savedToSent {
		b.WriteString("  Saved to Sent folder.\n")
	}

	return b.String()
}
