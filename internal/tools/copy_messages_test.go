package tools

import (
	"context"
	"encoding/json"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// mockCopier is a test double for the messageCopier interface.
type mockCopier struct {
	called bool
	err    error
}

func (m *mockCopier) CopyMessages(
	_, _ string,
	_ []imap.UID,
	_ string,
) error {
	m.called = true
	return m.err
}

func TestCopyMessages_WiresCorrectly(t *testing.T) {
	mock := &mockCopier{}
	tool := NewCopyMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],`+
				`"destination":"Archive"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if !mock.called {
		t.Error("CopyMessages was not called")
	}

	assertContains(t, result, "Copied")
}
