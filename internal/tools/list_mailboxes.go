package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// mailboxLister is a narrow interface for listing mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type mailboxLister interface {
	ListMailboxes(account string) ([]*imap.ListData, error)
}

// ListMailboxes is an MCP tool that lists all mailboxes for an
// IMAP account.
type ListMailboxes struct {
	lister mailboxLister
}

// NewListMailboxes creates a new ListMailboxes tool.
func NewListMailboxes(lister mailboxLister) *ListMailboxes {
	return &ListMailboxes{lister: lister}
}

// Description returns a description of what the tool does.
func (t *ListMailboxes) Description() string {
	return "List all mailboxes for an IMAP account with " +
		"special-use annotations"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *ListMailboxes) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from config " +
					"(e.g., 'gmail', 'protonmail')",
			},
		},
		"required": []string{"account"},
	}
}

// Execute lists all mailboxes for the specified account.
func (t *ListMailboxes) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account string `json:"account"`
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

	mailboxes, err := t.lister.ListMailboxes(params.Account)
	if err != nil {
		return "", fmt.Errorf(
			"failed to list mailboxes: %w",
			err,
		)
	}

	if len(mailboxes) == 0 {
		return fmt.Sprintf(
			"No mailboxes found for %s.",
			params.Account,
		), nil
	}

	return formatMailboxes(params.Account, mailboxes), nil
}

// specialUseLabels maps IMAP special-use attributes to
// human-readable labels.
var specialUseLabels = map[imap.MailboxAttr]string{
	imap.MailboxAttrAll:       "all mail",
	imap.MailboxAttrArchive:   "archive",
	imap.MailboxAttrDrafts:    "drafts",
	imap.MailboxAttrFlagged:   "flagged",
	imap.MailboxAttrImportant: "important",
	imap.MailboxAttrJunk:      "junk",
	imap.MailboxAttrSent:      "sent",
	imap.MailboxAttrTrash:     "trash",
}

// formatMailboxes formats a list of mailboxes into a
// human-readable string. INBOX is always listed first,
// remaining mailboxes are sorted alphabetically.
func formatMailboxes(
	account string,
	mailboxes []*imap.ListData,
) string {
	// Separate INBOX from the rest, then sort the rest.
	var inbox *imap.ListData
	rest := make([]*imap.ListData, 0, len(mailboxes))

	for _, mb := range mailboxes {
		if strings.EqualFold(mb.Mailbox, "INBOX") {
			inbox = mb
		} else {
			rest = append(rest, mb)
		}
	}

	sort.Slice(rest, func(i, j int) bool {
		return strings.ToLower(rest[i].Mailbox) <
			strings.ToLower(rest[j].Mailbox)
	})

	// Build output with INBOX first.
	sorted := make([]*imap.ListData, 0, len(mailboxes))
	if inbox != nil {
		sorted = append(sorted, inbox)
	}
	sorted = append(sorted, rest...)

	var b strings.Builder
	fmt.Fprintf(
		&b,
		"Mailboxes for %s:\n",
		account,
	)

	for _, mb := range sorted {
		annotation := specialUseAnnotation(mb.Attrs)
		if annotation != "" {
			fmt.Fprintf(
				&b,
				"\n  %s    (%s)",
				mb.Mailbox,
				annotation,
			)
		} else {
			fmt.Fprintf(&b, "\n  %s", mb.Mailbox)
		}
	}

	b.WriteString("\n")
	return b.String()
}

// specialUseAnnotation returns the human-readable label for
// the first special-use attribute found, or "" if none.
func specialUseAnnotation(
	attrs []imap.MailboxAttr,
) string {
	for _, attr := range attrs {
		if label, ok := specialUseLabels[attr]; ok {
			return label
		}
	}
	return ""
}
