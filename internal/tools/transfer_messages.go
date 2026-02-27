package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// transferFunc performs a message transfer operation
// (move or copy).
type transferFunc func(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error

// transferTool is the shared implementation for move and
// copy tools.
type transferTool struct {
	verb     string // "move" or "copy"
	pastVerb string // "Moved" or "Copied"
	fn       transferFunc
}

func newTransferTool(
	verb, pastVerb string,
	fn transferFunc,
) *transferTool {
	return &transferTool{
		verb:     verb,
		pastVerb: pastVerb,
		fn:       fn,
	}
}

// Description returns a description of what the tool does.
func (t *transferTool) Description() string {
	return strings.ToUpper(t.verb[:1]) + t.verb[1:] +
		" messages from one mailbox to another"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *transferTool) InputSchema() map[string]interface{} {
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
					"to " + t.verb,
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

// Execute transfers messages from one mailbox to another.
func (t *transferTool) Execute(
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

	uids := toIMAPUIDs(params.UIDs)

	if err := t.fn(
		params.Account,
		params.Mailbox,
		uids,
		params.Destination,
	); err != nil {
		return "", fmt.Errorf(
			"failed to %s messages: %w",
			t.verb,
			err,
		)
	}

	return formatTransferResult(
		t.pastVerb,
		params.Account,
		params.Mailbox,
		params.UIDs,
		params.Destination,
	), nil
}

// formatTransferResult builds a human-readable confirmation
// of a transfer operation.
func formatTransferResult(
	pastVerb, account, mailbox string,
	uids []uint32,
	destination string,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"%s %d message(s) from %s/%s to %s.\n",
		pastVerb,
		len(uids),
		account,
		mailbox,
		destination,
	)

	fmt.Fprintf(&b, "  UIDs: %s\n", formatUIDs(uids))

	return b.String()
}
