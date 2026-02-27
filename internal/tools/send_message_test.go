package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	imap "github.com/emersion/go-imap/v2"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// mockEmailSender implements emailSender for testing.
type mockEmailSender struct {
	config  *config.Config
	sendErr error

	// Captured call parameters.
	sentAccount string
	sentFrom    string
	sentTo      []string
	sentMsg     []byte
}

func (m *mockEmailSender) Send(
	account, from string,
	to []string,
	msg io.Reader,
) error {
	m.sentAccount = account
	m.sentFrom = from
	m.sentTo = to
	var buf bytes.Buffer
	buf.ReadFrom(msg)
	m.sentMsg = buf.Bytes()
	return m.sendErr
}

func (m *mockEmailSender) Config() *config.Config {
	return m.config
}

// mockSentSaver implements sentSaver for testing.
type mockSentSaver struct {
	sentMailbox   string
	findErr       error
	appendErr     error
	appendCalled  bool
	appendedMsg   []byte
	appendedFlags []imap.Flag
}

func (m *mockSentSaver) FindSentMailbox(
	_ string,
) (string, error) {
	return m.sentMailbox, m.findErr
}

func (m *mockSentSaver) AppendMessage(
	_, _ string,
	msg []byte,
	flags []imap.Flag,
) error {
	m.appendCalled = true
	m.appendedMsg = msg
	m.appendedFlags = flags
	return m.appendErr
}

func smtpEnabledConfig() *config.Config {
	return &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user@example.com",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
			},
		},
	}
}

func TestSendMessage_Description(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestSendMessage_InputSchema(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)
	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf(
			"schema type = %v, want object",
			schema["type"],
		)
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be []string")
	}

	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}

	for _, field := range []string{
		"account",
		"to",
		"subject",
		"body",
	} {
		if !requiredSet[field] {
			t.Errorf(
				"%q should be in required fields",
				field,
			)
		}
	}
}

func TestSendMessage_Success(t *testing.T) {
	sender := &mockEmailSender{config: smtpEnabledConfig()}
	saver := &mockSentSaver{}
	tool := NewSendMessage(sender, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"recipient@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	result, err := tool.Execute(
		context.Background(), args,
	)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "sent successfully")
	assertContains(t, result, "recipient@example.com")
	assertContains(t, result, "Test")

	if sender.sentAccount != "test" {
		t.Errorf(
			"sentAccount = %q, want %q",
			sender.sentAccount,
			"test",
		)
	}
	if sender.sentFrom != "user@example.com" {
		t.Errorf(
			"sentFrom = %q, want %q",
			sender.sentFrom,
			"user@example.com",
		)
	}
	if len(sender.sentTo) != 1 ||
		sender.sentTo[0] != "recipient@example.com" {
		t.Errorf(
			"sentTo = %v, want [recipient@example.com]",
			sender.sentTo,
		)
	}
}

func TestSendMessage_WithSMTPFrom(t *testing.T) {
	cfg := smtpEnabledConfig()
	cfg.Accounts["test"] = config.Account{
		Host:        "imap.example.com",
		Port:        993,
		Username:    "user@example.com",
		Password:    "pass",
		SMTPEnabled: true,
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		SMTPFrom:    "custom@example.com",
	}
	sender := &mockEmailSender{config: cfg}
	tool := NewSendMessage(sender, &mockSentSaver{})

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if sender.sentFrom != "custom@example.com" {
		t.Errorf(
			"sentFrom = %q, want %q",
			sender.sentFrom,
			"custom@example.com",
		)
	}
}

