package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// messageMover is a narrow interface for moving messages
// between mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type messageMover interface {
	MoveMessages(
		account, mailbox string,
		uids []imap.UID,
		destMailbox string,
	) error
}

// MoveMessages is an MCP tool that moves messages from one
// mailbox to another.
type MoveMessages struct {
	mover messageMover
}

// NewMoveMessages creates a new MoveMessages tool.
func NewMoveMessages(
	mover messageMover,
) *MoveMessages {
	return &MoveMessages{mover: mover}
}

// Description returns a description of what the tool does.
func (t *MoveMessages) Description() string {
	return "Move messages from one mailbox to another"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *MoveMessages) InputSchema() map[string]interface{} {
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
					"to move",
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

// Execute moves messages from one mailbox to another.
func (t *MoveMessages) Execute(
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

	if err := t.mover.MoveMessages(
		params.Account,
		params.Mailbox,
		uids,
		params.Destination,
	); err != nil {
		return "", fmt.Errorf(
			"failed to move messages: %w",
			err,
		)
	}

	return formatMoveResult(
		params.Account,
		params.Mailbox,
		params.UIDs,
		params.Destination,
	), nil
}

// formatMoveResult builds a human-readable confirmation of
// a move operation.
func formatMoveResult(
	account, mailbox string,
	uids []uint32,
	destination string,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Moved %d message(s) from %s/%s to %s.\n",
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
