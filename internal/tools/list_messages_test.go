package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// mockMessageFetcher is a test double for the messageFetcher
// interface.
type mockMessageFetcher struct {
	selectData *imap.SelectData
	messages   []*imapclient.FetchMessageBuffer
	examineErr error
	fetchErr   error
}

func (m *mockMessageFetcher) ExamineMailbox(
	_, _ string,
) (*imap.SelectData, error) {
	if m.examineErr != nil {
		return nil, m.examineErr
	}
	return m.selectData, nil
}

func (m *mockMessageFetcher) FetchMessages(
	_ string,
	_ imap.SeqSet,
	_ *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.messages, nil
}

func TestListMessages_Description(t *testing.T) {
	tool := NewListMessages(&mockMessageFetcher{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestListMessages_InputSchema(t *testing.T) {
	tool := NewListMessages(&mockMessageFetcher{})

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
	if _, ok := props["mailbox"]; !ok {
		t.Error("schema should have 'mailbox' property")
	}
	if _, ok := props["page"]; !ok {
		t.Error("schema should have 'page' property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a []string")
	}
	if len(required) != 2 {
		t.Fatalf(
			"expected 2 required fields, got %d",
			len(required),
		)
	}

	requiredSet := map[string]bool{}
	for _, r := range required {
		requiredSet[r] = true
	}
	if !requiredSet["account"] || !requiredSet["mailbox"] {
		t.Errorf(
			"required = %v, want [account, mailbox]",
			required,
		)
	}
}

func TestListMessages_Success(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 3},
		messages: []*imapclient.FetchMessageBuffer{
			{
				SeqNum: 1,
				UID:    imap.UID(101),
				Flags:  []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 2, 24, 10, 0, 0, 0,
						time.UTC,
					),
					Subject: "Old message",
					From: []imap.Address{
						{
							Mailbox: "alice",
							Host:    "example.com",
						},
					},
				},
			},
			{
				SeqNum: 2,
				UID:    imap.UID(102),
				Flags:  []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 2, 25, 10, 0, 0, 0,
						time.UTC,
					),
					Subject: "Middle message",
					From: []imap.Address{
						{
							Mailbox: "bob",
							Host:    "work.com",
						},
					},
				},
			},
			{
				SeqNum: 3,
				UID:    imap.UID(103),
				Flags:  []imap.Flag{},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 2, 26, 10, 0, 0, 0,
						time.UTC,
					),
					Subject: "Newest message",
					From: []imap.Address{
						{
							Mailbox: "carol",
							Host:    "test.org",
						},
					},
				},
			},
		},
	}
	tool := NewListMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Messages in gmail/INBOX")
	assertContains(t, result, "3 total")
	assertContains(t, result, "alice@example.com")
	assertContains(t, result, "bob@work.com")
	assertContains(t, result, "carol@test.org")
	assertContains(t, result, "Old message")
	assertContains(t, result, "Newest message")
}

func TestListMessages_NewestFirst(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 2},
		messages: []*imapclient.FetchMessageBuffer{
			{
				SeqNum: 1,
				UID:    imap.UID(1),
				Flags:  []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 1, 1, 0, 0, 0, 0,
						time.UTC,
					),
					Subject: "OLDER",
					From: []imap.Address{
						{
							Mailbox: "a",
							Host:    "b.com",
						},
					},
				},
			},
			{
				SeqNum: 2,
				UID:    imap.UID(2),
				Flags:  []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 2, 1, 0, 0, 0, 0,
						time.UTC,
					),
					Subject: "NEWER",
					From: []imap.Address{
						{
							Mailbox: "c",
							Host:    "d.com",
						},
					},
				},
			},
		},
	}
	tool := NewListMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	newerIdx := strings.Index(result, "NEWER")
	olderIdx := strings.Index(result, "OLDER")
	if newerIdx == -1 || olderIdx == -1 {
		t.Fatal("expected both NEWER and OLDER in result")
	}
	if newerIdx >= olderIdx {
		t.Error("NEWER should appear before OLDER")
	}
}

func TestListMessages_EmptyMailbox(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 0},
	}
	tool := NewListMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	expected := "No messages in gmail/INBOX."
	if result != expected {
		t.Errorf(
			"result = %q, want %q",
			result,
			expected,
		)
	}
}

func TestListMessages_PageOutOfRange(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 50},
	}
	tool := NewListMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX","page":2}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for page out of range",
		)
	}
	assertContains(t, err.Error(), "out of range")
}

func TestListMessages_MissingAccount(t *testing.T) {
	mock := &mockMessageFetcher{}
	tool := NewListMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"mailbox":"INBOX"}`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for missing account",
		)
	}
	assertContains(t, err.Error(), "account is required")
}

func TestListMessages_MissingMailbox(t *testing.T) {
	mock := &mockMessageFetcher{}
	tool := NewListMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{"account":"gmail"}`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for missing mailbox",
		)
	}
	assertContains(t, err.Error(), "mailbox is required")
}

func TestListMessages_InvalidJSON(t *testing.T) {
	mock := &mockMessageFetcher{}
	tool := NewListMessages(mock)

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

func TestListMessages_ExamineError(t *testing.T) {
	mock := &mockMessageFetcher{
		examineErr: fmt.Errorf("connection refused"),
	}
	tool := NewListMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from ExamineMailbox",
		)
	}
	assertContains(t, err.Error(), "connection refused")
}

