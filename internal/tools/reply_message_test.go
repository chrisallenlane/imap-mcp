package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	imaplib "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// mockReplyGetter implements replyGetter for testing.
type mockReplyGetter struct {
	messages []*imapclient.FetchMessageBuffer
	fetchErr error
}

func (m *mockReplyGetter) FetchMessagesByUID(
	_, _ string,
	_ []imaplib.UID,
	_ *imaplib.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	return m.messages, m.fetchErr
}

// mockReplySender implements replySender for testing.
type mockReplySender struct {
	config  *config.Config
	sendErr error
	sentTo  []string
	sentMsg []byte
}

func (m *mockReplySender) Send(
	_, _ string,
	to []string,
	msg io.Reader,
) error {
	m.sentTo = to
	var buf bytes.Buffer
	buf.ReadFrom(msg)
	m.sentMsg = buf.Bytes()
	return m.sendErr
}

func (m *mockReplySender) Config() *config.Config {
	return m.config
}

// mockReplySentSaver implements replySentSaver for testing.
type mockReplySentSaver struct {
	sentMailbox  string
	findErr      error
	appendErr    error
	appendCalled bool
}

func (m *mockReplySentSaver) FindSentMailbox(
	_ string,
) (string, error) {
	return m.sentMailbox, m.findErr
}

func (m *mockReplySentSaver) AppendMessage(
	_, _ string,
	_ []byte,
	_ []imaplib.Flag,
) error {
	m.appendCalled = true
	return m.appendErr
}

func replyConfig() *config.Config {
	return &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "me@example.com",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
			},
		},
	}
}

// sourceMessage builds a mock source message for testing.
func sourceMessage() *imapclient.FetchMessageBuffer {
	return &imapclient.FetchMessageBuffer{
		UID: 42,
		Envelope: &imaplib.Envelope{
			Date:      time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Subject:   "Original Subject",
			MessageID: "original-msg-id@example.com",
			From: []imaplib.Address{
				{Name: "Sender", Mailbox: "sender", Host: "example.com"},
			},
			To: []imaplib.Address{
				{Name: "Me", Mailbox: "me", Host: "example.com"},
			},
			Cc: []imaplib.Address{
				{Name: "Other", Mailbox: "other", Host: "example.com"},
			},
		},
		BodySection: []imapclient.FetchBodySectionBuffer{
			{
				Section: &imaplib.FetchItemBodySection{
					Peek: true,
				},
				Bytes: []byte(
					"Content-Type: text/plain\r\n\r\n" +
						"Original body text.\r\n",
				),
			},
		},
	}
}

func TestReplyMessage_Description(t *testing.T) {
	tool := NewReplyMessage(
		&mockReplyGetter{},
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestReplyMessage_InputSchema(t *testing.T) {
	tool := NewReplyMessage(
		&mockReplyGetter{},
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf(
			"schema type = %v, want object",
			schema["type"],
		)
	}
}

func TestReplyMessage_Reply(t *testing.T) {
	getter := &mockReplyGetter{
		messages: []*imapclient.FetchMessageBuffer{
			sourceMessage(),
		},
	}
	sender := &mockReplySender{config: replyConfig()}
	tool := NewReplyMessage(
		getter,
		sender,
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply",
		"body":    "Thanks for your message!",
	})

	result, err := tool.Execute(nil, args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "Reply sent successfully")

	// Verify the reply was sent to the original sender.
	if len(sender.sentTo) == 0 {
		t.Fatal("no recipients")
	}
	assertContains(
		t,
		sender.sentTo[0],
		"sender@example.com",
	)

	// Verify the composed message contains expected content.
	msgStr := string(sender.sentMsg)
	assertContains(t, msgStr, "Re: Original Subject")
	assertContains(
		t, msgStr, "Thanks for your message!",
	)
	assertContains(
		t, msgStr, "In-Reply-To: <original-msg-id@example.com>",
	)
}

func TestReplyMessage_ReplyAll(t *testing.T) {
	getter := &mockReplyGetter{
		messages: []*imapclient.FetchMessageBuffer{
			sourceMessage(),
		},
	}
	sender := &mockReplySender{config: replyConfig()}
	tool := NewReplyMessage(
		getter,
		sender,
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply_all",
		"body":    "Replying to all!",
	})

	result, err := tool.Execute(nil, args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "Reply-all sent successfully")

	// Should include sender and CC (minus self).
	hasSender := false
	hasOther := false
	for _, rcpt := range sender.sentTo {
		if rcpt == "sender@example.com" {
			hasSender = true
		}
		if rcpt == "other@example.com" {
			hasOther = true
		}
	}
	if !hasSender {
		t.Error("reply_all should include original sender")
	}
	if !hasOther {
		t.Error("reply_all should include CC recipient")
	}

	// Self should be excluded.
	for _, rcpt := range sender.sentTo {
		if rcpt == "me@example.com" {
			t.Error(
				"reply_all should exclude self",
			)
		}
	}
}

