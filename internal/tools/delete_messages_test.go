package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

type moveDeleteCall struct {
	account     string
	mailbox     string
	uids        []imap.UID
	destMailbox string
}

type expungeCall struct {
	account string
	mailbox string
	uids    []imap.UID
}

type mockDeleter struct {
	trashMailbox string
	findTrashErr error
	moveCalls    []moveDeleteCall
	moveErr      error
	expungeCalls []expungeCall
	expungeErr   error
}

func (m *mockDeleter) FindTrashMailbox(
	_ string,
) (string, error) {
	return m.trashMailbox, m.findTrashErr
}

func (m *mockDeleter) MoveMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	m.moveCalls = append(m.moveCalls, moveDeleteCall{
		account:     account,
		mailbox:     mailbox,
		uids:        uids,
		destMailbox: destMailbox,
	})
	return m.moveErr
}

func (m *mockDeleter) ExpungeMessages(
	account, mailbox string,
	uids []imap.UID,
) error {
	m.expungeCalls = append(m.expungeCalls, expungeCall{
		account: account,
		mailbox: mailbox,
		uids:    uids,
	})
	return m.expungeErr
}

func TestDeleteMessages_Description(t *testing.T) {
	tool := NewDeleteMessages(&mockDeleter{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestDeleteMessages_InputSchema(t *testing.T) {
	tool := NewDeleteMessages(&mockDeleter{})

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
		"account", "mailbox", "uids", "permanent",
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
	expectedRequired := []string{
		"account", "mailbox", "uids",
	}
	for _, r := range expectedRequired {
		if !requiredSet[r] {
			t.Errorf(
				"required should contain %q, "+
					"got %v",
				r,
				required,
			)
		}
	}

	if requiredSet["permanent"] {
		t.Error(
			"permanent should not be required",
		)
	}
}

func TestDeleteMessages_SafeDelete(t *testing.T) {
	mock := &mockDeleter{
		trashMailbox: "Trash",
	}
	tool := NewDeleteMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201,5202]}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Verify FindTrashMailbox was called
	// (implicitly via trashMailbox being used)

	// Verify MoveMessages was called correctly
	if len(mock.moveCalls) != 1 {
		t.Fatalf(
			"expected 1 MoveMessages call, got %d",
			len(mock.moveCalls),
		)
	}

	call := mock.moveCalls[0]
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
	if len(call.uids) != 2 {
		t.Errorf(
			"uids count = %d, want 2",
			len(call.uids),
		)
	}
	if call.destMailbox != "Trash" {
		t.Errorf(
			"destMailbox = %q, want %q",
			call.destMailbox,
			"Trash",
		)
	}

	// Verify no ExpungeMessages calls
	if len(mock.expungeCalls) != 0 {
		t.Errorf(
			"expected 0 ExpungeMessages calls, got %d",
			len(mock.expungeCalls),
		)
	}

	assertContains(
		t, result, "Moved 2 message(s) to Trash",
	)
	assertContains(t, result, "gmail")
	assertContains(t, result, "From: INBOX")
	assertContains(t, result, "5201, 5202")
}

func TestDeleteMessages_PermanentDelete(t *testing.T) {
	mock := &mockDeleter{}
	tool := NewDeleteMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[5201],`+
				`"permanent":true}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Verify ExpungeMessages was called
	if len(mock.expungeCalls) != 1 {
		t.Fatalf(
			"expected 1 ExpungeMessages call, got %d",
			len(mock.expungeCalls),
		)
	}

	call := mock.expungeCalls[0]
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
	if len(call.uids) != 1 {
		t.Errorf(
			"uids count = %d, want 1",
			len(call.uids),
		)
	}

	// Verify no FindTrashMailbox or MoveMessages calls
	if len(mock.moveCalls) != 0 {
		t.Errorf(
			"expected 0 MoveMessages calls, got %d",
			len(mock.moveCalls),
		)
	}

	assertContains(
		t, result,
		"Permanently deleted 1 message(s)",
	)
	assertContains(t, result, "gmail/INBOX")
	assertContains(t, result, "5201")
	assertContains(t, result, "WARNING")
}

func TestDeleteMessages_EmptyUIDs(t *testing.T) {
	mock := &mockDeleter{}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[]}`,
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

func TestDeleteMessages_MissingAccount(t *testing.T) {
	mock := &mockDeleter{}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"mailbox":"INBOX","uids":[1]}`,
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

func TestDeleteMessages_MissingMailbox(t *testing.T) {
	mock := &mockDeleter{}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","uids":[1]}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"missing mailbox",
		)
	}
	assertContains(
		t, err.Error(), "mailbox is required",
	)
}

func TestDeleteMessages_NoTrashMailbox(t *testing.T) {
	mock := &mockDeleter{
		findTrashErr: fmt.Errorf("no trash found"),
	}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1]}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error when " +
				"no trash mailbox found",
		)
	}
	assertContains(
		t, err.Error(),
		"no trash mailbox found",
	)
	assertContains(
		t, err.Error(),
		"permanent: true",
	)
}

func TestDeleteMessages_AlreadyInTrash(t *testing.T) {
	mock := &mockDeleter{
		trashMailbox: "Trash",
	}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"Trash",`+
				`"uids":[1]}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error when " +
				"already in Trash",
		)
	}
	assertContains(
		t, err.Error(),
		"already in Trash",
	)
	assertContains(
		t, err.Error(),
		"permanent: true",
	)
}

func TestDeleteMessages_MoveError(t *testing.T) {
	mock := &mockDeleter{
		trashMailbox: "Trash",
		moveErr:      fmt.Errorf("connection refused"),
	}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1]}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from " +
				"MoveMessages",
		)
	}
	assertContains(
		t, err.Error(), "connection refused",
	)
}

func TestDeleteMessages_ExpungeError(t *testing.T) {
	mock := &mockDeleter{
		expungeErr: fmt.Errorf("server error"),
	}
	tool := NewDeleteMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"uids":[1],`+
				`"permanent":true}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from " +
				"ExpungeMessages",
		)
	}
	assertContains(
		t, err.Error(), "server error",
	)
}

func TestDeleteMessages_InvalidJSON(t *testing.T) {
	mock := &mockDeleter{}
	tool := NewDeleteMessages(mock)

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
