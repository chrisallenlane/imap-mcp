package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	imaplib "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// ReplyMessage is an MCP tool that replies to, replies all,
// or forwards an email message.
type ReplyMessage struct {
	getter messageGetter
	sender emailSender
	saver  sentSaver
}

// NewReplyMessage creates a new ReplyMessage tool.
func NewReplyMessage(
	getter messageGetter,
	sender emailSender,
	saver sentSaver,
) *ReplyMessage {
	return &ReplyMessage{
		getter: getter,
		sender: sender,
		saver:  saver,
	}
}

// Description returns a description of what the tool does.
func (t *ReplyMessage) Description() string {
	return "Reply to, reply all, or forward an " +
		"email message"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *ReplyMessage) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from " +
					"config (e.g., 'gmail')",
			},
			"mailbox": map[string]interface{}{
				"type": "string",
				"description": "Mailbox containing " +
					"the source message",
			},
			"uid": map[string]interface{}{
				"type": "integer",
				"description": "UID of the message " +
					"to reply to or forward",
			},
			"mode": map[string]interface{}{
				"type": "string",
				"enum": []string{
					"reply",
					"reply_all",
					"forward",
				},
				"description": "Reply mode: " +
					"reply, reply_all, or forward",
			},
			"to": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Override or specify " +
					"recipients (required for " +
					"forward)",
			},
			"cc": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "CC recipients",
			},
			"bcc": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "BCC recipients",
			},
			"body": map[string]interface{}{
				"type": "string",
				"description": "Your reply or " +
					"forward text (prepended " +
					"above quoted original)",
			},
			"include_attachments": map[string]interface{}{
				"type": "boolean",
				"description": "For forward mode, " +
					"carry original attachments " +
					"(default: false)",
			},
		},
		"required": []string{
			"account",
			"mailbox",
			"uid",
			"mode",
			"body",
		},
	}
}

// Execute replies to, replies all, or forwards a message.
func (t *ReplyMessage) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account            string          `json:"account"`
		Mailbox            string          `json:"mailbox"`
		UID                json.RawMessage `json:"uid"`
		Mode               string          `json:"mode"`
		To                 []string        `json:"to"`
		CC                 []string        `json:"cc"`
		BCC                []string        `json:"bcc"`
		Body               string          `json:"body"`
		IncludeAttachments bool            `json:"include_attachments"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf(
			"failed to parse arguments: %w",
			err,
		)
	}

	if err := validateReplyParams(
		params.Account,
		params.Mailbox,
		params.Mode,
		params.Body,
		params.To,
	); err != nil {
		return "", err
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

	acct, from, err := resolveSMTPAccount(
		t.sender.Config(), params.Account,
	)
	if err != nil {
		return "", err
	}

	// Fetch the source message.
	srcMsg, srcBody, err := t.fetchSource(
		params.Account,
		params.Mailbox,
		uid,
	)
	if err != nil {
		return "", err
	}

	// Build the composed message.
	cp, allRecipients, err := buildReplyParams(
		params.Mode,
		from,
		params.To,
		params.CC,
		params.BCC,
		params.Body,
		srcMsg,
		srcBody,
	)
	if err != nil {
		return "", err
	}

	// Handle attachment forwarding.
	if params.Mode == "forward" &&
		params.IncludeAttachments &&
		len(srcBody.attachments) > 0 {
		rawAtts, err := extractAllAttachments(
			srcBody.raw,
			len(srcBody.attachments),
		)
		if err != nil {
			return "", fmt.Errorf(
				"failed to extract attachments: %w",
				err,
			)
		}
		cp.RawAttachments = rawAtts
	}

	msgBytes, err := composeMessage(cp)
	if err != nil {
		return "", fmt.Errorf(
			"failed to compose message: %w",
			err,
		)
	}

	if err := t.sender.Send(
		params.Account,
		from,
		allRecipients,
		bytes.NewReader(msgBytes),
	); err != nil {
		return "", fmt.Errorf(
			"failed to send message: %w",
			err,
		)
	}

	// Save to Sent folder if configured.
	savedToSent := acct.SaveSent &&
		trySaveToSent(t.saver, params.Account, msgBytes)

	modeLabel := "Reply"
	switch params.Mode {
	case "reply_all":
		modeLabel = "Reply-all"
	case "forward":
		modeLabel = "Forward"
	}

	return formatSendConfirmation(sendConfirmation{
		Title:       modeLabel,
		To:          cp.To,
		CC:          cp.CC,
		Subject:     cp.Subject,
		SavedToSent: savedToSent,
	}), nil
}

// validateReplyParams validates the required parameters.
func validateReplyParams(
	account, mailbox, mode, body string,
	to []string,
) error {
	if account == "" {
		return fmt.Errorf("account is required")
	}
	if mailbox == "" {
		return fmt.Errorf("mailbox is required")
	}
	if mode == "" {
		return fmt.Errorf("mode is required")
	}
	if mode != "reply" &&
		mode != "reply_all" &&
		mode != "forward" {
		return fmt.Errorf(
			"mode must be \"reply\", " +
				"\"reply_all\", or \"forward\"",
		)
	}
	if body == "" {
		return fmt.Errorf("body is required")
	}
	if mode == "forward" && len(to) == 0 {
		return fmt.Errorf(
			"to is required for forward mode",
		)
	}
	return nil
}

// sourceBody holds the parsed body content from a source
// message, including raw bytes for attachment extraction.
type sourceBody struct {
	text        string
	fromHTML    bool
	attachments []attachment
	raw         []byte
}

// fetchSource fetches the source message and parses its body.
func (t *ReplyMessage) fetchSource(
	account, mailbox string,
	uid imaplib.UID,
) (*imapclient.FetchMessageBuffer, sourceBody, error) {
	messages, err := t.getter.FetchMessagesByUID(
		account,
		mailbox,
		[]imaplib.UID{uid},
		&imaplib.FetchOptions{
			Envelope: true,
			Flags:    true,
			UID:      true,
			BodySection: []*imaplib.FetchItemBodySection{
				{Peek: true},
			},
		},
	)
	if err != nil {
		return nil, sourceBody{}, fmt.Errorf(
			"failed to fetch source message: %w",
			err,
		)
	}
	if len(messages) == 0 {
		return nil, sourceBody{}, fmt.Errorf(
			"source message not found",
		)
	}

	msg := messages[0]
	bodySection := &imaplib.FetchItemBodySection{}
	rawBytes := msg.FindBodySection(bodySection)

	var sb sourceBody
	if rawBytes != nil {
		sb.raw = rawBytes
		parsed, parseErr := parseBody(rawBytes)
		if parseErr == nil {
			sb.text = parsed.text
			sb.fromHTML = parsed.fromHTML
			sb.attachments = parsed.attachments
		}
	}

	return msg, sb, nil
}
