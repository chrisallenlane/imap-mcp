package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// specialUseAttrs is the set of IMAP special-use attributes
// that protect a mailbox from deletion.
var specialUseAttrs = map[imap.MailboxAttr]bool{
	imap.MailboxAttrSent:    true,
	imap.MailboxAttrTrash:   true,
	imap.MailboxAttrDrafts:  true,
	imap.MailboxAttrJunk:    true,
	imap.MailboxAttrArchive: true,
	imap.MailboxAttrFlagged: true,
}

// mailboxDeleter is a narrow interface for deleting mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type mailboxDeleter interface {
	DeleteMailbox(account, name string) error
	ListMailboxes(
		account string,
	) ([]*imap.ListData, error)
}

// DeleteMailbox is an MCP tool that deletes a mailbox.
type DeleteMailbox struct {
	deleter mailboxDeleter
}

// NewDeleteMailbox creates a new DeleteMailbox tool.
func NewDeleteMailbox(
	deleter mailboxDeleter,
) *DeleteMailbox {
	return &DeleteMailbox{deleter: deleter}
}

// Description returns a description of what the tool does.
func (t *DeleteMailbox) Description() string {
	return "Delete a mailbox (folder)"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *DeleteMailbox) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from " +
					"config",
			},
			"name": map[string]interface{}{
				"type": "string",
				"description": "Name of the mailbox " +
					"to delete",
			},
		},
		"required": []string{"account", "name"},
	}
}

// Execute deletes a mailbox from the IMAP server.
func (t *DeleteMailbox) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account string `json:"account"`
		Name    string `json:"name"`
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
	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Refuse to delete INBOX.
	if strings.EqualFold(params.Name, "INBOX") {
		return "", fmt.Errorf(
			"cannot delete INBOX",
		)
	}

	// Check for special-use attributes.
	mailboxes, err := t.deleter.ListMailboxes(
		params.Account,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to list mailboxes: %w",
			err,
		)
	}

	for _, mb := range mailboxes {
		if mb.Mailbox != params.Name {
			continue
		}
		for _, attr := range mb.Attrs {
			if specialUseAttrs[attr] {
				return "", fmt.Errorf(
					"cannot delete mailbox %q:"+
						" it has special-use"+
						" attribute %s",
					params.Name,
					string(attr),
				)
			}
		}
		break
	}

	if err := t.deleter.DeleteMailbox(
		params.Account,
		params.Name,
	); err != nil {
		return "", fmt.Errorf(
			"failed to delete mailbox: %w",
			err,
		)
	}

	return fmt.Sprintf(
		"Deleted mailbox %q from %s.\n",
		params.Name,
		params.Account,
	), nil
}
