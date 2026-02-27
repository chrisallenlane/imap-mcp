package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// messageGetter is a narrow interface for fetching messages
// by UID.
// *imapmanager.ConnectionManager satisfies this implicitly.
type messageGetter interface {
	FetchMessagesByUID(
		account, mailbox string,
		uids []imap.UID,
		options *imap.FetchOptions,
	) ([]*imapclient.FetchMessageBuffer, error)
}

// fetchSingleMessage fetches a single message by UID,
// returning an error if the message is not found.
func fetchSingleMessage(
	getter messageGetter,
	account, mailbox string,
	uid imap.UID,
	options *imap.FetchOptions,
) (*imapclient.FetchMessageBuffer, error) {
	messages, err := getter.FetchMessagesByUID(
		account,
		mailbox,
		[]imap.UID{uid},
		options,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch message: %w",
			err,
		)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("message not found")
	}
	return messages[0], nil
}

// GetMessage is an MCP tool that retrieves a full email
// message by UID.
type GetMessage struct {
	getter messageGetter
}

// NewGetMessage creates a new GetMessage tool.
func NewGetMessage(getter messageGetter) *GetMessage {
	return &GetMessage{getter: getter}
}

// Description returns a description of what the tool does.
func (t *GetMessage) Description() string {
	return "Retrieve a full email message by UID, " +
		"including headers, body, and attachment metadata"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *GetMessage) InputSchema() map[string]interface{} {
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
			"uid": map[string]interface{}{
				"type":        "integer",
				"description": "Message UID",
			},
		},
		"required": []string{
			"account",
			"mailbox",
			"uid",
		},
	}
}

// Execute retrieves a full message by UID.
func (t *GetMessage) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account string          `json:"account"`
		Mailbox string          `json:"mailbox"`
		UID     json.RawMessage `json:"uid"`
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

	uid, err := parseUID(params.UID)
	if err != nil {
		return "", err
	}
	if uid == 0 {
		return "", fmt.Errorf(
			"uid is required and must be > 0",
		)
	}

	msg, err := fetchSingleMessage(
		t.getter,
		params.Account,
		params.Mailbox,
		uid,
		&imap.FetchOptions{
			Envelope: true,
			Flags:    true,
			UID:      true,
			BodySection: []*imap.FetchItemBodySection{
				{Peek: true},
			},
		},
	)
	if err != nil {
		return "", err
	}

	return formatFullMessage(
		params.Account,
		params.Mailbox,
		msg,
	)
}

// formatFullMessage formats a complete message for display.
func formatFullMessage(
	account, mailbox string,
	msg *imapclient.FetchMessageBuffer,
) (string, error) {
	var b strings.Builder

	fmt.Fprintf(
		&b,
		"Message UID %d in %s/%s:\n",
		msg.UID,
		account,
		mailbox,
	)

	formatEnvelope(&b, msg)

	if flagStr := formatFlags(msg.Flags); flagStr != "" {
		fmt.Fprintf(&b, "  Flags:   %s\n", flagStr)
	}

	bodySection := &imap.FetchItemBodySection{}
	bodyBytes := msg.FindBodySection(bodySection)

	if bodyBytes == nil {
		b.WriteString("\n  Body:\n")
		b.WriteString(
			"  (no body data available)\n",
		)
		return b.String(), nil
	}

	parsed, err := parseBody(bodyBytes)
	if err != nil {
		return "", fmt.Errorf(
			"failed to parse message body: %w",
			err,
		)
	}

	if parsed.fromHTML {
		b.WriteString(
			"\n  Body (converted from HTML):\n",
		)
	} else {
		b.WriteString("\n  Body:\n")
	}

	if parsed.text == "" {
		b.WriteString(
			"  (no readable body content)\n",
		)
	} else {
		for _, line := range strings.Split(
			parsed.text, "\n",
		) {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}

	if len(parsed.attachments) > 0 {
		b.WriteString("\n  Attachments:\n")
		for i, att := range parsed.attachments {
			fmt.Fprintf(
				&b,
				"  %d. %s (%s, %s)\n",
				i+1,
				att.filename,
				formatSize(att.size),
				att.mediaType,
			)
		}
	}

	return b.String(), nil
}

// formatEnvelope writes envelope headers to the builder.
func formatEnvelope(
	b *strings.Builder,
	msg *imapclient.FetchMessageBuffer,
) {
	if msg.Envelope == nil {
		b.WriteString("\n  (no envelope data)\n")
		return
	}

	env := msg.Envelope

	fmt.Fprintf(
		b,
		"\n  From:    %s\n",
		formatAddresses(env.From),
	)
	fmt.Fprintf(
		b,
		"  To:      %s\n",
		formatAddresses(env.To),
	)
	if len(env.Cc) > 0 {
		fmt.Fprintf(
			b,
			"  CC:      %s\n",
			formatAddresses(env.Cc),
		)
	}
	fmt.Fprintf(
		b,
		"  Date:    %s\n",
		env.Date.Format(dateFormatRFC2822),
	)
	fmt.Fprintf(b, "  Subject: %s\n", env.Subject)
}
