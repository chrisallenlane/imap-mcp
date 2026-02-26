package tools

import (
	"testing"

	imap "github.com/emersion/go-imap/v2"
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
