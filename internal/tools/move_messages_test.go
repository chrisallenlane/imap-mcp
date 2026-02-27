package tools

import (
	"context"
	"encoding/json"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// mockMover is a test double for the messageMover interface.
type mockMover struct {
	called bool
	err    error
}

func (m *mockMover) MoveMessages(
	_, _ string,
	_ []imap.UID,
	_ string,
) error {
	m.called = true
	return m.err
}

func TestMoveMessages_WiresCorrectly(t *testing.T) {
	mock := &mockMover{}
	tool := NewMoveMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],`+
				`"destination":"Trash"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !mock.called {
		t.Error("MoveMessages was not called")
	}
}
