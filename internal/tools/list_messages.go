package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

const pageSize = 100

// messageFetcher is a narrow interface for examining mailboxes
// and fetching messages.
// *imapmanager.Manager satisfies this implicitly.
type messageFetcher interface {
	ExamineMailbox(
		account, mailbox string,
	) (*imap.SelectData, error)
	FetchMessages(
		account string,
		seqSet imap.SeqSet,
		options *imap.FetchOptions,
	) ([]*imapclient.FetchMessageBuffer, error)
}

// ListMessages is an MCP tool that lists message envelopes in
// a mailbox with pagination.
type ListMessages struct {
	fetcher messageFetcher
}

// NewListMessages creates a new ListMessages tool.
func NewListMessages(
	fetcher messageFetcher,
) *ListMessages {
	return &ListMessages{fetcher: fetcher}
}

// Description returns a description of what the tool does.
func (t *ListMessages) Description() string {
	return "List message envelopes in a mailbox " +
		"with pagination"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *ListMessages) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from config " +
					"(e.g., 'gmail', 'protonmail')",
			},
			"mailbox": map[string]interface{}{
				"type": "string",
				"description": "Mailbox name " +
					"(e.g., 'INBOX', " +
					"'[Gmail]/Sent Mail')",
			},
			"page": map[string]interface{}{
				"type": "integer",
				"description": "Page number " +
					"(default: 1, 100 messages " +
					"per page)",
				"default": 1,
			},
		},
		"required": []string{"account", "mailbox"},
	}
}

// Execute lists message envelopes for the specified account
// and mailbox.
func (t *ListMessages) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account string `json:"account"`
		Mailbox string `json:"mailbox"`
		Page    int    `json:"page"`
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
	if params.Page <= 0 {
		params.Page = 1
	}

	selectData, err := t.fetcher.ExamineMailbox(
		params.Account,
		params.Mailbox,
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to examine mailbox: %w",
			err,
		)
	}

	if selectData.NumMessages == 0 {
		return fmt.Sprintf(
			"No messages in %s/%s.",
			params.Account,
			params.Mailbox,
		), nil
	}

	lo, hi, totalPages, err := pageRange(
		selectData.NumMessages,
		params.Page,
		pageSize,
	)
	if err != nil {
		return "", err
	}

	var seqSet imap.SeqSet
	seqSet.AddRange(lo, hi)

	messages, err := t.fetcher.FetchMessages(
		params.Account,
		seqSet,
		&imap.FetchOptions{
			Envelope: true,
			Flags:    true,
			UID:      true,
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"failed to fetch messages: %w",
			err,
		)
	}

	return formatMessages(
		params.Account,
		params.Mailbox,
		messages,
		params.Page,
		totalPages,
		selectData.NumMessages,
	), nil
}

// pageRange computes the lo and hi sequence numbers for the
// given page, along with the total number of pages. Messages
// are ordered by sequence number (lowest = oldest), and pages
// count from the newest end. Page 1 returns the highest
// sequence numbers.
func pageRange(
	total uint32,
	page, pageSz int,
) (lo, hi uint32, totalPages int, err error) {
	if pageSz < 1 {
		return 0, 0, 0, fmt.Errorf(
			"pageSz must be >= 1, got %d",
			pageSz,
		)
	}

	if page < 1 {
		return 0, 0, 0, fmt.Errorf(
			"page must be >= 1, got %d",
			page,
		)
	}

	totalPages = (int(total) + pageSz - 1) / pageSz
	if page > totalPages {
		return 0, 0, 0, fmt.Errorf(
			"page %d out of range (1-%d)",
			page,
			totalPages,
		)
	}

	hi = total - uint32((page-1)*pageSz)

	// Use signed arithmetic to avoid uint32 underflow
	// when hi < pageSz.
	loInt := int(hi) - pageSz + 1
	if loInt < 1 {
		loInt = 1
	}
	lo = uint32(loInt)

	return lo, hi, totalPages, nil
}

// formatMessages formats fetched messages into a
// human-readable string. Messages are displayed newest first
// (reverse sequence number order).
func formatMessages(
	account, mailbox string,
	messages []*imapclient.FetchMessageBuffer,
	page, totalPages int,
	totalMessages uint32,
) string {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Messages in %s/%s (page %d of %d, %d total):\n",
		account,
		mailbox,
		page,
		totalPages,
		totalMessages,
	)

	// Display newest first (highest sequence number first).
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		b.WriteString("\n")
		formatMessage(&b, msg)
	}

	fmt.Fprintf(
		&b,
		"\nPage %d of %d. "+
			"Use page parameter to navigate.\n",
		page,
		totalPages,
	)

	return b.String()
}
