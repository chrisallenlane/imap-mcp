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

// mockFlagSetterSequence returns a different error for
// each successive StoreFlags call.
type mockFlagSetterSequence struct {
	calls []storeFlagsCall
	errs  []error
}

func (m *mockFlagSetterSequence) StoreFlags(
	account, mailbox string,
	uids []imap.UID,
	op imap.StoreFlagsOp,
	flags []imap.Flag,
) error {
	idx := len(m.calls)
	m.calls = append(m.calls, storeFlagsCall{
		account: account,
		mailbox: mailbox,
		uids:    uids,
		op:      op,
		flags:   flags,
	})
	if idx < len(m.errs) {
		return m.errs[idx]
	}
	return nil
}

func TestMarkMessages_InputSchema(t *testing.T) {
	assertSchema(
		t,
		NewMarkMessages(&mockFlagSetter{}).InputSchema(),
		[]string{
			"account", "mailbox", "uids",
			"read", "flagged",
		},
		[]string{"account", "mailbox", "uids"},
	)
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

	var addCall, delCall *storeFlagsCall
	for i := range mock.calls {
		switch mock.calls[i].op {
		case imap.StoreFlagsAdd:
			addCall = &mock.calls[i]
		case imap.StoreFlagsDel:
			delCall = &mock.calls[i]
		}
	}

	if addCall == nil {
		t.Fatal("expected a StoreFlagsAdd call")
	}
	if len(addCall.flags) != 1 ||
		addCall.flags[0] != imap.FlagFlagged {
		t.Errorf(
			"add call flags = %v, "+
				"want [\\Flagged]",
			addCall.flags,
		)
	}

	if delCall == nil {
		t.Fatal("expected a StoreFlagsDel call")
	}
	if len(delCall.flags) != 1 ||
		delCall.flags[0] != imap.FlagSeen {
		t.Errorf(
			"del call flags = %v, want [\\Seen]",
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
	assertInvalidJSONError(
		t,
		NewMarkMessages(&mockFlagSetter{}),
	)
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

func TestMarkMessages_SecondStoreFlagsError(t *testing.T) {
	// First StoreFlags call succeeds, second fails.
	// Use both add and remove flags to trigger two calls.
	mock := &mockFlagSetterSequence{
		errs: []error{
			nil,
			fmt.Errorf("server unavailable"),
		},
	}
	tool := NewMarkMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],`+
				`"read":true,"flagged":false}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from " +
				"second StoreFlags call",
		)
	}

	assertContains(
		t, err.Error(), "server unavailable",
	)

	if len(mock.calls) != 2 {
		t.Errorf(
			"expected 2 StoreFlags calls, got %d",
			len(mock.calls),
		)
	}
}