func TestListMessages_FetchError(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 10},
		fetchErr:   fmt.Errorf("timeout"),
	}
	tool := NewListMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from FetchMessages",
		)
	}
	assertContains(t, err.Error(), "timeout")
}

func TestListMessages_FlagIndicators(t *testing.T) {
	tests := []struct {
		name  string
		flags []imap.Flag
		want  string
	}{
		{
			"unread (no Seen)",
			[]imap.Flag{},
			"[unread]",
		},
		{
			"flagged",
			[]imap.Flag{imap.FlagFlagged},
			"[unread, flagged]",
		},
		{
			"replied",
			[]imap.Flag{
				imap.FlagSeen,
				imap.FlagAnswered,
			},
			"[replied]",
		},
		{
			"draft",
			[]imap.Flag{imap.FlagSeen, imap.FlagDraft},
			"[draft]",
		},
		{
			"deleted",
			[]imap.Flag{
				imap.FlagSeen,
				imap.FlagDeleted,
			},
			"[deleted]",
		},
		{
			"multiple flags",
			[]imap.Flag{imap.FlagFlagged, imap.FlagDraft},
			"[unread, flagged, draft]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockMessageFetcher{
				selectData: &imap.SelectData{
					NumMessages: 1,
				},
				messages: []*imapclient.FetchMessageBuffer{
					{
						SeqNum: 1,
						UID:    imap.UID(1),
						Flags:  tt.flags,
						Envelope: &imap.Envelope{
							Date: time.Date(
								2025, 1, 1,
								0, 0, 0, 0,
								time.UTC,
							),
							Subject: "Test",
							From: []imap.Address{
								{
									Mailbox: "a",
									Host:    "b.com",
								},
							},
						},
					},
				},
			}
			tool := NewListMessages(mock)

			result, err := tool.Execute(
				context.Background(),
				json.RawMessage(
					`{"account":"a",`+
						`"mailbox":"INBOX"}`,
				),
			)
			if err != nil {
				t.Fatalf(
					"Execute() unexpected error: %v",
					err,
				)
			}

			assertContains(t, result, tt.want)
		})
	}
}

func TestListMessages_NoFlagsForSeenOnly(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 1},
		messages: []*imapclient.FetchMessageBuffer{
			{
				SeqNum: 1,
				UID:    imap.UID(1),
				Flags:  []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 1, 1, 0, 0, 0, 0,
						time.UTC,
					),
					Subject: "Read message",
					From: []imap.Address{
						{
							Mailbox: "a",
							Host:    "b.com",
						},
					},
				},
			},
		},
	}
	tool := NewListMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// A seen-only message should have no flag indicators.
	assertNotContains(t, result, "[unread]")
	assertNotContains(t, result, "[flagged]")
	assertNotContains(t, result, "[replied]")
	assertNotContains(t, result, "[draft]")
	assertNotContains(t, result, "[deleted]")
}

func TestListMessages_DefaultPage(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 1},
		messages: []*imapclient.FetchMessageBuffer{
			{
				SeqNum: 1,
				UID:    imap.UID(1),
				Flags:  []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 1, 1, 0, 0, 0, 0,
						time.UTC,
					),
					Subject: "Test",
					From: []imap.Address{
						{
							Mailbox: "a",
							Host:    "b.com",
						},
					},
				},
			},
		},
	}
	tool := NewListMessages(mock)

	// page=0 should default to page 1
	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX","page":0}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "page 1 of 1")
}

func TestListMessages_NilEnvelope(t *testing.T) {
	mock := &mockMessageFetcher{
		selectData: &imap.SelectData{NumMessages: 1},
		messages: []*imapclient.FetchMessageBuffer{
			{
				SeqNum:   1,
				UID:      imap.UID(42),
				Flags:    []imap.Flag{},
				Envelope: nil,
			},
		},
	}
	tool := NewListMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "UID 42")
	assertContains(t, result, "no envelope data")
}

func TestPageRange(t *testing.T) {
	tests := []struct {
		name      string
		total     uint32
		page      int
		size      int
		wantLo    uint32
		wantHi    uint32
		wantPages int
		wantErr   bool
	}{
		{
			"first page of 250",
			250, 1, 100,
			151, 250, 3, false,
		},
		{
			"second page of 250",
			250, 2, 100,
			51, 150, 3, false,
		},
		{
			"last page of 250",
			250, 3, 100,
			1, 50, 3, false,
		},
		{
			"single message",
			1, 1, 100,
			1, 1, 1, false,
		},
		{
			"exactly one page",
			100, 1, 100,
			1, 100, 1, false,
		},
		{
			"page out of range",
			50, 2, 100,
			0, 0, 0, true,
		},
		{
			"page zero",
			50, 0, 100,
			0, 0, 0, true,
		},
		{
			"negative page",
			50, -1, 100,
			0, 0, 0, true,
		},
		{
			"zero page size",
			50, 1, 0,
			0, 0, 0, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lo, hi, pages, err := pageRange(
				tt.total,
				tt.page,
				tt.size,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"pageRange() error = %v, "+
						"wantErr %v",
					err,
					tt.wantErr,
				)
				return
			}
			if !tt.wantErr {
				if lo != tt.wantLo ||
					hi != tt.wantHi ||
					pages != tt.wantPages {
					t.Errorf(
						"pageRange() = "+
							"(%d, %d, %d), "+
							"want (%d, %d, %d)",
						lo, hi, pages,
						tt.wantLo,
						tt.wantHi,
						tt.wantPages,
					)
				}
			}
		})
	}
}
