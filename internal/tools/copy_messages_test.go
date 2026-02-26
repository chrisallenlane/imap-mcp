package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// mockCopier is a test double for the messageCopier interface.
type mockCopier struct {
	calls []copyCall
	err   error
}

type copyCall struct {
	account     string
	mailbox     string
	uids        []imap.UID
	destMailbox string
}

func (m *mockCopier) CopyMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	m.calls = append(m.calls, copyCall{
		account:     account,
		mailbox:     mailbox,
		uids:        uids,
		destMailbox: destMailbox,
	})
	return m.err
}

func TestCopyMessages_Description(t *testing.T) {
	tool := NewCopyMessages(&mockCopier{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestCopyMessages_InputSchema(t *testing.T) {
	tool := NewCopyMessages(&mockCopier{})

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

	expectedProps := []string{
		"account", "mailbox", "uids", "destination",
	}
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

func TestCopyMessages_Success(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201,5202,5203],`+
				`"destination":"Work/Important"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 CopyMessages call, got %d",
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
	if call.mailbox != "INBOX" {
		t.Errorf(
			"mailbox = %q, want %q",
			call.mailbox,
			"INBOX",
		)
	}
	if len(call.uids) != 3 {
		t.Errorf(
			"uids count = %d, want 3",
			len(call.uids),
		)
	}
	if call.destMailbox != "Work/Important" {
		t.Errorf(
			"destMailbox = %q, want %q",
			call.destMailbox,
			"Work/Important",
		)
	}

	assertContains(
		t, result, "Copied 3 message(s)",
	)
	assertContains(t, result, "gmail/INBOX")
	assertContains(t, result, "Work/Important")
	assertContains(t, result, "5201, 5202, 5203")
}

func TestCopyMessages_EmptyUIDs(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[],`+
				`"destination":"Trash"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for empty UIDs",
		)
	}
	assertContains(
		t, err.Error(), "uids must not be empty",
	)
}

func TestCopyMessages_MissingAccount(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"mailbox":"INBOX",`+
				`"uids":[1],`+
				`"destination":"Trash"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"missing account",
		)
	}
	assertContains(t, err.Error(), "account is required")
}

func TestCopyMessages_MissingMailbox(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"uids":[1],`+
				`"destination":"Trash"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"missing mailbox",
		)
	}
	assertContains(t, err.Error(), "mailbox is required")
}

func TestCopyMessages_MissingDestination(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1]}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"missing destination",
		)
	}
	assertContains(
		t, err.Error(), "destination is required",
	)
}

func TestCopyMessages_DestinationEqualsSource(
	t *testing.T,
) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],`+
				`"destination":"INBOX"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error when " +
				"destination equals source",
		)
	}
	assertContains(
		t, err.Error(),
		"destination must differ from source mailbox",
	)
}

func TestCopyMessages_InvalidJSON(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{invalid`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for invalid JSON",
		)
	}
	assertContains(t, err.Error(), "parse")
}

func TestCopyMessages_CopyError(t *testing.T) {
	mock := &mockCopier{
		err: fmt.Errorf("connection refused"),
	}
	tool := NewCopyMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],`+
				`"destination":"Trash"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from " +
				"CopyMessages",
		)
	}
	assertContains(
		t, err.Error(), "connection refused",
	)
}
