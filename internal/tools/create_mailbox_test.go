package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// mockMailboxCreator is a test double for the mailboxCreator
// interface.
type mockMailboxCreator struct {
	calls []createMailboxCall
	err   error
}

type createMailboxCall struct {
	account string
	name    string
}

func (m *mockMailboxCreator) CreateMailbox(
	account, name string,
) error {
	m.calls = append(m.calls, createMailboxCall{
		account: account,
		name:    name,
	})
	return m.err
}

func TestCreateMailbox_Description(t *testing.T) {
	tool := NewCreateMailbox(&mockMailboxCreator{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestCreateMailbox_InputSchema(t *testing.T) {
	tool := NewCreateMailbox(&mockMailboxCreator{})

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

func TestCreateMailbox_Success(t *testing.T) {
	mock := &mockMailboxCreator{}
	tool := NewCreateMailbox(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"name":"Work/Projects"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 CreateMailbox call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.account != "gmail" {
		t.Errorf(
			"account = %q, want %q",
			call.account,
			"gmail",
		)
	}
	if call.name != "Work/Projects" {
		t.Errorf(
			"name = %q, want %q",
			call.name,
			"Work/Projects",
		)
	}

	assertContains(t, result, "Created mailbox")
	assertContains(t, result, "Work/Projects")
	assertContains(t, result, "gmail")
}

func TestCreateMailbox_MissingAccount(t *testing.T) {
	mock := &mockMailboxCreator{}
	tool := NewCreateMailbox(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"name":"Work/Projects"}`,
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

func TestCreateMailbox_MissingName(t *testing.T) {
	mock := &mockMailboxCreator{}
	tool := NewCreateMailbox(mock)

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

func TestCreateMailbox_CreateError(t *testing.T) {
	mock := &mockMailboxCreator{
		err: fmt.Errorf("mailbox already exists"),
	}
	tool := NewCreateMailbox(mock)

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
				"CreateMailbox",
		)
	}
	assertContains(
		t, err.Error(), "mailbox already exists",
	)
}

func TestCreateMailbox_InvalidJSON(t *testing.T) {
	mock := &mockMailboxCreator{}
	tool := NewCreateMailbox(mock)

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
