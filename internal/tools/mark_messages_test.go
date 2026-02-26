package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// mockFlagSetter is a test double for the messageFlagSetter
// interface.
type mockFlagSetter struct {
	calls []storeFlagsCall
	err   error
}

type storeFlagsCall struct {
	account string
	mailbox string
	uids    []imap.UID
	op      imap.StoreFlagsOp
	flags   []imap.Flag
}

func (m *mockFlagSetter) StoreFlags(
	account, mailbox string,
	uids []imap.UID,
	op imap.StoreFlagsOp,
	flags []imap.Flag,
) error {
	m.calls = append(m.calls, storeFlagsCall{
		account: account,
		mailbox: mailbox,
		uids:    uids,
		op:      op,
		flags:   flags,
	})
	return m.err
}

func TestMarkMessages_Description(t *testing.T) {
	tool := NewMarkMessages(&mockFlagSetter{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestMarkMessages_InputSchema(t *testing.T) {
	tool := NewMarkMessages(&mockFlagSetter{})

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
		"account", "mailbox", "uids",
		"read", "flagged",
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
	if !requiredSet["account"] ||
		!requiredSet["mailbox"] ||
		!requiredSet["uids"] {
		t.Errorf(
			"required = %v, want "+
				"[account, mailbox, uids]",
			required,
		)
	}
}

func TestMarkMessages_MarkRead(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201,5202],"read":true}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 StoreFlags call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.op != imap.StoreFlagsAdd {
		t.Errorf("op = %v, want StoreFlagsAdd", call.op)
	}
	if len(call.flags) != 1 ||
		call.flags[0] != imap.FlagSeen {
		t.Errorf(
			"flags = %v, want [\\Seen]",
			call.flags,
		)
	}
	if len(call.uids) != 2 {
		t.Errorf(
			"uids count = %d, want 2",
			len(call.uids),
		)
	}

	assertContains(t, result, "Updated 2 message(s)")
	assertContains(t, result, "gmail/INBOX")
	assertContains(t, result, "Added flags: \\Seen")
	assertContains(t, result, "5201, 5202")
}

func TestMarkMessages_MarkUnread(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201],"read":false}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 StoreFlags call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.op != imap.StoreFlagsDel {
		t.Errorf("op = %v, want StoreFlagsDel", call.op)
	}
	if len(call.flags) != 1 ||
		call.flags[0] != imap.FlagSeen {
		t.Errorf(
			"flags = %v, want [\\Seen]",
			call.flags,
		)
	}

	assertContains(t, result, "Updated 1 message(s)")
	assertContains(t, result, "Removed flags: \\Seen")
}

func TestMarkMessages_Flag(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201],"flagged":true}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 StoreFlags call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.op != imap.StoreFlagsAdd {
		t.Errorf("op = %v, want StoreFlagsAdd", call.op)
	}
	if len(call.flags) != 1 ||
		call.flags[0] != imap.FlagFlagged {
		t.Errorf(
			"flags = %v, want [\\Flagged]",
			call.flags,
		)
	}

	assertContains(t, result, "Added flags: \\Flagged")
}

func TestMarkMessages_Unflag(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201],"flagged":false}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 StoreFlags call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.op != imap.StoreFlagsDel {
		t.Errorf("op = %v, want StoreFlagsDel", call.op)
	}
	if len(call.flags) != 1 ||
		call.flags[0] != imap.FlagFlagged {
		t.Errorf(
			"flags = %v, want [\\Flagged]",
			call.flags,
		)
	}

	assertContains(t, result, "Removed flags: \\Flagged")
}

func TestMarkMessages_MultipleFlags(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201],`+
				`"read":false,"flagged":true}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Should make two StoreFlags calls:
	// one to add \Flagged, one to remove \Seen.
	if len(mock.calls) != 2 {
		t.Fatalf(
			"expected 2 StoreFlags calls, got %d",
			len(mock.calls),
		)
	}

	addCall := mock.calls[0]
	if addCall.op != imap.StoreFlagsAdd {
		t.Errorf(
			"first call op = %v, want StoreFlagsAdd",
			addCall.op,
		)
	}
	if len(addCall.flags) != 1 ||
		addCall.flags[0] != imap.FlagFlagged {
		t.Errorf(
			"first call flags = %v, "+
				"want [\\Flagged]",
			addCall.flags,
		)
	}

	delCall := mock.calls[1]
	if delCall.op != imap.StoreFlagsDel {
		t.Errorf(
			"second call op = %v, want StoreFlagsDel",
			delCall.op,
		)
	}
	if len(delCall.flags) != 1 ||
		delCall.flags[0] != imap.FlagSeen {
		t.Errorf(
			"second call flags = %v, want [\\Seen]",
			delCall.flags,
		)
	}

	assertContains(t, result, "Added flags: \\Flagged")
	assertContains(t, result, "Removed flags: \\Seen")
}

func TestMarkMessages_BothAdd(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1,2,3],`+
				`"read":true,"flagged":true}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Both flags added in one call.
	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 StoreFlags call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.op != imap.StoreFlagsAdd {
		t.Errorf("op = %v, want StoreFlagsAdd", call.op)
	}
	if len(call.flags) != 2 {
		t.Errorf(
			"flags count = %d, want 2",
			len(call.flags),
		)
	}

	assertContains(t, result, "Updated 3 message(s)")
	assertContains(t, result, "1, 2, 3")
}

func TestMarkMessages_BothRemove(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201],`+
				`"read":false,"flagged":false}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Both flags removed in one call.
	if len(mock.calls) != 1 {
		t.Fatalf(
			"expected 1 StoreFlags call, got %d",
			len(mock.calls),
		)
	}

	call := mock.calls[0]
	if call.op != imap.StoreFlagsDel {
		t.Errorf("op = %v, want StoreFlagsDel", call.op)
	}
	if len(call.flags) != 2 {
		t.Errorf(
			"flags count = %d, want 2",
			len(call.flags),
		)
	}

	assertContains(t, result, "Removed flags:")
}

func TestMarkMessages_NoFlags(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201]}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for no flags",
		)
	}
	assertContains(
		t, err.Error(),
		"at least one flag parameter",
	)
}

func TestMarkMessages_EmptyUIDs(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[],"read":true}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for empty UIDs",
		)
	}
	assertContains(t, err.Error(), "uids must not be empty")
}

func TestMarkMessages_MissingAccount(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"mailbox":"INBOX",`+
				`"uids":[1],"read":true}`,
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

func TestMarkMessages_MissingMailbox(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"uids":[1],"read":true}`,
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

func TestMarkMessages_InvalidJSON(t *testing.T) {
	mock := &mockFlagSetter{}
	tool := NewMarkMessages(mock)

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

func TestMarkMessages_StoreFlagsError(t *testing.T) {
	mock := &mockFlagSetter{
		err: fmt.Errorf("connection refused"),
	}
	tool := NewMarkMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],"read":true}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from StoreFlags",
		)
	}
	assertContains(t, err.Error(), "connection refused")
}
