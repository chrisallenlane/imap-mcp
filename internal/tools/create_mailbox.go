package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// mailboxCreator is a narrow interface for creating mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type mailboxCreator interface {
	CreateMailbox(account, name string) error
}

// CreateMailbox is an MCP tool that creates a new mailbox.
type CreateMailbox struct {
	creator mailboxCreator
}

// NewCreateMailbox creates a new CreateMailbox tool.
func NewCreateMailbox(
	creator mailboxCreator,
) *CreateMailbox {
	return &CreateMailbox{creator: creator}
}

// Description returns a description of what the tool does.
func (t *CreateMailbox) Description() string {
	return "Create a new mailbox (folder)"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *CreateMailbox) InputSchema() map[string]interface{} {
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
					"to create (e.g., " +
					"'Work/Projects')",
			},
		},
		"required": []string{"account", "name"},
	}
}

// Execute creates a new mailbox on the IMAP server.
func (t *CreateMailbox) Execute(
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

	if err := t.creator.CreateMailbox(
		params.Account,
		params.Name,
	); err != nil {
		return "", fmt.Errorf(
			"failed to create mailbox: %w",
			err,
		)
	}

	return fmt.Sprintf(
		"Created mailbox %q in %s.\n",
		params.Name,
		params.Account,
	), nil
}
