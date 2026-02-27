package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	imap "github.com/emersion/go-imap/v2"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// mockDraftConfigProvider implements draftConfigProvider.
type mockDraftConfigProvider struct {
	config *config.Config
}

func (m *mockDraftConfigProvider) Config() *config.Config {
	return m.config
}

// mockDraftSaver implements draftSaver.
type mockDraftSaver struct {
	draftsMailbox string
	findErr       error
	appendErr     error
	appendCalled  bool
	appendedMsg   []byte
	appendedFlags []imap.Flag
}

func (m *mockDraftSaver) FindDraftsMailbox(
	_ string,
) (string, error) {
	return m.draftsMailbox, m.findErr
}

func (m *mockDraftSaver) AppendMessage(
	_, _ string,
	msg []byte,
	flags []imap.Flag,
) error {
	m.appendCalled = true
	m.appendedMsg = msg
	m.appendedFlags = flags
	return m.appendErr
}

func smtpEnabledDraftConfig() *config.Config {
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

func TestSaveDraft_Description(t *testing.T) {
	tool := NewSaveDraft(
		&mockDraftConfigProvider{
			config: smtpEnabledDraftConfig(),
		},
		&mockDraftSaver{},
	)
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestSaveDraft_InputSchema(t *testing.T) {
	tool := NewSaveDraft(
		&mockDraftConfigProvider{
			config: smtpEnabledDraftConfig(),
		},
		&mockDraftSaver{},
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
	if len(required) != 1 || required[0] != "account" {
		t.Errorf(
			"required = %v, want [account]",
			required,
		)
	}
}

func TestSaveDraft_Success(t *testing.T) {
	cfg := &mockDraftConfigProvider{
		config: smtpEnabledDraftConfig(),
	}
	saver := &mockDraftSaver{draftsMailbox: "Drafts"}
	tool := NewSaveDraft(cfg, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"to":      []string{"recipient@example.com"},
		"subject": "Draft Subject",
		"body":    "Draft body text",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "Draft saved")
	assertContains(t, result, "Drafts")
	assertContains(t, result, "recipient@example.com")
	assertContains(t, result, "Draft Subject")

	if !saver.appendCalled {
		t.Error("AppendMessage should have been called")
	}

	// Verify \Draft flag was set.
	foundDraft := false
	for _, f := range saver.appendedFlags {
		if f == imap.FlagDraft {
			foundDraft = true
			break
		}
	}
	if !foundDraft {
		t.Error(
			"expected \\Draft flag in appended message",
		)
	}
}

func TestSaveDraft_MinimalDraft(t *testing.T) {
	cfg := &mockDraftConfigProvider{
		config: smtpEnabledDraftConfig(),
	}
	saver := &mockDraftSaver{draftsMailbox: "Drafts"}
	tool := NewSaveDraft(cfg, saver)

	// Only account is required.
	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
	})

	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	assertContains(t, result, "Draft saved")
	if !saver.appendCalled {
		t.Error("AppendMessage should have been called")
	}
}

func TestSaveDraft_MissingAccount(t *testing.T) {
	tool := NewSaveDraft(
		&mockDraftConfigProvider{
			config: smtpEnabledDraftConfig(),
		},
		&mockDraftSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

func TestSaveDraft_UnknownAccount(t *testing.T) {
	tool := NewSaveDraft(
		&mockDraftConfigProvider{
			config: smtpEnabledDraftConfig(),
		},
		&mockDraftSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "nonexistent",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	assertContains(t, err.Error(), "unknown account")
}

func TestSaveDraft_SMTPNotEnabled(t *testing.T) {
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
	tool := NewSaveDraft(
		&mockDraftConfigProvider{config: cfg},
		&mockDraftSaver{},
	)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when SMTP not enabled")
	}
	assertContains(t, err.Error(), "not enabled")
}

func TestSaveDraft_FindDraftsFails(t *testing.T) {
	cfg := &mockDraftConfigProvider{
		config: smtpEnabledDraftConfig(),
	}
	saver := &mockDraftSaver{
		findErr: errors.New("no drafts folder"),
	}
	tool := NewSaveDraft(cfg, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"body":    "test",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when Drafts not found")
	}
	assertContains(t, err.Error(), "Drafts folder")
}

func TestSaveDraft_AppendFails(t *testing.T) {
	cfg := &mockDraftConfigProvider{
		config: smtpEnabledDraftConfig(),
	}
	saver := &mockDraftSaver{
		draftsMailbox: "Drafts",
		appendErr:     errors.New("IMAP error"),
	}
	tool := NewSaveDraft(cfg, saver)

	args, _ := json.Marshal(map[string]interface{}{
		"account": "test",
		"body":    "test",
	})

	_, err := tool.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when append fails")
	}
	assertContains(t, err.Error(), "failed to save draft")
}

func TestSaveDraft_InvalidJSON(t *testing.T) {
	tool := NewSaveDraft(
		&mockDraftConfigProvider{
			config: smtpEnabledDraftConfig(),
		},
		&mockDraftSaver{},
	)

	_, err := tool.Execute(
		context.Background(), json.RawMessage(`{bad}`),
	)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFormatDraftResult(t *testing.T) {
	result := formatDraftResult(
		"Drafts",
		[]string{"to@example.com"},
		"Test Subject",
	)
	assertContains(t, result, "Draft saved")
	assertContains(t, result, "Drafts")
	assertContains(t, result, "to@example.com")
	assertContains(t, result, "Test Subject")
}

func TestFormatDraftResult_Minimal(t *testing.T) {
	result := formatDraftResult("Drafts", nil, "")
	assertContains(t, result, "Draft saved")
	assertNotContains(t, result, "To:")
	assertNotContains(t, result, "Subject:")
}
