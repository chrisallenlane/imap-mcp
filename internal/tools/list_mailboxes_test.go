package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// mockMailboxLister is a test double for the mailboxLister
// interface.
type mockMailboxLister struct {
	mailboxes map[string][]*imap.ListData
	err       error
}

func (m *mockMailboxLister) ListMailboxes(
	account string,
) ([]*imap.ListData, error) {
	if m.err != nil {
		return nil, m.err
	}
	mbs, ok := m.mailboxes[account]
	if !ok {
		return nil, fmt.Errorf("unknown account: %q", account)
	}
	return mbs, nil
}

func TestListMailboxes_Description(t *testing.T) {
	tool := NewListMailboxes(&mockMailboxLister{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestListMailboxes_InputSchema(t *testing.T) {
	tool := NewListMailboxes(&mockMailboxLister{})

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
	if _, ok := props["account"]; !ok {
		t.Error("schema should have 'account' property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a []string")
	}
	if len(required) != 1 || required[0] != "account" {
		t.Errorf(
			"required = %v, want [account]",
			required,
		)
	}
}

func TestListMailboxes_Success(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"gmail": {
				{Mailbox: "INBOX", Delim: '/'},
				{
					Mailbox: "[Gmail]/Sent Mail",
					Delim:   '/',
					Attrs: []imap.MailboxAttr{
						imap.MailboxAttrSent,
					},
				},
				{
					Mailbox: "[Gmail]/Trash",
					Delim:   '/',
					Attrs: []imap.MailboxAttr{
						imap.MailboxAttrTrash,
					},
				},
				{Mailbox: "Work", Delim: '/'},
			},
		},
	}
	tool := NewListMailboxes(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"gmail"}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Mailboxes for gmail:")
	assertContains(t, result, "INBOX")
	assertContains(t, result, "[Gmail]/Sent Mail")
	assertContains(t, result, "(sent)")
	assertContains(t, result, "[Gmail]/Trash")
	assertContains(t, result, "(trash)")
	assertContains(t, result, "Work")
}

func TestListMailboxes_InboxAlwaysFirst(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"test": {
				{Mailbox: "Zebra", Delim: '/'},
				{Mailbox: "Alpha", Delim: '/'},
				{Mailbox: "INBOX", Delim: '/'},
				{Mailbox: "Middle", Delim: '/'},
			},
		},
	}
	tool := NewListMailboxes(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"test"}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	inboxIdx := strings.Index(result, "INBOX")
	alphaIdx := strings.Index(result, "Alpha")
	middleIdx := strings.Index(result, "Middle")
	zebraIdx := strings.Index(result, "Zebra")

	if inboxIdx == -1 {
		t.Fatal("INBOX not found in result")
	}
	if inboxIdx >= alphaIdx {
		t.Error("INBOX should appear before Alpha")
	}
	if alphaIdx >= middleIdx {
		t.Error("Alpha should appear before Middle")
	}
	if middleIdx >= zebraIdx {
		t.Error("Middle should appear before Zebra")
	}
}

func TestListMailboxes_SpecialUseAnnotations(t *testing.T) {
	tests := []struct {
		name  string
		attr  imap.MailboxAttr
		label string
	}{
		{"archive", imap.MailboxAttrArchive, "(archive)"},
		{"drafts", imap.MailboxAttrDrafts, "(drafts)"},
		{"flagged", imap.MailboxAttrFlagged, "(flagged)"},
		{"junk", imap.MailboxAttrJunk, "(junk)"},
		{"sent", imap.MailboxAttrSent, "(sent)"},
		{"trash", imap.MailboxAttrTrash, "(trash)"},
		{"all", imap.MailboxAttrAll, "(all mail)"},
		{
			"important",
			imap.MailboxAttrImportant,
			"(important)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMailboxLister{
				mailboxes: map[string][]*imap.ListData{
					"acct": {
						{
							Mailbox: "TestBox",
							Delim:   '/',
							Attrs: []imap.MailboxAttr{
								tt.attr,
							},
						},
					},
				},
			}
			tool := NewListMailboxes(mock)

			result, err := tool.Execute(
				context.Background(),
				json.RawMessage(
					`{"account":"acct"}`,
				),
			)
			if err != nil {
				t.Fatalf(
					"Execute() unexpected error: %v",
					err,
				)
			}

			assertContains(t, result, tt.label)
		})
	}
}

func TestListMailboxes_NoAnnotationForRegularAttrs(
	t *testing.T,
) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"acct": {
				{
					Mailbox: "Regular",
					Delim:   '/',
					Attrs: []imap.MailboxAttr{
						imap.MailboxAttrHasChildren,
					},
				},
			},
		},
	}
	tool := NewListMailboxes(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"acct"}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Regular")
	if strings.Contains(result, "(") {
		t.Error(
			"result should not contain annotation " +
				"for non-special-use attrs",
		)
	}
}

func TestListMailboxes_EmptyMailboxList(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"empty": {},
		},
	}
	tool := NewListMailboxes(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"empty"}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	expected := "No mailboxes found for empty."
	if result != expected {
		t.Errorf(
			"result = %q, want %q",
			result,
			expected,
		)
	}
}

func TestListMailboxes_UnknownAccount(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{},
	}
	tool := NewListMailboxes(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"nonexistent"}`),
	)
	if err == nil {
		t.Fatal("Execute() expected error for unknown account")
	}

	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf(
			"error should mention account name, got: %v",
			err,
		)
	}
}

func TestListMailboxes_MissingAccount(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{},
	}
	tool := NewListMailboxes(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for missing account",
		)
	}

	if !strings.Contains(err.Error(), "account is required") {
		t.Errorf(
			"error should say account is required, got: %v",
			err,
		)
	}
}

func TestListMailboxes_EmptyAccount(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{},
	}
	tool := NewListMailboxes(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":""}`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for empty account",
		)
	}

	if !strings.Contains(err.Error(), "account is required") {
		t.Errorf(
			"error should say account is required, got: %v",
			err,
		)
	}
}

func TestListMailboxes_InvalidJSON(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{},
	}
	tool := NewListMailboxes(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{invalid`),
	)
	if err == nil {
		t.Fatal("Execute() expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf(
			"error should mention parsing, got: %v",
			err,
		)
	}
}

func TestListMailboxes_CaseInsensitiveInbox(t *testing.T) {
	// IMAP spec says INBOX is case-insensitive.
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"acct": {
				{Mailbox: "Zebra", Delim: '/'},
				{Mailbox: "inbox", Delim: '/'},
				{Mailbox: "Alpha", Delim: '/'},
			},
		},
	}
	tool := NewListMailboxes(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"acct"}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	inboxIdx := strings.Index(result, "inbox")
	alphaIdx := strings.Index(result, "Alpha")
	if inboxIdx >= alphaIdx {
		t.Error(
			"inbox (lowercase) should appear before Alpha",
		)
	}
}

func TestSpecialUseAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		attrs []imap.MailboxAttr
		want  string
	}{
		{
			"no attrs",
			nil,
			"",
		},
		{
			"non-special attr",
			[]imap.MailboxAttr{imap.MailboxAttrHasChildren},
			"",
		},
		{
			"single special attr",
			[]imap.MailboxAttr{imap.MailboxAttrDrafts},
			"drafts",
		},
		{
			"first special wins",
			[]imap.MailboxAttr{
				imap.MailboxAttrHasChildren,
				imap.MailboxAttrSent,
				imap.MailboxAttrAll,
			},
			"sent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := specialUseAnnotation(tt.attrs)
			if got != tt.want {
				t.Errorf(
					"specialUseAnnotation() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
