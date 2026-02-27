package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// maxSearchResults is the maximum number of UIDs returned
// by a search before capping.
const maxSearchResults = 100

// messageSearcher is a narrow interface for searching
// messages and fetching results by UID.
// *imapmanager.ConnectionManager satisfies this implicitly.
type messageSearcher interface {
	SearchMessages(
		account, mailbox string,
		criteria *imap.SearchCriteria,
	) ([]imap.UID, error)
	FetchMessagesByUID(
		account, mailbox string,
		uids []imap.UID,
		options *imap.FetchOptions,
	) ([]*imapclient.FetchMessageBuffer, error)
}

// SearchMessages is an MCP tool that searches messages in a
// mailbox using IMAP SEARCH criteria.
type SearchMessages struct {
	searcher messageSearcher
}

// NewSearchMessages creates a new SearchMessages tool.
func NewSearchMessages(
	searcher messageSearcher,
) *SearchMessages {
	return &SearchMessages{searcher: searcher}
}

// Description returns a description of what the tool does.
func (t *SearchMessages) Description() string {
	return "Search messages in a mailbox using " +
		"IMAP SEARCH criteria"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *SearchMessages) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from " +
					"config (e.g., 'gmail', " +
					"'protonmail')",
			},
			"mailbox": map[string]interface{}{
				"type": "string",
				"description": "Mailbox name " +
					"(e.g., 'INBOX', " +
					"'[Gmail]/Sent Mail')",
			},
			"from": map[string]interface{}{
				"type": "string",
				"description": "Search in " +
					"From header",
			},
			"to": map[string]interface{}{
				"type": "string",
				"description": "Search in " +
					"To header",
			},
			"subject": map[string]interface{}{
				"type": "string",
				"description": "Search in " +
					"Subject header",
			},
			"body": map[string]interface{}{
				"type": "string",
				"description": "Search in " +
					"message body text",
			},
			"since": map[string]interface{}{
				"type": "string",
				"description": "Messages since " +
					"date (YYYY-MM-DD)",
			},
			"before": map[string]interface{}{
				"type": "string",
				"description": "Messages before " +
					"date (YYYY-MM-DD)",
			},
			"flagged": map[string]interface{}{
				"type": "boolean",
				"description": "If true, only " +
					"flagged; if false, " +
					"only unflagged",
			},
			"seen": map[string]interface{}{
				"type": "boolean",
				"description": "If true, only " +
					"seen; if false, " +
					"only unseen",
			},
		},
		"required": []string{"account", "mailbox"},
	}
}

// Execute searches for messages matching the given criteria.
func (t *SearchMessages) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account string `json:"account"`
		Mailbox string `json:"mailbox"`
		From    string `json:"from"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
		Since   string `json:"since"`
		Before  string `json:"before"`
		Flagged *bool  `json:"flagged"`
		Seen    *bool  `json:"seen"`
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

	criteria, err := buildCriteria(
		params.From,
		params.To,
		params.Subject,
		params.Body,
		params.Since,
		params.Before,
		params.Flagged,
		params.Seen,
	)
	if err != nil {
		return "", err
	}

	uids, err := t.searcher.SearchMessages(
		params.Account,
		params.Mailbox,
		criteria,
	)
	if err != nil {
		return "", fmt.Errorf(
			"search failed: %w",
			err,
		)
	}

	if len(uids) == 0 {
		return fmt.Sprintf(
			"No messages matching search criteria "+
				"in %s/%s.",
			params.Account,
			params.Mailbox,
		), nil
	}

	totalMatches := len(uids)
	capped := totalMatches > maxSearchResults

	fetchUIDs := uids
	if capped {
		// Keep the highest UIDs (newest messages).
		fetchUIDs = uids[len(uids)-maxSearchResults:]
	}

	messages, err := t.searcher.FetchMessagesByUID(
		params.Account,
		params.Mailbox,
		fetchUIDs,
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

	return formatSearchResults(
		params.Account,
		params.Mailbox,
		messages,
		totalMatches,
		capped,
	), nil
}

// buildCriteria constructs an imap.SearchCriteria from the
// tool's input parameters. It returns an error if no search
// criteria are provided or if date parsing fails.
func buildCriteria(
	from, to, subject, body string,
	since, before string,
	flagged, seen *bool,
) (*imap.SearchCriteria, error) {
	hasCriteria := from != "" ||
		to != "" ||
		subject != "" ||
		body != "" ||
		since != "" ||
		before != "" ||
		flagged != nil ||
		seen != nil

	if !hasCriteria {
		return nil, fmt.Errorf(
			"at least one search criterion is required",
		)
	}

	criteria := &imap.SearchCriteria{}

	for _, hdr := range []struct{ key, val string }{
		{"From", from},
		{"To", to},
		{"Subject", subject},
	} {
		if hdr.val != "" {
			criteria.Header = append(
				criteria.Header,
				imap.SearchCriteriaHeaderField{
					Key:   hdr.key,
					Value: hdr.val,
				},
			)
		}
	}

	if body != "" {
		criteria.Body = append(criteria.Body, body)
	}

	parseDate := func(
		name, value string,
	) (time.Time, error) {
		t, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, fmt.Errorf(
				"invalid %s date %q: "+
					"expected YYYY-MM-DD",
				name,
				value,
			)
		}
		return t, nil
	}

	if since != "" {
		t, err := parseDate("since", since)
		if err != nil {
			return nil, err
		}
		criteria.Since = t
	}

	if before != "" {
		t, err := parseDate("before", before)
		if err != nil {
			return nil, err
		}
		criteria.Before = t
	}

	for _, fc := range []struct {
		val  *bool
		flag imap.Flag
	}{
		{flagged, imap.FlagFlagged},
		{seen, imap.FlagSeen},
	} {
		if fc.val == nil {
			continue
		}
		if *fc.val {
			criteria.Flag = append(
				criteria.Flag, fc.flag,
			)
		} else {
			criteria.NotFlag = append(
				criteria.NotFlag, fc.flag,
			)
		}
	}

	return criteria, nil
}

// formatSearchResults formats search results into a
// human-readable string. Messages are sorted newest first
// by envelope date.
func formatSearchResults(
	account, mailbox string,
	messages []*imapclient.FetchMessageBuffer,
	totalMatches int,
	capped bool,
) string {
	// Sort newest first by envelope date.
	slices.SortFunc(
		messages,
		func(
			a, b *imapclient.FetchMessageBuffer,
		) int {
			return envelopeDate(b).Compare(
				envelopeDate(a),
			)
		},
	)

	matchWord := "matches"
	if totalMatches == 1 {
		matchWord = "match"
	}

	info := fmt.Sprintf(
		"%d %s, showing all",
		totalMatches,
		matchWord,
	)
	if capped {
		info = fmt.Sprintf(
			"showing %d of %d %s",
			maxSearchResults,
			totalMatches,
			matchWord,
		)
	}

	var b strings.Builder
	fmt.Fprintf(
		&b,
		"Search results in %s/%s (%s):\n",
		account,
		mailbox,
		info,
	)

	for _, msg := range messages {
		b.WriteString("\n")
		formatMessage(&b, msg)
	}

	if capped {
		fmt.Fprintf(
			&b,
			"\n%d total %s. "+
				"Showing first %d. "+
				"Narrow your search criteria "+
				"for more specific results.\n",
			totalMatches,
			matchWord,
			maxSearchResults,
		)
	}

	return b.String()
}
