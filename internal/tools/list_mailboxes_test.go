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
	statuses  map[string]*imap.StatusData
	statusErr map[string]error
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
		return nil, fmt.Errorf(
			"unknown account: %q",
			account,
		)
	}
	return mbs, nil
}

func (m *mockMailboxLister) MailboxStatus(
	_, mailbox string,
) (*imap.StatusData, error) {
	if m.statusErr != nil {
		if err, ok := m.statusErr[mailbox]; ok {
			return nil, err
		}
	}
	if m.statuses != nil {
		if sd, ok := m.statuses[mailbox]; ok {
			return sd, nil
		}
	}
	// Return zero counts by default.
	zero := uint32(0)
	return &imap.StatusData{
		Mailbox:     mailbox,
		NumMessages: &zero,
		NumUnseen:   &zero,
	}, nil
}

// uint32Ptr is a test helper that returns a pointer to v.
func uint32Ptr(v uint32) *uint32 {
	return &v
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
		statuses: map[string]*imap.StatusData{
			"INBOX": {
				Mailbox:     "INBOX",
				NumMessages: uint32Ptr(5203),
				NumUnseen:   uint32Ptr(12),
			},
			"[Gmail]/Sent Mail": {
				Mailbox:     "[Gmail]/Sent Mail",
				NumMessages: uint32Ptr(4210),
				NumUnseen:   uint32Ptr(0),
			},
			"[Gmail]/Trash": {
				Mailbox:     "[Gmail]/Trash",
				NumMessages: uint32Ptr(47),
				NumUnseen:   uint32Ptr(0),
			},
			"Work": {
				Mailbox:     "Work",
				NumMessages: uint32Ptr(892),
				NumUnseen:   uint32Ptr(0),
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
	assertContains(t, result, "5203 messages, 12 unread")
	assertContains(t, result, "[Gmail]/Sent Mail")
	assertContains(t, result, "(sent)")
	assertContains(t, result, "4210 messages, 0 unread")
	assertContains(t, result, "[Gmail]/Trash")
	assertContains(t, result, "(trash)")
	assertContains(t, result, "47 messages, 0 unread")
	assertContains(t, result, "Work")
	assertContains(t, result, "892 messages, 0 unread")
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
		{
			"archive",
			imap.MailboxAttrArchive,
			"(archive)",
		},
		{
			"drafts",
			imap.MailboxAttrDrafts,
			"(drafts)",
		},
		{
			"flagged",
			imap.MailboxAttrFlagged,
			"(flagged)",
		},
		{"junk", imap.MailboxAttrJunk, "(junk)"},
		{"sent", imap.MailboxAttrSent, "(sent)"},
		{"trash", imap.MailboxAttrTrash, "(trash)"},
		{
			"all",
			imap.MailboxAttrAll,
			"(all mail)",
		},
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
		t.Fatal(
			"Execute() expected error for unknown account",
		)
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

	if !strings.Contains(
		err.Error(),
		"account is required",
	) {
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

	if !strings.Contains(
		err.Error(),
		"account is required",
	) {
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

func TestListMailboxes_NoselectSkipsStatus(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"acct": {
				{
					Mailbox: "[Gmail]",
					Delim:   '/',
					Attrs: []imap.MailboxAttr{
						imap.MailboxAttrNoSelect,
					},
				},
				{Mailbox: "INBOX", Delim: '/'},
			},
		},
		statuses: map[string]*imap.StatusData{
			"INBOX": {
				Mailbox:     "INBOX",
				NumMessages: uint32Ptr(10),
				NumUnseen:   uint32Ptr(3),
			},
		},
		// If status were called for [Gmail], this would
		// cause a failure. The \Noselect check prevents it.
		statusErr: map[string]error{
			"[Gmail]": fmt.Errorf("cannot status noselect"),
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

	assertContains(t, result, "[Gmail]")
	assertContains(t, result, "INBOX")
	assertContains(t, result, "10 messages, 3 unread")

	// [Gmail] should not have any count info.
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "[Gmail]") &&
			!strings.Contains(line, "INBOX") {
			if strings.Contains(line, "message") {
				t.Errorf(
					"\\Noselect mailbox should not "+
						"show counts: %q",
					line,
				)
			}
		}
	}
}

func TestListMailboxes_StatusErrorGraceful(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"acct": {
				{Mailbox: "INBOX", Delim: '/'},
				{Mailbox: "Work", Delim: '/'},
			},
		},
		statuses: map[string]*imap.StatusData{
			"Work": {
				Mailbox:     "Work",
				NumMessages: uint32Ptr(50),
				NumUnseen:   uint32Ptr(5),
			},
		},
		statusErr: map[string]error{
			"INBOX": fmt.Errorf("status timeout"),
		},
	}
	tool := NewListMailboxes(mock)

	// Should not return an error even though INBOX status
	// failed -- partial results are fine.
	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"acct"}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "INBOX")
	assertContains(t, result, "Work")
	assertContains(t, result, "50 messages, 5 unread")

	// INBOX should appear but without counts.
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "INBOX") {
			if strings.Contains(line, "message") {
				t.Errorf(
					"INBOX should not show counts "+
						"when status failed: %q",
					line,
				)
			}
		}
	}
}

