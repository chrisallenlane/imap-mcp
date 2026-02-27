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

func TestCreateMailbox_InputSchema(t *testing.T) {
	assertSchema(
		t,
		NewCreateMailbox(&mockMailboxCreator{}).InputSchema(),
		[]string{"account", "name"},
		[]string{"account", "name"},
	)
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
	assertInvalidJSONError(
		t,
		NewCreateMailbox(&mockMailboxCreator{}),
	)
}