func TestReplyMessage_Forward(t *testing.T) {
	getter := &mockReplyGetter{
		messages: []*imapclient.FetchMessageBuffer{
			sourceMessage(),
		},
	}
	sender := &mockReplySender{config: replyConfig()}
	tool := NewReplyMessage(
		getter,
		sender,
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "forward",
		"to":      []string{"forward-to@example.com"},
		"body":    "FYI, see below.",
	})

	result, err := tool.Execute(nil, args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "Forward sent successfully")

	// Verify forwarded to the right recipient.
	if len(sender.sentTo) != 1 ||
		sender.sentTo[0] != "forward-to@example.com" {
		t.Errorf(
			"sentTo = %v, want [forward-to@example.com]",
			sender.sentTo,
		)
	}

	msgStr := string(sender.sentMsg)
	assertContains(t, msgStr, "Fwd: Original Subject")
	assertContains(t, msgStr, "FYI, see below.")
	assertContains(
		t, msgStr, "Forwarded message",
	)

	// Forward should NOT have In-Reply-To.
	assertNotContains(t, msgStr, "In-Reply-To")
}

func TestReplyMessage_ForwardRequiresTo(t *testing.T) {
	tool := NewReplyMessage(
		&mockReplyGetter{
			messages: []*imapclient.FetchMessageBuffer{
				sourceMessage(),
			},
		},
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "forward",
		"body":    "FYI",
	})

	_, err := tool.Execute(nil, args)
	if err == nil {
		t.Fatal(
			"expected error when forward has no To",
		)
	}
	assertContains(t, err.Error(), "to is required")
}

func TestReplyMessage_SubjectPrefixNotDuplicated(t *testing.T) {
	msg := sourceMessage()
	msg.Envelope.Subject = "Re: Already replied"

	getter := &mockReplyGetter{
		messages: []*imapclient.FetchMessageBuffer{msg},
	}
	sender := &mockReplySender{config: replyConfig()}
	tool := NewReplyMessage(
		getter,
		sender,
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply",
		"body":    "Another reply",
	})

	_, err := tool.Execute(nil, args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	msgStr := string(sender.sentMsg)
	// Should not be "Re: Re: Already replied".
	assertContains(t, msgStr, "Re: Already replied")
	assertNotContains(
		t, msgStr, "Re: Re: Already replied",
	)
}

func TestReplyMessage_MissingAccount(t *testing.T) {
	tool := NewReplyMessage(
		&mockReplyGetter{},
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply",
		"body":    "test",
	})

	_, err := tool.Execute(nil, args)
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestReplyMessage_InvalidMode(t *testing.T) {
	tool := NewReplyMessage(
		&mockReplyGetter{},
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "invalid",
		"body":    "test",
	})

	_, err := tool.Execute(nil, args)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestReplyMessage_FetchError(t *testing.T) {
	getter := &mockReplyGetter{
		fetchErr: errors.New("IMAP error"),
	}
	tool := NewReplyMessage(
		getter,
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply",
		"body":    "test",
	})

	_, err := tool.Execute(nil, args)
	if err == nil {
		t.Fatal("expected error for fetch failure")
	}
	assertContains(
		t, err.Error(), "failed to fetch",
	)
}

func TestReplyMessage_SendError(t *testing.T) {
	getter := &mockReplyGetter{
		messages: []*imapclient.FetchMessageBuffer{
			sourceMessage(),
		},
	}
	sender := &mockReplySender{
		config:  replyConfig(),
		sendErr: errors.New("SMTP error"),
	}
	tool := NewReplyMessage(
		getter,
		sender,
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply",
		"body":    "test",
	})

	_, err := tool.Execute(nil, args)
	if err == nil {
		t.Fatal("expected error for send failure")
	}
	assertContains(t, err.Error(), "failed to send")
}

