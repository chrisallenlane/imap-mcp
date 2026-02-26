package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// messageCopier is a narrow interface for copying messages
// between mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type messageCopier interface {
	CopyMessages(
		account, mailbox string,
		uids []imap.UID,
		destMailbox string,
	) error
}

// CopyMessages is an MCP tool that copies messages from one
// mailbox to another.
type CopyMessages struct {
	copier messageCopier
}

// NewCopyMessages creates a new CopyMessages tool.
func NewCopyMessages(
	copier messageCopier,
) *CopyMessages {
	return &CopyMessages{copier: copier}
}

// Description returns a description of what the tool does.
func (t *CopyMessages) Description() string {
	return "Copy messages from one mailbox to another"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *CopyMessages) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from " +
					"config",
			},
			"mailbox": map[string]interface{}{
				"type": "string",
				"description": "Source mailbox name " +
					"(e.g., 'INBOX')",
			},
			"uids": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "integer",
				},
				"description": "Message UIDs " +
					"to copy",
			},
			"destination": map[string]interface{}{
				"type": "string",
				"description": "Destination mailbox " +
					"name",
			},
		},
		"required": []string{
			"account", "mailbox", "uids",
			"destination",
		},
	}
}

// Execute copies messages from one mailbox to another.
func (t *CopyMessages) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account     string   `json:"account"`
		Mailbox     string   `json:"mailbox"`
		UIDs        []uint32 `json:"uids"`
		Destination string   `json:"destination"`
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
	if params.Mailbox == "" {
		return "", fmt.Errorf("mailbox is required")
	}
	if len(params.UIDs) == 0 {
		return "", fmt.Errorf("uids must not be empty")
	}
	if params.Destination == "" {
		return "", fmt.Errorf(
			"destination is required",
		)
	}
	if params.Destination == params.Mailbox {
		return "", fmt.Errorf(
			"destination must differ from " +
				"source mailbox",
		)
	}

	uids := make([]imap.UID, len(params.UIDs))
	for i, u := range params.UIDs {
		uids[i] = imap.UID(u)
	}

	if err := t.copier.CopyMessages(
		params.Account,
		params.Mailbox,
		uids,
		params.Destination,
	); err != nil {
		return "", fmt.Errorf(
			"failed to copy messages: %w",
			err,
		)
	}

	return formatCopyResult(
		params.Account,
		params.Mailbox,
		params.UIDs,
		params.Destination,
	), nil
}

// formatCopyResult builds a human-readable confirmation of
// a copy operation.
func formatCopyResult(
	account, mailbox string,
	uids []uint32,
	destination string,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Copied %d message(s) from %s/%s to %s.\n",
		len(uids),
		account,
		mailbox,
		destination,
	)

	uidStrs := make([]string, len(uids))
	for i, u := range uids {
		uidStrs[i] = fmt.Sprintf("%d", u)
	}
	fmt.Fprintf(
		&b,
		"  UIDs: %s\n",
		strings.Join(uidStrs, ", "),
	)

	return b.String()
}
