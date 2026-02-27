package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

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

	acct, from, err := resolveSMTPAccount(
		t.sender.Config(), params.Account,
	)
	if err != nil {
		return "", err
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

	allRecipients := collectRecipients(
		params.To, params.CC, params.BCC,
	)

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
	savedToSent := acct.SaveSent &&
		trySaveToSent(t.saver, params.Account, msgBytes)

	return formatSendConfirmation(sendConfirmation{
		Title:       "Message",
		To:          params.To,
		CC:          params.CC,
		BCC:         params.BCC,
		Subject:     params.Subject,
		Attachments: len(params.Attachments),
		SavedToSent: savedToSent,
	}), nil
}
