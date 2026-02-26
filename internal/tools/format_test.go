package tools

import (
	"strings"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func TestFormatFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags []imap.Flag
		want  string
	}{
		{
			"no flags (unread)",
			[]imap.Flag{},
			"unread",
		},
		{
			"seen only (empty)",
			[]imap.Flag{imap.FlagSeen},
			"",
		},
		{
			"flagged without seen",
			[]imap.Flag{imap.FlagFlagged},
			"unread, flagged",
		},
		{
			"seen and answered",
			[]imap.Flag{imap.FlagSeen, imap.FlagAnswered},
			"replied",
		},
		{
			"seen and draft",
			[]imap.Flag{imap.FlagSeen, imap.FlagDraft},
			"draft",
		},
		{
			"seen and deleted",
			[]imap.Flag{imap.FlagSeen, imap.FlagDeleted},
			"deleted",
		},
		{
			"multiple without seen",
			[]imap.Flag{imap.FlagFlagged, imap.FlagDraft},
			"unread, flagged, draft",
		},
		{
			"unrecognized flags silently ignored",
			[]imap.Flag{"$Forwarded", imap.FlagSeen},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFlags(tt.flags)
			if got != tt.want {
				t.Errorf(
					"formatFlags() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestFormatMessage_NilEnvelope(t *testing.T) {
	msg := &imapclient.FetchMessageBuffer{
		UID: imap.UID(42),
	}

	var b strings.Builder
	formatMessage(&b, msg)
	result := b.String()

	assertContains(t, result, "UID 42")
	assertContains(t, result, "no envelope data")
}

func TestFormatMessage_Standard(t *testing.T) {
	msg := &imapclient.FetchMessageBuffer{
		UID:   imap.UID(100),
		Flags: []imap.Flag{imap.FlagSeen},
		Envelope: &imap.Envelope{
			Date: time.Date(
				2025, 3, 15, 0, 0, 0, 0, time.UTC,
			),
			Subject: "Hello World",
			From: []imap.Address{
				{Mailbox: "alice", Host: "example.com"},
			},
		},
	}

	var b strings.Builder
	formatMessage(&b, msg)
	result := b.String()

	assertContains(t, result, "UID 100")
	assertContains(t, result, "2025-03-15")
	assertContains(t, result, "alice@example.com")
	assertContains(t, result, "Hello World")
	// Seen-only message should not have flag brackets.
	if strings.Contains(result, "[") {
		t.Error("seen-only message should not have brackets")
	}
}

func TestFormatMessage_UnknownSender(t *testing.T) {
	msg := &imapclient.FetchMessageBuffer{
		UID:   imap.UID(200),
		Flags: []imap.Flag{imap.FlagSeen},
		Envelope: &imap.Envelope{
			Date: time.Date(
				2025, 1, 1, 0, 0, 0, 0, time.UTC,
			),
			Subject: "No sender",
			From:    []imap.Address{},
		},
	}

	var b strings.Builder
	formatMessage(&b, msg)
	result := b.String()

	assertContains(t, result, "(unknown)")
}

func TestFormatMessage_WithFlags(t *testing.T) {
	msg := &imapclient.FetchMessageBuffer{
		UID:   imap.UID(300),
		Flags: []imap.Flag{imap.FlagFlagged},
		Envelope: &imap.Envelope{
			Date: time.Date(
				2025, 1, 1, 0, 0, 0, 0, time.UTC,
			),
			Subject: "Important",
			From: []imap.Address{
				{Mailbox: "a", Host: "b.com"},
			},
		},
	}

	var b strings.Builder
	formatMessage(&b, msg)
	result := b.String()

	assertContains(t, result, "[")
	assertContains(t, result, "]")
	assertContains(t, result, "unread")
	assertContains(t, result, "flagged")
}

func TestEnvelopeDate(t *testing.T) {
	t.Run("with envelope", func(t *testing.T) {
		expected := time.Date(
			2025, 6, 15, 10, 30, 0, 0, time.UTC,
		)
		msg := &imapclient.FetchMessageBuffer{
			Envelope: &imap.Envelope{Date: expected},
		}
		got := envelopeDate(msg)
		if !got.Equal(expected) {
			t.Errorf(
				"envelopeDate() = %v, want %v",
				got,
				expected,
			)
		}
	})

	t.Run("nil envelope", func(t *testing.T) {
		msg := &imapclient.FetchMessageBuffer{}
		got := envelopeDate(msg)
		if !got.IsZero() {
			t.Errorf(
				"envelopeDate() = %v, want zero time",
				got,
			)
		}
	})
}
