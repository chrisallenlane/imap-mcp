package tools

import (
	"strings"
	"testing"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// truncationSuffix is the string appended when the body
// exceeds maxBodySize.
const truncationSuffix = "\n\n[body truncated at 1 MB]"

func FuzzParseBody(f *testing.F) {
	// Seed corpus: structured messages.
	f.Add(makeRawMessage(
		"text/plain", "Hello, world!",
	))
	f.Add(makeRawMessage(
		"text/html",
		"<html><body>Hello</body></html>",
	))
	f.Add(makeMultipartMessage())
	f.Add(makeMultiAttachmentMessage())

	// Empty input.
	f.Add([]byte{})

	// Minimal valid RFC 2822: headers only, no body.
	f.Add([]byte(
		"From: a@b.com\r\n" +
			"Subject: minimal\r\n" +
			"\r\n",
	))

	// Deeply nested multipart boundaries.
	f.Add([]byte(
		"Content-Type: multipart/mixed; " +
			"boundary=\"outer\"\r\n" +
			"\r\n" +
			"--outer\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"mid\"\r\n" +
			"\r\n" +
			"--mid\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"inner\"\r\n" +
			"\r\n" +
			"--inner\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"deep\r\n" +
			"--inner--\r\n" +
			"--mid--\r\n" +
			"--outer--\r\n",
	))

	// Unknown charset to exercise IsUnknownCharset path.
	f.Add([]byte(
		"Content-Type: text/plain; " +
			"charset=\"x-nonexistent\"\r\n" +
			"\r\n" +
			"charset body\r\n",
	))

	f.Fuzz(func(t *testing.T, data []byte) {
		parsed, err := parseBody(data)
		// If parseBody returned an error, the other
		// return values are unspecified; nothing more
		// to check.
		if err != nil {
			return
		}

		// Body must not exceed maxBodySize + truncation
		// suffix length.
		maxLen := maxBodySize + len(truncationSuffix)
		if len(parsed.text) > maxLen {
			t.Errorf(
				"body length %d exceeds max %d",
				len(parsed.text),
				maxLen,
			)
		}

		// If body was truncated, the suffix must be
		// present.
		if len(parsed.text) > maxBodySize {
			if !strings.HasSuffix(
				parsed.text, truncationSuffix,
			) {
				t.Errorf(
					"oversized body missing " +
						"truncation suffix",
				)
			}
		}

		// Attachment metadata invariants.
		for i, att := range parsed.attachments {
			if att.filename == "" {
				t.Errorf(
					"attachment[%d] has empty "+
						"filename; expected "+
						"\"unnamed\"",
					i,
				)
			}
			if att.size < 0 {
				t.Errorf(
					"attachment[%d] has "+
						"negative size %d",
					i,
					att.size,
				)
			}
		}
	})
}

func FuzzFormatFullMessage(f *testing.F) {
	// Plain text message.
	f.Add(
		makeRawMessage("text/plain", "Hello, world!"),
		uint32(1),
		"Test",
		true,
		false,
	)

	// HTML-only message.
	f.Add(
		makeRawMessage(
			"text/html", "<html>Hello</html>",
		),
		uint32(2),
		"HTML",
		true,
		false,
	)

	// Multipart with attachment.
	f.Add(
		makeMultipartMessage(),
		uint32(3),
		"Multi",
		false,
		true,
	)

	// Multiple attachments.
	f.Add(
		makeMultiAttachmentMessage(),
		uint32(4),
		"Attach",
		true,
		false,
	)

	// Empty body bytes.
	f.Add(
		[]byte{},
		uint32(5),
		"Empty",
		true,
		false,
	)

	// Nil-like (zero-length) body.
	f.Add(
		[]byte(nil),
		uint32(6),
		"Nil",
		false,
		false,
	)

	f.Fuzz(func(
		t *testing.T,
		rawBody []byte,
		uid uint32,
		subject string,
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
			Envelope: &imap.Envelope{
				Date: time.Date(
					2025, 1, 1, 0, 0, 0, 0,
					time.UTC,
				),
				Subject: subject,
				From: []imap.Address{
					{
						Mailbox: "test",
						Host:    "example.com",
					},
				},
			},
		}

		if len(rawBody) > 0 {
			msg.BodySection = []imapclient.FetchBodySectionBuffer{
				{
					Section: &imap.FetchItemBodySection{},
					Bytes:   rawBody,
				},
			}
		}

		out, err := formatFullMessage(
			"acct", "INBOX", msg,
		)
		if err != nil {
			if err.Error() == "" {
				t.Error(
					"error message must " +
						"not be empty",
				)
			}
			return
		}

		if out == "" {
			t.Error("output must not be empty")
		}

		if !strings.HasPrefix(out, "Message UID") {
			t.Errorf(
				"output should start with "+
					"\"Message UID\", got:\n%s",
				out,
			)
		}
	})
}
