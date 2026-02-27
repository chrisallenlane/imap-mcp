package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// messageDeleter is a narrow interface for deleting messages.
// *imapmanager.ConnectionManager satisfies this implicitly.
type messageDeleter interface {
	FindTrashMailbox(account string) (string, error)
	MoveMessages(
		account, mailbox string,
		uids []imap.UID,
		destMailbox string,
	) error
	ExpungeMessages(
		account, mailbox string,
		uids []imap.UID,
	) error
}

// DeleteMessages is an MCP tool that deletes messages by
// moving them to Trash or permanently expunging them.
type DeleteMessages struct {
	deleter messageDeleter
}

// NewDeleteMessages creates a new DeleteMessages tool.
func NewDeleteMessages(
	deleter messageDeleter,
) *DeleteMessages {
	return &DeleteMessages{deleter: deleter}
}

// Description returns a description of what the tool does.
func (t *DeleteMessages) Description() string {
	return "Delete messages (move to Trash by default," +
		" or permanently expunge)"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *DeleteMessages) InputSchema() map[string]interface{} {
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
				"description": "Mailbox containing " +
					"the messages",
			},
			"uids": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "integer",
				},
				"description": "Message UIDs " +
					"to delete",
			},
			"permanent": map[string]interface{}{
				"type": "boolean",
				"description": "If true, permanently " +
					"expunge messages " +
					"(irreversible). Default: " +
					"false (move to Trash).",
			},
		},
		"required": []string{
			"account", "mailbox", "uids",
		},
	}
}

// Execute deletes messages by moving to Trash or permanently
// expunging them.
func (t *DeleteMessages) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account   string   `json:"account"`
		Mailbox   string   `json:"mailbox"`
		UIDs      []uint32 `json:"uids"`
		Permanent bool     `json:"permanent"`
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

	uids := toIMAPUIDs(params.UIDs)

	if params.Permanent {
		if err := t.deleter.ExpungeMessages(
			params.Account,
			params.Mailbox,
			uids,
		); err != nil {
			return "", fmt.Errorf(
				"failed to expunge messages: %w",
				err,
			)
		}

		return formatPermanentDeleteResult(
			params.Account,
			params.Mailbox,
			params.UIDs,
		), nil
	}

	trashMailbox, err := t.deleter.FindTrashMailbox(
		params.Account,
	)
	if err != nil {
		return "", fmt.Errorf(
			"no trash mailbox found for account %q;"+
				" use permanent: true to"+
				" permanently delete",
			params.Account,
		)
	}

	if trashMailbox == params.Mailbox {
		return "", fmt.Errorf(
			"messages are already in Trash (%s);"+
				" use permanent: true to"+
				" permanently remove them",
			params.Mailbox,
		)
	}

	if err := t.deleter.MoveMessages(
		params.Account,
		params.Mailbox,
		uids,
		trashMailbox,
	); err != nil {
		return "", fmt.Errorf(
			"failed to move messages to trash: %w",
			err,
		)
	}

	return formatSafeDeleteResult(
		params.Account,
		params.Mailbox,
		params.UIDs,
		trashMailbox,
	), nil
}

// formatSafeDeleteResult builds a human-readable confirmation
// of a safe delete (move to Trash) operation.
func formatSafeDeleteResult(
	account, mailbox string,
	uids []uint32,
	trashMailbox string,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Moved %d message(s) to %s in %s.\n",
		len(uids),
		trashMailbox,
		account,
	)
	fmt.Fprintf(&b, "  From: %s\n", mailbox)
	fmt.Fprintf(&b, "  UIDs: %s\n", formatUIDs(uids))

	return b.String()
}

// formatPermanentDeleteResult builds a human-readable
// confirmation of a permanent delete operation.
func formatPermanentDeleteResult(
	account, mailbox string,
	uids []uint32,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Permanently deleted %d message(s) from %s/%s.\n",
		len(uids),
		account,
		mailbox,
	)

	fmt.Fprintf(&b, "  UIDs: %s\n", formatUIDs(uids))

	fmt.Fprintf(
		&b,
		"  WARNING: This action cannot be undone.\n",
	)

	return b.String()
}
