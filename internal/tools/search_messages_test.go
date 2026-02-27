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

// mockMessageSearcher is a test double for the
// messageSearcher interface.
type mockMessageSearcher struct {
	uids      []imap.UID
	messages  []*imapclient.FetchMessageBuffer
	searchErr error
	fetchErr  error
}

func (m *mockMessageSearcher) SearchMessages(
	_, _ string,
	_ *imap.SearchCriteria,
) ([]imap.UID, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return m.uids, nil
}

func (m *mockMessageSearcher) FetchMessagesByUID(
	_, _ string,
	_ []imap.UID,
	_ *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.messages, nil
}

func TestSearchMessages_InputSchema(t *testing.T) {
	assertSchema(
		t,
		NewSearchMessages(&mockMessageSearcher{}).InputSchema(),
		[]string{
			"account", "mailbox", "from", "to",
			"subject", "body", "since", "before",
			"flagged", "seen",
		},
		[]string{"account", "mailbox"},
	)
}

func TestSearchMessages_Success(t *testing.T) {
	mock := &mockMessageSearcher{
		uids: []imap.UID{101, 102, 103},
		messages: []*imapclient.FetchMessageBuffer{
			{
				UID:   imap.UID(101),
				Flags: []imap.Flag{imap.FlagSeen},
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
				UID:   imap.UID(102),
				Flags: []imap.Flag{imap.FlagSeen},
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
				UID:   imap.UID(103),
				Flags: []imap.Flag{},
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
	tool := NewSearchMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"from":"alice"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(
		t, result, "Search results in gmail/INBOX",
	)
	assertContains(t, result, "3 matches, showing all")
	assertContains(t, result, "alice@example.com")
	assertContains(t, result, "bob@work.com")
	assertContains(t, result, "carol@test.org")
}

func TestSearchMessages_NewestFirst(t *testing.T) {
	mock := &mockMessageSearcher{
		uids: []imap.UID{1, 2},
		messages: []*imapclient.FetchMessageBuffer{
			{
				UID:   imap.UID(1),
				Flags: []imap.Flag{imap.FlagSeen},
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
				UID:   imap.UID(2),
				Flags: []imap.Flag{imap.FlagSeen},
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
	tool := NewSearchMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"subject":"test"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	newerIdx := strings.Index(result, "NEWER")
	olderIdx := strings.Index(result, "OLDER")
	if newerIdx == -1 || olderIdx == -1 {
		t.Fatal(
			"expected both NEWER and OLDER in result",
		)
	}
	if newerIdx >= olderIdx {
		t.Error("NEWER should appear before OLDER")
	}
}

func TestSearchMessages_NoCriteria(t *testing.T) {
	mock := &mockMessageSearcher{}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for no criteria",
		)
	}
	assertContains(
		t, err.Error(),
		"at least one search criterion",
	)
}

func TestSearchMessages_MissingAccount(t *testing.T) {
	mock := &mockMessageSearcher{}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"mailbox":"INBOX","from":"test"}`,
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

func TestSearchMessages_MissingMailbox(t *testing.T) {
	mock := &mockMessageSearcher{}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","from":"test"}`,
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

func TestSearchMessages_InvalidJSON(t *testing.T) {
	assertInvalidJSONError(
		t,
		NewSearchMessages(&mockMessageSearcher{}),
	)
}

func TestSearchMessages_ZeroResults(t *testing.T) {
	mock := &mockMessageSearcher{
		uids: []imap.UID{},
	}
	tool := NewSearchMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX",`+
				`"from":"nobody"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	expected := "No messages matching search criteria " +
		"in gmail/INBOX."
	if result != expected {
		t.Errorf(
			"result = %q, want %q",
			result,
			expected,
		)
	}
}

func TestSearchMessages_ValidDate(t *testing.T) {
	mock := &mockMessageSearcher{
		uids: []imap.UID{1},
		messages: []*imapclient.FetchMessageBuffer{
			{
				UID:   imap.UID(1),
				Flags: []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 2, 26, 0, 0, 0, 0,
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
	tool := NewSearchMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"since":"2025-02-01",`+
				`"before":"2025-03-01"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "1 match, showing all")
}

func TestSearchMessages_InvalidSinceDate(t *testing.T) {
	mock := &mockMessageSearcher{}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"since":"not-a-date"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"invalid since date",
		)
	}
	assertContains(t, err.Error(), "invalid since date")
	assertContains(t, err.Error(), "YYYY-MM-DD")
}

func TestSearchMessages_InvalidBeforeDate(t *testing.T) {
	mock := &mockMessageSearcher{}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"before":"2025/02/01"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"invalid before date",
		)
	}
	assertContains(t, err.Error(), "invalid before date")
	assertContains(t, err.Error(), "YYYY-MM-DD")
}

func TestSearchMessages_ExecuteWithMultipleCriteria(t *testing.T) {
	mock := &mockMessageSearcher{
		uids: []imap.UID{1},
		messages: []*imapclient.FetchMessageBuffer{
			{
				UID:   imap.UID(1),
				Flags: []imap.Flag{imap.FlagSeen},
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 2, 15, 0, 0, 0, 0,
						time.UTC,
					),
					Subject: "Combined search",
					From: []imap.Address{
						{
							Mailbox: "alice",
							Host:    "example.com",
						},
					},
				},
			},
		},
	}
	tool := NewSearchMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"from":"alice",`+
				`"subject":"meeting",`+
				`"since":"2025-02-01",`+
				`"seen":true}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Combined search")
	assertContains(t, result, "1 match, showing all")
}

func TestSearchMessages_CapAt100(t *testing.T) {
	// Create 150 UIDs.
	uids := make([]imap.UID, 150)
	for i := range uids {
		uids[i] = imap.UID(i + 1)
	}

	// Only the newest 100 UIDs (51-150) will be fetched.
	messages := make(
		[]*imapclient.FetchMessageBuffer,
		100,
	)
	for i := range messages {
		messages[i] = &imapclient.FetchMessageBuffer{
			UID:   imap.UID(i + 51),
			Flags: []imap.Flag{imap.FlagSeen},
			Envelope: &imap.Envelope{
				Date: time.Date(
					2025, 1, 1, 0, 0, 0, 0,
					time.UTC,
				).Add(
					time.Duration(i) * time.Hour,
				),
				Subject: fmt.Sprintf("Msg %d", i+51),
				From: []imap.Address{
					{
						Mailbox: "a",
						Host:    "b.com",
					},
				},
			},
		}
	}

	mock := &mockMessageSearcher{
		uids:     uids,
		messages: messages,
	}
	tool := NewSearchMessages(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"from":"a"}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "showing 100 of 150")
	assertContains(
		t, result,
		"150 total matches",
	)
}

func TestSearchMessages_SearchError(t *testing.T) {
	mock := &mockMessageSearcher{
		searchErr: fmt.Errorf("connection refused"),
	}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"from":"test"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from search",
		)
	}
	assertContains(t, err.Error(), "connection refused")
}

func TestSearchMessages_FetchError(t *testing.T) {
	mock := &mockMessageSearcher{
		uids:     []imap.UID{1},
		fetchErr: fmt.Errorf("timeout"),
	}
	tool := NewSearchMessages(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"from":"test"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from fetch",
		)
	}
	assertContains(t, err.Error(), "timeout")
}

func TestBuildCriteria(t *testing.T) {
	boolTrue := true
	boolFalse := false

	tests := []struct {
		name    string
		from    string
		to      string
		subject string
		body    string
		since   string
		before  string
		flagged *bool
		seen    *bool
		wantErr bool
		check   func(t *testing.T, c *imap.SearchCriteria)
	}{
		{
			name:    "no criteria",
			wantErr: true,
		},
		{
			name: "from only",
			from: "alice",
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Header) != 1 {
					t.Fatalf(
						"expected 1 header, got %d",
						len(c.Header),
					)
				}
				if c.Header[0].Key != "From" {
					t.Errorf(
						"header key = %q, want From",
						c.Header[0].Key,
					)
				}
				if c.Header[0].Value != "alice" {
					t.Errorf(
						"header value = %q, "+
							"want alice",
						c.Header[0].Value,
					)
				}
			},
		},
		{
			name: "to only",
			to:   "bob",
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Header) != 1 ||
					c.Header[0].Key != "To" {
					t.Error("expected To header")
				}
			},
		},
		{
			name:    "subject only",
			subject: "meeting",
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Header) != 1 ||
					c.Header[0].Key != "Subject" {
					t.Error("expected Subject header")
				}
			},
		},
		{
			name: "body only",
			body: "hello",
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Body) != 1 ||
					c.Body[0] != "hello" {
					t.Error("expected body criterion")
				}
			},
		},
		{
			name:  "since date",
			since: "2025-02-01",
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				expected := time.Date(
					2025, 2, 1, 0, 0, 0, 0,
					time.UTC,
				)
				if !c.Since.Equal(expected) {
					t.Errorf(
						"since = %v, want %v",
						c.Since,
						expected,
					)
				}
			},
		},
		{
			name:   "before date",
			before: "2025-03-01",
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				expected := time.Date(
					2025, 3, 1, 0, 0, 0, 0,
					time.UTC,
				)
				if !c.Before.Equal(expected) {
					t.Errorf(
						"before = %v, want %v",
						c.Before,
						expected,
					)
				}
			},
		},
		{
			name:    "flagged true",
			flagged: &boolTrue,
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Flag) != 1 ||
					c.Flag[0] != imap.FlagFlagged {
					t.Error(
						"expected FlagFlagged in Flag",
					)
				}
			},
		},
		{
			name:    "flagged false",
			flagged: &boolFalse,
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.NotFlag) != 1 ||
					c.NotFlag[0] !=
						imap.FlagFlagged {
					t.Error(
						"expected FlagFlagged " +
							"in NotFlag",
					)
				}
			},
		},
		{
			name: "seen true",
			seen: &boolTrue,
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Flag) != 1 ||
					c.Flag[0] != imap.FlagSeen {
					t.Error(
						"expected FlagSeen in Flag",
					)
				}
			},
		},
		{
			name: "seen false",
			seen: &boolFalse,
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.NotFlag) != 1 ||
					c.NotFlag[0] !=
						imap.FlagSeen {
					t.Error(
						"expected FlagSeen " +
							"in NotFlag",
					)
				}
			},
		},
		{
			name:    "flagged true and seen false",
			flagged: &boolTrue,
			seen:    &boolFalse,
			check: func(
				t *testing.T,
				c *imap.SearchCriteria,
			) {
				if len(c.Flag) != 1 ||
					c.Flag[0] != imap.FlagFlagged {
					t.Error(
						"expected FlagFlagged " +
							"in Flag",
					)
				}
				if len(c.NotFlag) != 1 ||
					c.NotFlag[0] !=
						imap.FlagSeen {
					t.Error(
						"expected FlagSeen " +
							"in NotFlag",
					)
				}
			},
		},
		{
			name:    "invalid since date",
			since:   "bad-date",
			wantErr: true,
		},
		{
			name:    "invalid before date",
			before:  "01/02/2025",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			criteria, err := buildCriteria(
				tt.from,
				tt.to,
				tt.subject,
				tt.body,
				tt.since,
				tt.before,
				tt.flagged,
				tt.seen,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"buildCriteria() error = %v, "+
						"wantErr %v",
					err,
					tt.wantErr,
				)
				return
			}

			if !tt.wantErr && tt.check != nil {
				tt.check(t, criteria)
			}
		})
	}
}
