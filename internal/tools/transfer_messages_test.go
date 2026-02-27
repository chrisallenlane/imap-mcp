package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// transferCall records a single transfer operation invocation.
type transferCall struct {
	account     string
	mailbox     string
	uids        []imap.UID
	destMailbox string
}

// mockTransfer records calls and returns a configurable error.
type mockTransfer struct {
	calls []transferCall
	err   error
}

func (m *mockTransfer) transfer(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	m.calls = append(m.calls, transferCall{
		account:     account,
		mailbox:     mailbox,
		uids:        uids,
		destMailbox: destMailbox,
	})
	return m.err
}

func newTestTransferTool(
	mock *mockTransfer,
) *transferTool {
	return newTransferTool(
		"move", "Moved", mock.transfer,
	)
}

func TestTransferTool_Description(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestTransferTool_InputSchema(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_EmptyUIDs(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_MissingAccount(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_MissingMailbox(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_MissingDestination(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_DestinationEqualsSource(
	t *testing.T,
) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_InvalidJSON(t *testing.T) {
	tool := newTestTransferTool(&mockTransfer{})

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

func TestTransferTool_OperationError(t *testing.T) {
	mock := &mockTransfer{
		err: fmt.Errorf("connection refused"),
	}
	tool := newTestTransferTool(mock)

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
			"Execute() expected error from operation",
		)
	}
	assertContains(
		t, err.Error(), "connection refused",
	)
}

func TestTransferTool_Success(t *testing.T) {
	mock := &mockTransfer{}
	tool := newTestTransferTool(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201,5202,5203],`+
				`"destination":"Work/Projects"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 call, got %d",
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
	if call.destMailbox != "Work/Projects" {
		t.Errorf(
			"destMailbox = %q, want %q",
			call.destMailbox,
			"Work/Projects",
		)
	}

	assertContains(
		t, result, "Moved 3 message(s)",
	)
	assertContains(t, result, "gmail/INBOX")
	assertContains(t, result, "Work/Projects")
	assertContains(t, result, "5201, 5202, 5203")
}
