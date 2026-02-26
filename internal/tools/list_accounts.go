package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	imapmanager "github.com/chrisallenlane/imap-mcp/internal/imap"
)

// ListAccounts is an MCP tool that lists all configured IMAP
// accounts with their connection status.
type ListAccounts struct {
	imap *imapmanager.Manager
}

// NewListAccounts creates a new ListAccounts tool.
func NewListAccounts(
	mgr *imapmanager.Manager,
) *ListAccounts {
	return &ListAccounts{imap: mgr}
}

// Description returns a description of what the tool does.
func (t *ListAccounts) Description() string {
	return "List all configured IMAP accounts with " +
		"connection status"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *ListAccounts) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

// Execute lists all configured accounts and their status.
func (t *ListAccounts) Execute(
	_ context.Context,
	_ json.RawMessage,
) (string, error) {
	cfg := t.imap.Config()

	if len(cfg.Accounts) == 0 {
		return "No accounts configured.", nil
	}

	// Sort account names for deterministic output.
	names := make([]string, 0, len(cfg.Accounts))
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("Configured accounts:\n")

	for i, name := range names {
		acct := cfg.Accounts[name]

		tlsLabel := ""
		if acct.TLS {
			tlsLabel = " (TLS)"
		}

		status := "not connected"
		if t.imap.IsConnected(name) {
			status = "connected"
		}

		fmt.Fprintf(&b, "\n%d. %s\n", i+1, name)
		fmt.Fprintf(
			&b,
			"   Host: %s:%d%s\n",
			acct.Host,
			acct.Port,
			tlsLabel,
		)
		fmt.Fprintf(&b, "   User: %s\n", acct.Username)
		fmt.Fprintf(&b, "   Status: %s\n", status)
	}

	return b.String(), nil
}
