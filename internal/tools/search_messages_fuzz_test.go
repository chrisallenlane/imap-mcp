package tools

import (
	"fmt"
	"strings"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func FuzzBuildCriteria(f *testing.F) {
	// Seeds from existing TestBuildCriteria table.
	//
	// Fields: from, to, subject, body,
	//         since, before,
	//         flagged, hasFlagged, seen, hasSeen
	f.Add(
		"", "", "", "",
		"", "",
		false, false, false, false,
	)
	f.Add(
		"alice", "", "", "",
		"", "",
		false, false, false, false,
	)
	f.Add(
		"", "bob", "", "",
		"", "",
		false, false, false, false,
	)
	f.Add(
		"", "", "meeting", "",
		"", "",
		false, false, false, false,
	)
	f.Add(
		"", "", "", "hello",
		"", "",
		false, false, false, false,
	)
	f.Add(
		"", "", "", "",
		"2025-02-01", "",
		false, false, false, false,
	)
	f.Add(
		"", "", "", "",
		"", "2025-03-01",
		false, false, false, false,
	)
	f.Add(
		"", "", "", "",
		"", "",
		true, true, false, false,
	)
	f.Add(
		"", "", "", "",
		"", "",
		false, true, false, false,
	)
	f.Add(
		"", "", "", "",
		"", "",
		false, false, true, true,
	)
	f.Add(
		"", "", "", "",
		"", "",
		false, false, false, true,
	)

	// Invalid dates.
	f.Add(
		"", "", "", "",
		"bad-date", "",
		false, false, false, false,
	)
	f.Add(
		"", "", "", "",
		"", "01/02/2025",
		false, false, false, false,
	)
	f.Add(
		"", "", "", "",
		"not-a-date", "",
		false, false, false, false,
	)

	// IMAP special characters.
	f.Add(
		"*", "%", "\"", "\\",
		"", "",
		false, false, false, false,
	)
	f.Add(
		"\r\n", "", "", "",
		"", "",
		false, false, false, false,
	)

	// Valid date boundaries.
	f.Add(
		"", "", "", "",
		"2025-12-31", "2025-01-01",
		false, false, false, false,
	)

	f.Fuzz(func(
		t *testing.T,
		from, to, subject, body string,
		since, before string,
		flagged, hasFlagged bool,
		seen, hasSeen bool,
	) {
		// Map hasFlagged/hasSeen to *bool pointers.
		var pFlagged *bool
		if hasFlagged {
			pFlagged = &flagged
		}
		var pSeen *bool
		if hasSeen {
			pSeen = &seen
		}

		criteria, err := buildCriteria(
			from, to, subject, body,
			since, before,
			pFlagged, pSeen,
		)

		// Must never panic (implicit).

		// All-empty with nil bools must error.
		allEmpty := from == "" &&
			to == "" &&
			subject == "" &&
			body == "" &&
			since == "" &&
			before == "" &&
			pFlagged == nil &&
			pSeen == nil
		if allEmpty {
			if err == nil {
				t.Fatal(
					"expected error " +
						"when no criteria",
				)
			}
			return
		}

		if err != nil {
			return
		}

		// Criteria must be non-nil on success.
		if criteria == nil {
			t.Fatal("criteria is nil on success")
		}

		// Header checks.
		if from != "" {
			assertHeader(
				t, criteria, "From", from,
			)
		}
		if to != "" {
			assertHeader(
				t, criteria, "To", to,
			)
		}
		if subject != "" {
			assertHeader(
				t, criteria, "Subject", subject,
			)
		}
	})
}

// assertHeader checks that criteria.Header contains an
// entry with the given key and value.
func assertHeader(
	t *testing.T,
	c *imap.SearchCriteria,
	key, value string,
) {
	t.Helper()
	for _, h := range c.Header {
		if h.Key == key && h.Value == value {
			return
		}
	}
	t.Errorf(
		"missing header %s=%q in criteria",
		key, value,
	)
}

func FuzzFormatSearchResults(f *testing.F) {
	// No messages, not capped.
	f.Add("work", "INBOX", 0, 0, false)

	// Single match.
	f.Add("work", "INBOX", 1, 1, false)

	// Multiple matches, not capped.
	f.Add("gmail", "Sent", 5, 5, false)

	// Capped results.
	f.Add("gmail", "INBOX", 3, 150, true)

	// Edge: capped with exactly maxSearchResults.
	f.Add("acct", "Archive", 100, 100, false)

	// Unicode in account/mailbox names.
	f.Add("日本語", "受信箱", 2, 2, false)

	// Special characters.
	f.Add("a<b>", "mail/sub", 1, 500, true)

	f.Fuzz(func(
		t *testing.T,
		account, mailbox string,
		msgCount, totalMatches int,
		capped bool,
	) {
		// Clamp to reasonable range.
		if msgCount < 0 {
			msgCount = 0
		}
		if msgCount > 100 {
			msgCount = 100
		}
		if totalMatches < msgCount {
			totalMatches = msgCount
		}

		// Build message slice.
		msgs := make(
			[]*imapclient.FetchMessageBuffer,
			msgCount,
		)
		for i := range msgs {
			msgs[i] = &imapclient.FetchMessageBuffer{
				UID: imap.UID(i + 1),
				Envelope: &imap.Envelope{
					Date: time.Date(
						2025, 1, i+1,
						0, 0, 0, 0,
						time.UTC,
					),
					Subject: fmt.Sprintf(
						"msg %d", i+1,
					),
				},
			}
		}

		out := formatSearchResults(
			account, mailbox,
			msgs, totalMatches, capped,
		)

		// Output must not be empty.
		if out == "" {
			t.Error("output must not be empty")
		}

		// Output must contain account and mailbox.
		if !strings.Contains(out, account) {
			t.Errorf(
				"output missing account %q",
				account,
			)
		}
		if !strings.Contains(out, mailbox) {
			t.Errorf(
				"output missing mailbox %q",
				mailbox,
			)
		}

		// Singular "match" for exactly 1.
		if totalMatches == 1 {
			if !strings.Contains(out, "match") {
				t.Error(
					"expected \"match\" for " +
						"totalMatches=1",
				)
			}
			if strings.Contains(out, "matches") {
				t.Error(
					"expected singular " +
						"\"match\", got " +
						"\"matches\"",
				)
			}
		}

		// Capped output must mention total.
		if capped {
			totalStr := fmt.Sprintf(
				"%d", totalMatches,
			)
			if !strings.Contains(out, totalStr) {
				t.Errorf(
					"capped output missing "+
						"total %s",
					totalStr,
				)
			}
		}
	})
}