func TestSendMessage_WithCCAndBCC(t *testing.T) {
	sender := &mockEmailSender{config: smtpEnabledConfig()}
	tool := NewSendMessage(sender, &mockSentSaver{})

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"cc":      []string{"cc@example.com"},
		"bcc":     []string{"bcc@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "CC:")
	assertContains(t, result, "BCC:")

	// All recipients should be in the envelope.
	expected := []string{
		"to@example.com",
		"cc@example.com",
		"bcc@example.com",
	}
	if len(sender.sentTo) != len(expected) {
		t.Fatalf(
			"sentTo length = %d, want %d",
			len(sender.sentTo),
			len(expected),
		)
	}
	for i, want := range expected {
		if sender.sentTo[i] != want {
			t.Errorf(
				"sentTo[%d] = %q, want %q",
				i,
				sender.sentTo[i],
				want,
			)
		}
	}
}

func TestSendMessage_SaveSent(t *testing.T) {
	cfg := smtpEnabledConfig()
	acct := cfg.Accounts["test"]
	acct.SaveSent = true
	cfg.Accounts["test"] = acct

	sender := &mockEmailSender{config: cfg}
	saver := &mockSentSaver{sentMailbox: "Sent"}
	tool := NewSendMessage(sender, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !saver.appendCalled {
		t.Error("AppendMessage should have been called")
	}
	assertContains(t, result, "Saved to Sent folder")

	// Verify \Seen flag was set.
	foundSeen := false
	for _, f := range saver.appendedFlags {
		if f == imap.FlagSeen {
			foundSeen = true
			break
		}
	}
	if !foundSeen {
		t.Error(
			"expected \\Seen flag in appended message",
		)
	}
}

func TestSendMessage_SaveSentFindError(t *testing.T) {
	cfg := smtpEnabledConfig()
	acct := cfg.Accounts["test"]
	acct.SaveSent = true
	cfg.Accounts["test"] = acct

	sender := &mockEmailSender{config: cfg}
	saver := &mockSentSaver{
		findErr: errors.New("no sent folder"),
	}
	tool := NewSendMessage(sender, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Send should still succeed.
	assertContains(t, result, "sent successfully")
	// But not saved to Sent.
	assertNotContains(t, result, "Saved to Sent folder")
}

func TestSendMessage_SaveSentAppendError(t *testing.T) {
	cfg := smtpEnabledConfig()
	acct := cfg.Accounts["test"]
	acct.SaveSent = true
	cfg.Accounts["test"] = acct

	sender := &mockEmailSender{config: cfg}
	saver := &mockSentSaver{
		sentMailbox: "Sent",
		appendErr:   errors.New("IMAP error"),
	}
	tool := NewSendMessage(sender, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	// Send should still succeed.
	assertContains(t, result, "sent successfully")
	// But not saved to Sent.
	assertNotContains(t, result, "Saved to Sent folder")
}

func TestSendMessage_SaveSentDisabled(t *testing.T) {
	sender := &mockEmailSender{config: smtpEnabledConfig()}
	saver := &mockSentSaver{sentMailbox: "Sent"}
	tool := NewSendMessage(sender, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if saver.appendCalled {
		t.Error(
			"AppendMessage should not have been called",
		)
	}
	assertNotContains(t, result, "Saved to Sent folder")
}

func TestSendMessage_MissingAccount(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestSendMessage_MissingTo(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"subject": "Test",
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing to")
	}
}

func TestSendMessage_MissingSubject(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing subject")
	}
}

func TestSendMessage_MissingBody(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing body")
	}
}

func TestSendMessage_UnknownAccount(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "nonexistent",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	assertContains(t, err.Error(), "unknown account")
}

func TestSendMessage_SMTPNotEnabled(t *testing.T) {
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
	tool := NewSendMessage(
		&mockEmailSender{config: cfg},
		&mockSentSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when SMTP not enabled")
	}
	assertContains(t, err.Error(), "not enabled")
}

func TestSendMessage_SendError(t *testing.T) {
	sender := &mockEmailSender{
		config:  smtpEnabledConfig(),
		sendErr: errors.New("SMTP connection refused"),
	}
	tool := NewSendMessage(sender, &mockSentSaver{})

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"to@example.com"},
		"subject": "Test",
		"body":    "Hello",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for send failure")
	}
	assertContains(t, err.Error(), "failed to send")
}

func TestSendMessage_InvalidJSON(t *testing.T) {
	tool := NewSendMessage(
		&mockEmailSender{config: smtpEnabledConfig()},
		&mockSentSaver{},
	)

	_, err := tool.Execute(
		context.Background(), json.RawMessage(`{invalid}`),
	)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFormatSendResult(t *testing.T) {
	result := formatSendConfirmation(sendConfirmation{
		Title:       "Message",
		To:          []string{"to@example.com"},
		CC:          []string{"cc@example.com"},
		BCC:         []string{"bcc@example.com"},
		Subject:     "Test Subject",
		Attachments: 1,
		SavedToSent: true,
	})

	assertContains(t, result, "sent successfully")
	assertContains(t, result, "to@example.com")
	assertContains(t, result, "CC:")
	assertContains(t, result, "BCC:")
	assertContains(t, result, "Test Subject")
	assertContains(t, result, "Attachments: 1")
	assertContains(t, result, "Saved to Sent folder")
}

func TestFormatSendResult_Minimal(t *testing.T) {
	result := formatSendConfirmation(sendConfirmation{
		Title:   "Message",
		To:      []string{"to@example.com"},
		Subject: "Test",
	})

	assertContains(t, result, "sent successfully")
	assertNotContains(t, result, "CC:")
	assertNotContains(t, result, "BCC:")
	assertNotContains(t, result, "Attachments:")
	assertNotContains(t, result, "Saved to Sent")
}