func TestListMailboxes_ZeroMessages(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"acct": {
				{Mailbox: "Empty", Delim: '/'},
			},
		},
		statuses: map[string]*imap.StatusData{
			"Empty": {
				Mailbox:     "Empty",
				NumMessages: uint32Ptr(0),
				NumUnseen:   uint32Ptr(0),
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

	assertContains(t, result, "0 messages, 0 unread")
}

func TestListMailboxes_SingularMessage(t *testing.T) {
	mock := &mockMailboxLister{
		mailboxes: map[string][]*imap.ListData{
			"acct": {
				{Mailbox: "Solo", Delim: '/'},
			},
		},
		statuses: map[string]*imap.StatusData{
			"Solo": {
				Mailbox:     "Solo",
				NumMessages: uint32Ptr(1),
				NumUnseen:   uint32Ptr(1),
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

	assertContains(t, result, "1 message, 1 unread")
	if strings.Contains(result, "1 messages") {
		t.Error("should use singular 'message' for count 1")
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
					"specialUseAnnotation() = %q, "+
						"want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name string
		data *imap.StatusData
		want string
	}{
		{
			"nil NumMessages",
			&imap.StatusData{Mailbox: "x"},
			"",
		},
		{
			"zero messages",
			&imap.StatusData{
				NumMessages: uint32Ptr(0),
				NumUnseen:   uint32Ptr(0),
			},
			"0 messages, 0 unread",
		},
		{
			"singular message",
			&imap.StatusData{
				NumMessages: uint32Ptr(1),
				NumUnseen:   uint32Ptr(0),
			},
			"1 message, 0 unread",
		},
		{
			"plural messages",
			&imap.StatusData{
				NumMessages: uint32Ptr(42),
				NumUnseen:   uint32Ptr(7),
			},
			"42 messages, 7 unread",
		},
		{
			"nil NumUnseen",
			&imap.StatusData{
				NumMessages: uint32Ptr(10),
			},
			"10 messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatStatus(tt.data)
			if got != tt.want {
				t.Errorf(
					"formatStatus() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestHasAttr(t *testing.T) {
	tests := []struct {
		name   string
		attrs  []imap.MailboxAttr
		target imap.MailboxAttr
		want   bool
	}{
		{
			"nil attrs",
			nil,
			imap.MailboxAttrNoSelect,
			false,
		},
		{
			"empty attrs",
			[]imap.MailboxAttr{},
			imap.MailboxAttrNoSelect,
			false,
		},
		{
			"contains target",
			[]imap.MailboxAttr{
				imap.MailboxAttrHasChildren,
				imap.MailboxAttrNoSelect,
			},
			imap.MailboxAttrNoSelect,
			true,
		},
		{
			"does not contain target",
			[]imap.MailboxAttr{
				imap.MailboxAttrHasChildren,
			},
			imap.MailboxAttrNoSelect,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAttr(tt.attrs, tt.target)
			if got != tt.want {
				t.Errorf(
					"hasAttr() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}