func TestReplyMessage_SMTPNotEnabled(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
				Password: "pass",
			},
		},
	}
	tool := NewReplyMessage(
		&mockReplyGetter{
			messages: []*imapclient.FetchMessageBuffer{
				sourceMessage(),
			},
		},
		&mockReplySender{config: cfg},
		&mockReplySentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"mailbox": "INBOX",
		"uid":     42,
		"mode":    "reply",
		"body":    "test",
	})

	_, err := tool.Execute(nil, args)
	if err == nil {
		t.Fatal("expected error when SMTP not enabled")
	}
	assertContains(t, err.Error(), "not enabled")
}

func TestReplyMessage_InvalidJSON(t *testing.T) {
	tool := NewReplyMessage(
		&mockReplyGetter{},
		&mockReplySender{config: replyConfig()},
		&mockReplySentSaver{},
	)

	_, err := tool.Execute(nil, json.RawMessage(`{bad}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAddPrefix(t *testing.T) {
	tests := []struct {
		prefix  string
		subject string
		want    string
	}{
		{"Re", "Hello", "Re: Hello"},
		{"Re", "Re: Hello", "Re: Hello"},
		{"Re", "re: hello", "re: hello"},
		{"Fwd", "Hello", "Fwd: Hello"},
		{"Fwd", "Fwd: Hello", "Fwd: Hello"},
		{"Re", "", "Re: "},
	}

	for _, tt := range tests {
		got := addPrefix(tt.prefix, tt.subject)
		if got != tt.want {
			t.Errorf(
				"addPrefix(%q, %q) = %q, want %q",
				tt.prefix,
				tt.subject,
				got,
				tt.want,
			)
		}
	}
}

func TestValidateReplyParams(t *testing.T) {
	tests := []struct {
		name    string
		account string
		mailbox string
		mode    string
		body    string
		to      []string
		wantErr string
	}{
		{
			name:    "valid reply",
			account: "test",
			mailbox: "INBOX",
			mode:    "reply",
			body:    "body",
		},
		{
			name:    "missing account",
			mailbox: "INBOX",
			mode:    "reply",
			body:    "body",
			wantErr: "account is required",
		},
		{
			name:    "missing mailbox",
			account: "test",
			mode:    "reply",
			body:    "body",
			wantErr: "mailbox is required",
		},
		{
			name:    "missing mode",
			account: "test",
			mailbox: "INBOX",
			body:    "body",
			wantErr: "mode is required",
		},
		{
			name:    "invalid mode",
			account: "test",
			mailbox: "INBOX",
			mode:    "bogus",
			body:    "body",
			wantErr: "mode must be",
		},
		{
			name:    "missing body",
			account: "test",
			mailbox: "INBOX",
			mode:    "reply",
			wantErr: "body is required",
		},
		{
			name:    "forward without to",
			account: "test",
			mailbox: "INBOX",
			mode:    "forward",
			body:    "fwd",
			wantErr: "to is required",
		},
		{
			name:    "forward with to",
			account: "test",
			mailbox: "INBOX",
			mode:    "forward",
			body:    "fwd",
			to:      []string{"to@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReplyParams(
				tt.account,
				tt.mailbox,
				tt.mode,
				tt.body,
				tt.to,
			)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf(
						"unexpected error: %v",
						err,
					)
				}
			} else {
				if err == nil {
					t.Fatal("expected error")
				}
				assertContains(
					t,
					err.Error(),
					tt.wantErr,
				)
			}
		})
	}
}

func TestFormatReplyResult(t *testing.T) {
	result := formatReplyResult(
		"reply",
		[]string{"to@example.com"},
		nil,
		"Re: Test",
		true,
	)
	assertContains(t, result, "Reply sent successfully")
	assertContains(t, result, "to@example.com")
	assertContains(t, result, "Re: Test")
	assertContains(t, result, "Saved to Sent folder")
}

func TestFormatReplyResult_ReplyAll(t *testing.T) {
	result := formatReplyResult(
		"reply_all",
		[]string{"to@example.com"},
		[]string{"cc@example.com"},
		"Re: Test",
		false,
	)
	assertContains(t, result, "Reply-all sent successfully")
	assertContains(t, result, "CC:")
	assertNotContains(t, result, "Saved to Sent")
}

func TestFormatReplyResult_Forward(t *testing.T) {
	result := formatReplyResult(
		"forward",
		[]string{"fwd@example.com"},
		nil,
		"Fwd: Test",
		false,
	)
	assertContains(t, result, "Forward sent successfully")
}
