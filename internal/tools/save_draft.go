package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// configProvider is a narrow interface for accessing
// account config to resolve the From address.
// *smtp.Manager and *imapmanager.ConnectionManager satisfy
// this implicitly.
type configProvider interface {
	Config() *config.Config
}

// draftSaver is a narrow interface for saving drafts via
// IMAP APPEND.
// *imapmanager.ConnectionManager satisfies this implicitly.
type draftSaver interface {
	FindDraftsMailbox(account string) (string, error)
	AppendMessage(
		account, mailbox string,
		msg []byte,
		flags []imap.Flag,
	) error
}

// SaveDraft is an MCP tool that composes a message and saves
// it to the Drafts folder.
type SaveDraft struct {
	cfg   configProvider
	saver draftSaver
}

// NewSaveDraft creates a new SaveDraft tool.
func NewSaveDraft(
	cfg configProvider,
	saver draftSaver,
) *SaveDraft {
	return &SaveDraft{cfg: cfg, saver: saver}
}

// Description returns a description of what the tool does.
func (t *SaveDraft) Description() string {
	return "Compose a message and save it as a " +
		"draft in the Drafts folder"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *SaveDraft) InputSchema() map[string]interface{} {
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
					"addresses (optional for " +
					"early drafts)",
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
		"required": []string{"account"},
	}
}

// Execute composes a draft message and saves it to the
// Drafts folder.
func (t *SaveDraft) Execute(
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

	_, from, err := resolveSMTPAccount(
		t.cfg.Config(), params.Account,
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
			"failed to compose draft: %w",
			err,
		)
	}

	draftsMailbox, err := t.saver.FindDraftsMailbox(
		params.Account,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to find Drafts folder: %w",
			err,
		)
	}

	if err := t.saver.AppendMessage(
		params.Account,
		draftsMailbox,
		msgBytes,
		[]imap.Flag{imap.FlagDraft},
	); err != nil {
		return "", fmt.Errorf(
			"failed to save draft: %w",
			err,
		)
	}

	return formatDraftResult(
		draftsMailbox,
		params.To,
		params.Subject,
	), nil
}

// formatDraftResult formats the save confirmation message.
func formatDraftResult(
	folder string,
	to []string,
	subject string,
) string {
	var b strings.Builder
	fmt.Fprintf(
		&b,
		"Draft saved to %q.\n",
		folder,
	)

	if len(to) > 0 {
		fmt.Fprintf(
			&b,
			"\n  To:      %s\n",
			strings.Join(to, ", "),
		)
	}
	if subject != "" {
		fmt.Fprintf(&b, "  Subject: %s\n", subject)
	}

	return b.String()
}
