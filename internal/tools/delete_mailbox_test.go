package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// mockMailboxDeleter is a test double for the mailboxDeleter
// interface.
type mockMailboxDeleter struct {
	deleteCalls []deleteMailboxCall
	deleteErr   error
	mailboxes   []*imap.ListData
	listErr     error
}

type deleteMailboxCall struct {
	account string
	name    string
}

func (m *mockMailboxDeleter) DeleteMailbox(
	account, name string,
) error {
	m.deleteCalls = append(
		m.deleteCalls,
		deleteMailboxCall{
			account: account,
			name:    name,
		},
	)
	return m.deleteErr
}

func (m *mockMailboxDeleter) ListMailboxes(
	_ string,
) ([]*imap.ListData, error) {
	return m.mailboxes, m.listErr
}

func TestDeleteMailbox_Description(t *testing.T) {
	tool := NewDeleteMailbox(&mockMailboxDeleter{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestDeleteMailbox_InputSchema(t *testing.T) {
	tool := NewDeleteMailbox(&mockMailboxDeleter{})

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf(
			"schema type = %v, want object",
			schema["type"],
		)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}

	expectedProps := []string{"account", "name"}
	for _, p := range expectedProps {
		if _, ok := props[p]; !ok {
			t.Errorf(
				"schema should have %q property",
				p,
			)
		}
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a []string")
	}

	requiredSet := map[string]bool{}
	for _, r := range required {
		requiredSet[r] = true
	}
	for _, r := range expectedProps {
		if !requiredSet[r] {
			t.Errorf(
				"required should contain %q, "+
					"got %v",
				r,
				required,
			)
		}
	}
}

func TestDeleteMailbox_Success(t *testing.T) {
	mock := &mockMailboxDeleter{
		mailboxes: []*imap.ListData{
			{Mailbox: "INBOX"},
			{Mailbox: "Work/Old-Projects"},
		},
	}
	tool := NewDeleteMailbox(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"name":"Work/Old-Projects"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.deleteCalls) != 1 {
		t.Fatalf(
			"expected 1 DeleteMailbox call, got %d",
			len(mock.deleteCalls),
		)
	}

	call := mock.deleteCalls[0]
	if call.account != "gmail" {
		t.Errorf(
			"account = %q, want %q",
			call.account,
			"gmail",
		)
	}
	if call.name != "Work/Old-Projects" {
		t.Errorf(
			"name = %q, want %q",
			call.name,
			"Work/Old-Projects",
		)
	}

	assertContains(t, result, "Deleted mailbox")
	assertContains(t, result, "Work/Old-Projects")
	assertContains(t, result, "gmail")
}

func TestDeleteMailbox_MissingAccount(t *testing.T) {
	mock := &mockMailboxDeleter{}
	tool := NewDeleteMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"name":"Work/Old-Projects"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"missing account",
		)
	}
	assertContains(
		t, err.Error(), "account is required",
	)
}

func TestDeleteMailbox_MissingName(t *testing.T) {
	mock := &mockMailboxDeleter{}
	tool := NewDeleteMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"missing name",
		)
	}
	assertContains(
		t, err.Error(), "name is required",
	)
}

func TestDeleteMailbox_RefuseINBOX(t *testing.T) {
	mock := &mockMailboxDeleter{}
	tool := NewDeleteMailbox(mock)

	// Test exact "INBOX"
	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","name":"INBOX"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for INBOX",
		)
	}
	assertContains(
		t, err.Error(), "cannot delete INBOX",
	)

	// Test case-insensitive "inbox"
	_, err = tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","name":"inbox"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"case-insensitive INBOX",
		)
	}
	assertContains(
		t, err.Error(), "cannot delete INBOX",
	)
}

func TestDeleteMailbox_RefuseSpecialUse(t *testing.T) {
	mock := &mockMailboxDeleter{
		mailboxes: []*imap.ListData{
			{
				Mailbox: "Trash",
				Attrs: []imap.MailboxAttr{
					imap.MailboxAttrTrash,
				},
			},
		},
	}
	tool := NewDeleteMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","name":"Trash"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"special-use mailbox",
		)
	}
	assertContains(
		t, err.Error(),
		"cannot delete mailbox",
	)
	assertContains(
		t, err.Error(),
		"special-use attribute",
	)
	assertContains(
		t, err.Error(),
		`\Trash`,
	)

	// Verify DeleteMailbox was NOT called
	if len(mock.deleteCalls) != 0 {
		t.Errorf(
			"expected 0 DeleteMailbox calls, got %d",
			len(mock.deleteCalls),
		)
	}
}

func TestDeleteMailbox_ListMailboxesError(t *testing.T) {
	mock := &mockMailboxDeleter{
		listErr: fmt.Errorf("connection timeout"),
	}
	tool := NewDeleteMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"name":"Work/Projects"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from " +
				"ListMailboxes",
		)
	}
	assertContains(
		t, err.Error(), "connection timeout",
	)
}

func TestDeleteMailbox_DeleteError(t *testing.T) {
	mock := &mockMailboxDeleter{
		mailboxes: []*imap.ListData{
			{Mailbox: "Work/Projects"},
		},
		deleteErr: fmt.Errorf("permission denied"),
	}
	tool := NewDeleteMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"name":"Work/Projects"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from " +
				"DeleteMailbox",
		)
	}
	assertContains(
		t, err.Error(), "permission denied",
	)
}

func TestDeleteMailbox_InvalidJSON(t *testing.T) {
	mock := &mockMailboxDeleter{}
	tool := NewDeleteMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{invalid`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"invalid JSON",
		)
	}
	assertContains(t, err.Error(), "parse")
}
