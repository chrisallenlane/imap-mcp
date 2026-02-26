package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // register charsets
)

// messageGetter is a narrow interface for fetching a single
// message by UID.
// *imapmanager.Manager satisfies this implicitly.
type messageGetter interface {
	FetchMessageByUID(
		account string,
		mailbox string,
		uid imap.UID,
		options *imap.FetchOptions,
	) ([]*imapclient.FetchMessageBuffer, error)
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
		Account string   `json:"account"`
		Mailbox string   `json:"mailbox"`
		UID     imap.UID `json:"uid"`
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
	if params.UID == 0 {
		return "", fmt.Errorf(
			"uid is required and must be > 0",
		)
	}

	messages, err := t.getter.FetchMessageByUID(
		params.Account,
		params.Mailbox,
		params.UID,
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
		return "", fmt.Errorf(
			"failed to fetch message: %w",
			err,
		)
	}

	if len(messages) == 0 {
		return "", fmt.Errorf("message not found")
	}

	msg := messages[0]
	return formatFullMessage(
		params.Account,
		params.Mailbox,
		msg,
	)
}

// maxBodySize is the maximum number of bytes to read from a
// plain-text message body. Bodies exceeding this limit are
// truncated.
const maxBodySize = 1 << 20 // 1 MB

// attachment holds metadata about a MIME attachment.
type attachment struct {
	filename  string
	size      int
	mediaType string
}

// parsedBody holds the result of parsing a message body.
type parsedBody struct {
	text        string
	attachments []attachment
	fromHTML    bool
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
	formatFlagLine(&b, msg.Flags)

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
		writeIndentedBody(&b, parsed.text)
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
		env.Date.Format(
			"Mon, 02 Jan 2006 15:04:05 -0700",
		),
	)
	fmt.Fprintf(b, "  Subject: %s\n", env.Subject)
}

// formatFlagLine writes the flags line to the builder.
func formatFlagLine(
	b *strings.Builder,
	flags []imap.Flag,
) {
	flagStr := formatFlags(flags)
	if flagStr != "" {
		fmt.Fprintf(b, "  Flags:   %s\n", flagStr)
	}
}

// formatAddresses formats a slice of IMAP addresses as a
// comma-separated string. Addresses with a display name
// render as "Name <email>", otherwise just "email".
func formatAddresses(addrs []imap.Address) string {
	if len(addrs) == 0 {
		return "(unknown)"
	}

	parts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		parts = append(parts, formatAddress(addr))
	}
	return strings.Join(parts, ", ")
}

// formatAddress formats a single IMAP address.
func formatAddress(addr imap.Address) string {
	email := addr.Addr()
	if email == "" {
		email = "(unknown)"
	}
	if addr.Name != "" {
		return fmt.Sprintf("%s <%s>", addr.Name, email)
	}
	return email
}

// parseBody parses raw RFC 2822 message bytes, extracting the
// plain text body (or HTML fallback) and attachment metadata.
func parseBody(raw []byte) (parsedBody, error) {
	entity, err := message.Read(bytes.NewReader(raw))
	if err != nil && !message.IsUnknownCharset(err) {
		return parsedBody{}, fmt.Errorf(
			"failed to read message: %w",
			err,
		)
	}

	var plainText string
	var htmlText string
	var attachments []attachment
	var plainFound bool
	var htmlFound bool

	walkErr := entity.Walk(
		func(
			path []int,
			part *message.Entity,
			err error,
		) error {
			if err != nil {
				return nil
			}

			mediaType, ctParams, _ := part.Header.ContentType()
			disp, dispParams, _ := part.Header.ContentDisposition()

			// Collect the first text/plain part.
			if mediaType == "text/plain" &&
				disp != "attachment" &&
				!plainFound {
				data, readErr := readBodyPart(
					part.Body,
				)
				if readErr != nil {
					return nil
				}
				plainText = data
				plainFound = true
				return nil
			}

			// Collect the first text/html part as
			// fallback.
			if mediaType == "text/html" &&
				disp != "attachment" &&
				!htmlFound {
				data, readErr := readBodyPart(
					part.Body,
				)
				if readErr != nil {
					return nil
				}
				htmlText = data
				htmlFound = true
				return nil
			}

			isAttachment := disp == "attachment" ||
				(path != nil &&
					!strings.HasPrefix(
						mediaType,
						"text/",
					) &&
					!strings.HasPrefix(
						mediaType,
						"multipart/",
					))

			if isAttachment {
				filename := dispParams["filename"]
				if filename == "" {
					filename = ctParams["name"]
				}
				if filename == "" {
					filename = "unnamed"
				}

				n, copyErr := io.Copy(
					io.Discard,
					part.Body,
				)
				if copyErr != nil {
					return nil
				}

				attachments = append(
					attachments,
					attachment{
						filename:  filename,
						size:      int(n),
						mediaType: mediaType,
					},
				)
			}

			return nil
		},
	)
	if walkErr != nil {
		return parsedBody{}, fmt.Errorf(
			"failed to walk message parts: %w",
			walkErr,
		)
	}

	// Prefer text/plain; fall back to text/html.
	if plainFound {
		return parsedBody{
			text:        plainText,
			attachments: attachments,
		}, nil
	}
	if htmlFound {
		converted := HTMLToText(htmlText)
		return parsedBody{
			text:        converted,
			attachments: attachments,
			fromHTML:    true,
		}, nil
	}

	return parsedBody{attachments: attachments}, nil
}

// readBodyPart reads a MIME body part up to maxBodySize,
// appending a truncation notice if the limit is exceeded.
func readBodyPart(r io.Reader) (string, error) {
	lr := &io.LimitedReader{
		R: r,
		N: maxBodySize + 1,
	}
	data, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	if len(data) > maxBodySize {
		return string(
			data[:maxBodySize],
		) + "\n\n[body truncated at 1 MB]", nil
	}
	return string(data), nil
}

// writeIndentedBody writes body text with each line indented
// by two spaces.
func writeIndentedBody(b *strings.Builder, body string) {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		fmt.Fprintf(b, "  %s\n", line)
	}
}

// formatSize formats a byte count as a human-readable string.
func formatSize(bytes int) string {
	const kb = 1024
	const mb = 1024 * kb

	switch {
	case bytes >= mb:
		return fmt.Sprintf(
			"%.1f MB",
			float64(bytes)/float64(mb),
		)
	case bytes >= kb:
		return fmt.Sprintf(
			"%d KB",
			bytes/kb,
		)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
