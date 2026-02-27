package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// messageFlagSetter is a narrow interface for storing flags
// on messages.
// *imapmanager.ConnectionManager satisfies this implicitly.
type messageFlagSetter interface {
	StoreFlags(
		account, mailbox string,
		uids []imap.UID,
		op imap.StoreFlagsOp,
		flags []imap.Flag,
	) error
}

// MarkMessages is an MCP tool that sets or clears flags on
// messages in a mailbox.
type MarkMessages struct {
	setter messageFlagSetter
}

// NewMarkMessages creates a new MarkMessages tool.
func NewMarkMessages(
	setter messageFlagSetter,
) *MarkMessages {
	return &MarkMessages{setter: setter}
}

// Description returns a description of what the tool does.
func (t *MarkMessages) Description() string {
	return "Mark messages as read/unread or " +
		"flagged/unflagged"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *MarkMessages) InputSchema() map[string]interface{} {
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
				"description": "Mailbox name " +
					"(e.g., 'INBOX')",
			},
			"uids": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "integer",
				},
				"description": "Message UIDs " +
					"to update",
			},
			"read": map[string]interface{}{
				"type": "boolean",
				"description": "Set true to mark " +
					"as read (\\Seen), false to " +
					"mark as unread",
			},
			"flagged": map[string]interface{}{
				"type": "boolean",
				"description": "Set true to flag " +
					"(\\Flagged), false to unflag",
			},
		},
		"required": []string{
			"account", "mailbox", "uids",
		},
	}
}

// Execute marks messages with the requested flag changes.
func (t *MarkMessages) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account string   `json:"account"`
		Mailbox string   `json:"mailbox"`
		UIDs    []uint32 `json:"uids"`
		Read    *bool    `json:"read"`
		Flagged *bool    `json:"flagged"`
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
	if params.Read == nil && params.Flagged == nil {
		return "", fmt.Errorf(
			"at least one flag parameter " +
				"(read or flagged) is required",
		)
	}

	uids := toIMAPUIDs(params.UIDs)

	var addFlags, removeFlags []imap.Flag

	if params.Read != nil {
		if *params.Read {
			addFlags = append(addFlags, imap.FlagSeen)
		} else {
			removeFlags = append(
				removeFlags, imap.FlagSeen,
			)
		}
	}

	if params.Flagged != nil {
		if *params.Flagged {
			addFlags = append(
				addFlags, imap.FlagFlagged,
			)
		} else {
			removeFlags = append(
				removeFlags, imap.FlagFlagged,
			)
		}
	}

	if len(addFlags) > 0 {
		if err := t.setter.StoreFlags(
			params.Account,
			params.Mailbox,
			uids,
			imap.StoreFlagsAdd,
			addFlags,
		); err != nil {
			return "", fmt.Errorf(
				"failed to add flags: %w",
				err,
			)
		}
	}

	if len(removeFlags) > 0 {
		if err := t.setter.StoreFlags(
			params.Account,
			params.Mailbox,
			uids,
			imap.StoreFlagsDel,
			removeFlags,
		); err != nil {
			return "", fmt.Errorf(
				"failed to remove flags: %w",
				err,
			)
		}
	}

	return formatMarkResult(
		params.Account,
		params.Mailbox,
		params.UIDs,
		addFlags,
		removeFlags,
	), nil
}

// formatMarkResult builds a human-readable confirmation of
// flag changes.
func formatMarkResult(
	account, mailbox string,
	uids []uint32,
	addFlags, removeFlags []imap.Flag,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Updated %d message(s) in %s/%s:\n",
		len(uids),
		account,
		mailbox,
	)

	if len(addFlags) > 0 {
		fmt.Fprintf(
			&b,
			"  Added flags: %s\n",
			formatFlagNames(addFlags),
		)
	}

	if len(removeFlags) > 0 {
		fmt.Fprintf(
			&b,
			"  Removed flags: %s\n",
			formatFlagNames(removeFlags),
		)
	}

	fmt.Fprintf(&b, "  UIDs: %s\n", formatUIDs(uids))

	return b.String()
}
