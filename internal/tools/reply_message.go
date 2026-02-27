package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	return formatReplyResult(
		params.Mode,
		cp.To,
		cp.CC,
		cp.Subject,
		savedToSent,
	), nil
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

// buildReplyParams constructs the composeParams and all
// envelope recipients for a reply/reply_all/forward.
func buildReplyParams(
	mode, from string,
	overrideTo, cc, bcc []string,
	userBody string,
	srcMsg *imapclient.FetchMessageBuffer,
	srcBody sourceBody,
) (composeParams, []string, error) {
	env := srcMsg.Envelope
	if env == nil {
		return composeParams{}, nil, fmt.Errorf(
			"source message has no envelope data",
		)
	}

	var cp composeParams
	cp.From = from
	cp.BCC = bcc

	switch mode {
	case "reply":
		cp.To = replyTo(env, overrideTo)
		cp.CC = cc
		cp.Subject = addPrefix("Re", env.Subject)
		cp.InReplyTo = env.MessageID
		cp.References = buildReferences(env)
		cp.Body = quoteReply(userBody, env, srcBody)

	case "reply_all":
		toAddrs, ccAddrs := replyAllRecipients(
			env, from, overrideTo, cc,
		)
		cp.To = toAddrs
		cp.CC = ccAddrs
		cp.Subject = addPrefix("Re", env.Subject)
		cp.InReplyTo = env.MessageID
		cp.References = buildReferences(env)
		cp.Body = quoteReply(userBody, env, srcBody)

	case "forward":
		cp.To = overrideTo
		cp.CC = cc
		cp.Subject = addPrefix("Fwd", env.Subject)
		cp.Body = quoteForward(userBody, env, srcBody)
	}

	return cp, collectRecipients(
		cp.To, cp.CC, cp.BCC,
	), nil
}

// replyTo returns the To addresses for a reply. Uses override
// if provided, otherwise replies to the original sender.
func replyTo(
	env *imaplib.Envelope,
	override []string,
) []string {
	if len(override) > 0 {
		return override
	}
	return envelopeAddrs(env.From)
}

// replyAllRecipients calculates To and CC for reply_all,
// excluding self from both lists.
func replyAllRecipients(
	env *imaplib.Envelope,
	self string,
	overrideTo, overrideCC []string,
) (to, cc []string) {
	if len(overrideTo) > 0 {
		return overrideTo, overrideCC
	}

	selfLower := strings.ToLower(self)

	// To = original From + original To (minus self).
	for _, addr := range env.From {
		email := addr.Addr()
		if strings.ToLower(email) != selfLower {
			to = append(to, email)
		}
	}
	for _, addr := range env.To {
		email := addr.Addr()
		if strings.ToLower(email) != selfLower {
			to = append(to, email)
		}
	}

	// CC = original CC (minus self), merged with override.
	ccSet := make(map[string]bool)
	for _, addr := range overrideCC {
		ccLower := strings.ToLower(addr)
		if ccLower != selfLower {
			cc = append(cc, addr)
			ccSet[ccLower] = true
		}
	}
	for _, addr := range env.Cc {
		email := addr.Addr()
		emailLower := strings.ToLower(email)
		if emailLower != selfLower &&
			!ccSet[emailLower] {
			cc = append(cc, email)
		}
	}

	return to, cc
}

// envelopeAddrs extracts email addresses from IMAP addresses.
func envelopeAddrs(addrs []imaplib.Address) []string {
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if email := addr.Addr(); email != "" {
			result = append(result, email)
		}
	}
	return result
}

// buildReferences builds the References header value from
// the source message's References and Message-ID.
func buildReferences(
	env *imaplib.Envelope,
) []string {
	// References header is not directly in Envelope,
	// so we build from Message-ID only.
	if env.MessageID != "" {
		return []string{env.MessageID}
	}
	return nil
}

// addPrefix adds "Re: " or "Fwd: " to a subject if not
// already present.
func addPrefix(prefix, subject string) string {
	check := prefix + ": "
	if strings.HasPrefix(
		strings.ToLower(subject),
		strings.ToLower(check),
	) {
		return subject
	}
	return check + subject
}

// quoteReply formats the reply body with the original quoted.
func quoteReply(
	userBody string,
	env *imaplib.Envelope,
	srcBody sourceBody,
) string {
	var b strings.Builder
	b.WriteString(userBody)
	b.WriteString("\n\n")

	sender := "(unknown)"
	if len(env.From) > 0 {
		sender = formatAddresses(env.From)
	}

	date := env.Date.Format(
		dateFormatRFC2822,
	)

	fmt.Fprintf(
		&b,
		"On %s, %s wrote:\n",
		date,
		sender,
	)

	if srcBody.text != "" {
		for _, line := range strings.Split(
			srcBody.text, "\n",
		) {
			fmt.Fprintf(&b, "> %s\n", line)
		}
	}

	return b.String()
}

// quoteForward formats the forward body with the original
// message metadata and content.
func quoteForward(
	userBody string,
	env *imaplib.Envelope,
	srcBody sourceBody,
) string {
	var b strings.Builder
	b.WriteString(userBody)

	b.WriteString(
		"\n\n---------- Forwarded message ----------\n",
	)
	fmt.Fprintf(
		&b,
		"From: %s\n",
		formatAddresses(env.From),
	)
	fmt.Fprintf(
		&b,
		"Date: %s\n",
		env.Date.Format(
			dateFormatRFC2822,
		),
	)
	fmt.Fprintf(&b, "Subject: %s\n", env.Subject)
	fmt.Fprintf(
		&b,
		"To: %s\n",
		formatAddresses(env.To),
	)
	b.WriteString("\n")

	if srcBody.text != "" {
		b.WriteString(srcBody.text)
		b.WriteString("\n")
	}

	return b.String()
}

// extractAllAttachments extracts all attachments from a raw
// message as rawAttachment values.
func extractAllAttachments(
	raw []byte,
	count int,
) ([]rawAttachment, error) {
	var result []rawAttachment
	for i := 1; i <= count; i++ {
		att, err := extractAttachment(raw, i)
		if err != nil {
			return nil, fmt.Errorf(
				"attachment %d: %w",
				i,
				err,
			)
		}

		filename := att.filename
		if filename == "" {
			filename = fmt.Sprintf(
				"attachment_%d",
				i,
			)
		}

		result = append(result, rawAttachment{
			Filename:  filename,
			MediaType: att.mediaType,
			Data:      att.data,
		})
	}
	return result, nil
}

// formatReplyResult formats the reply/forward confirmation.
func formatReplyResult(
	mode string,
	to, cc []string,
	subject string,
	savedToSent bool,
) string {
	var b strings.Builder

	modeLabel := "Reply"
	switch mode {
	case "reply_all":
		modeLabel = "Reply-all"
	case "forward":
		modeLabel = "Forward"
	}

	fmt.Fprintf(&b, "%s sent successfully.\n", modeLabel)

	fmt.Fprintf(
		&b,
		"\n  To:      %s\n",
		strings.Join(to, ", "),
	)
	if len(cc) > 0 {
		fmt.Fprintf(
			&b,
			"  CC:      %s\n",
			strings.Join(cc, ", "),
		)
	}
	fmt.Fprintf(&b, "  Subject: %s\n", subject)

	if savedToSent {
		b.WriteString("  Saved to Sent folder.\n")
	}

	return b.String()
}
