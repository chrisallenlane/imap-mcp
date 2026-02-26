package tools

import (
	"fmt"
	"strings"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func FuzzFormatMessage(f *testing.F) {
	// Standard message.
	f.Add(
		uint32(100),
		"Hello World",
		"alice",
		"example.com",
		int64(1710460800),
		true,
		false,
	)

	// Nil envelope (all strings empty).
	f.Add(
		uint32(42),
		"",
		"",
		"",
		int64(0),
		false,
		false,
	)

	// Control characters in subject.
	f.Add(
		uint32(1),
		"Hello\x00World\r\n",
		"user",
		"a.com",
		int64(0),
		false,
		false,
	)

	// Long subject.
	f.Add(
		uint32(1),
		strings.Repeat("A", 1000),
		"x",
		"y.com",
		int64(0),
		true,
		true,
	)

	// Tab in subject.
	f.Add(
		uint32(1),
		"A\tB",
		"x",
		"y.com",
		int64(0),
		false,
		false,
	)

	// Special characters in mailbox/host.
	f.Add(
		uint32(1),
		"Test",
		"a@b",
		"c<d>e",
		int64(0),
		false,
		false,
	)

	f.Fuzz(func(
		t *testing.T,
		uid uint32,
		subject string,
		fromMailbox string,
		fromHost string,
		dateUnix int64,
		hasSeen bool,
		hasFlagged bool,
	) {
		var flags []imap.Flag
		if hasSeen {
			flags = append(flags, imap.FlagSeen)
		}
		if hasFlagged {
			flags = append(
				flags, imap.FlagFlagged,
			)
		}

		msg := &imapclient.FetchMessageBuffer{
			UID:   imap.UID(uid),
			Flags: flags,
		}

		// Test nil envelope path when all strings
		// are empty.
		if subject == "" &&
			fromMailbox == "" &&
			fromHost == "" {
			msg.Envelope = nil
		} else {
			msg.Envelope = &imap.Envelope{
				Date:    time.Unix(dateUnix, 0),
				Subject: subject,
				From: []imap.Address{
					{
						Mailbox: fromMailbox,
						Host:    fromHost,
					},
				},
			}
		}

		var b strings.Builder
		formatMessage(&b, msg)
		out := b.String()

		// Output must contain the UID.
		uidStr := fmt.Sprintf("%d", uid)
		if !strings.Contains(out, uidStr) {
			t.Errorf(
				"output missing UID %s:\n%s",
				uidStr,
				out,
			)
		}

		if msg.Envelope == nil {
			if !strings.Contains(
				out,
				"no envelope data",
			) {
				t.Errorf(
					"nil envelope output "+
						"missing marker:\n%s",
					out,
				)
			}
		} else {
			if !strings.Contains(out, subject) {
				t.Errorf(
					"output missing subject "+
						"%q:\n%s",
					subject,
					out,
				)
			}
		}
	})
}
