package tools

import (
	"strings"
	"testing"
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
		body, attachments, err := parseBody(data)
		// If parseBody returned an error, the other
		// return values are unspecified; nothing more
		// to check.
		if err != nil {
			return
		}

		// Body must not exceed maxBodySize + truncation
		// suffix length.
		maxLen := maxBodySize + len(truncationSuffix)
		if len(body) > maxLen {
			t.Errorf(
				"body length %d exceeds max %d",
				len(body),
				maxLen,
			)
		}

		// If body was truncated, the suffix must be
		// present.
		if len(body) > maxBodySize {
			if !strings.HasSuffix(
				body, truncationSuffix,
			) {
				t.Errorf(
					"oversized body missing " +
						"truncation suffix",
				)
			}
		}

		// Attachment metadata invariants.
		for i, att := range attachments {
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
