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

// mockMessageGetter is a test double for the messageGetter
// interface.
type mockMessageGetter struct {
	messages []*imapclient.FetchMessageBuffer
	err      error
}

func (m *mockMessageGetter) FetchMessagesByUID(
	_ string,
	_ string,
	_ []imap.UID,
	_ *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.messages, nil
}

// makeRawMessage builds a simple RFC 2822 message.
func makeRawMessage(
	contentType, body string,
) []byte {
	return []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: " + contentType + "\r\n" +
			"\r\n" +
			body,
	)
}

// makeMultipartMessage builds a multipart/mixed RFC 2822
// message with a text part and an attachment.
func makeMultipartMessage() []byte {
	return []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"boundary123\"\r\n" +
			"\r\n" +
			"--boundary123\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"Hello, world!\r\n" +
			"--boundary123\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Disposition: attachment; " +
			"filename=\"test.pdf\"\r\n" +
			"\r\n" +
			"PDFcontent\r\n" +
			"--boundary123--\r\n",
	)
}

// makeMultiAttachmentMessage builds a multipart message with
// multiple attachments.
func makeMultiAttachmentMessage() []byte {
	return []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"b999\"\r\n" +
			"\r\n" +
			"--b999\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"See attached.\r\n" +
			"--b999\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Disposition: attachment; " +
			"filename=\"report.pdf\"\r\n" +
			"\r\n" +
			"PDFdata1234567890\r\n" +
			"--b999\r\n" +
			"Content-Type: image/png\r\n" +
			"Content-Disposition: attachment; " +
			"filename=\"photo.png\"\r\n" +
			"\r\n" +
			"PNGdata\r\n" +
			"--b999--\r\n",
	)
}

// mockMsg builds a FetchMessageBuffer with the given raw
// body bytes and standard envelope data.
func mockMsg(
	uid imap.UID,
	flags []imap.Flag,
	envelope *imap.Envelope,
	rawBody []byte,
) *imapclient.FetchMessageBuffer {
	msg := &imapclient.FetchMessageBuffer{
		UID:      uid,
		Flags:    flags,
		Envelope: envelope,
	}
	if rawBody != nil {
		msg.BodySection = []imapclient.FetchBodySectionBuffer{
			{
				Section: &imap.FetchItemBodySection{},
				Bytes:   rawBody,
			},
		}
	}
	return msg
}

// standardEnvelope returns a typical envelope for tests.
func standardEnvelope() *imap.Envelope {
	return &imap.Envelope{
		Date: time.Date(
			2025, 2, 26, 10, 30, 0, 0,
			time.FixedZone("EST", -5*3600),
		),
		Subject: "Meeting tomorrow",
		From: []imap.Address{
			{
				Name:    "Alice Smith",
				Mailbox: "alice",
				Host:    "example.com",
			},
		},
		To: []imap.Address{
			{
				Mailbox: "chris",
				Host:    "example.com",
			},
		},
	}
}

func TestGetMessage_InputSchema(t *testing.T) {
	assertSchema(
		t,
		NewGetMessage(&mockMessageGetter{}).InputSchema(),
		[]string{"account", "mailbox", "uid"},
		[]string{"account", "mailbox", "uid"},
	)
}

func TestGetMessage_Success(t *testing.T) {
	rawBody := makeRawMessage(
		"text/plain",
		"Hello, world!",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(5201),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"mailbox":"INBOX","uid":5201}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Message UID 5201")
	assertContains(t, result, "gmail/INBOX")
	assertContains(
		t,
		result,
		"Alice Smith <alice@example.com>",
	)
	assertContains(t, result, "chris@example.com")
	assertContains(t, result, "Meeting tomorrow")
	assertContains(t, result, "Hello, world!")
}

func TestGetMessage_UIDAsString(t *testing.T) {
	// Claude sometimes passes UIDs as quoted strings;
	// verify the tool accepts either form.
	rawBody := makeRawMessage(
		"text/plain",
		"Hello, world!",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(5201),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail",`+
				`"mailbox":"INBOX","uid":"5201"}`,
		),
	)
	if err != nil {
		t.Fatalf(
			"Execute() unexpected error: %v", err,
		)
	}

	assertContains(t, result, "Message UID 5201")
}

func TestGetMessage_Multipart(t *testing.T) {
	rawBody := makeMultipartMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":100}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Hello, world!")
	assertContains(t, result, "Attachments:")
	assertContains(t, result, "test.pdf")
	assertContains(t, result, "application/pdf")
}

func TestGetMessage_HTMLFallback(t *testing.T) {
	rawBody := makeRawMessage(
		"text/html",
		"<html><body><p>Hello</p></body></html>",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(200),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":200}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(
		t,
		result,
		"Body (converted from HTML):",
	)
	assertContains(t, result, "Hello")
}

func TestGetMessage_HTMLFallbackWithLinks(t *testing.T) {
	rawBody := makeRawMessage(
		"text/html",
		`<html><body>`+
			`<p>Visit <a href="https://example.com">`+
			`our site</a>.</p>`+
			`</body></html>`,
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(201),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":201}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(
		t,
		result,
		"Body (converted from HTML):",
	)
	assertContains(
		t,
		result,
		"our site (https://example.com)",
	)
}

func TestGetMessage_PlainTextPreferredOverHTML(t *testing.T) {
	// Multipart with both text/plain and text/html.
	rawBody := []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/alternative; " +
			"boundary=\"alt123\"\r\n" +
			"\r\n" +
			"--alt123\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"Plain text version\r\n" +
			"--alt123\r\n" +
			"Content-Type: text/html\r\n" +
			"\r\n" +
			"<p>HTML version</p>\r\n" +
			"--alt123--\r\n",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(202),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":202}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Plain text version")

	// Should NOT show the HTML conversion label.
	if strings.Contains(
		result, "converted from HTML",
	) {
		t.Error(
			"should use plain text, " +
				"not HTML fallback",
		)
	}
}

func TestGetMessage_NoReadableContent(t *testing.T) {
	// A message with only an attachment, no text parts.
	rawBody := []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"notext\"\r\n" +
			"\r\n" +
			"--notext\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Disposition: attachment; " +
			"filename=\"doc.pdf\"\r\n" +
			"\r\n" +
			"PDFdata\r\n" +
			"--notext--\r\n",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(203),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":203}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(
		t, result, "no readable body content",
	)
}

func TestGetMessage_WithAttachments(t *testing.T) {
	rawBody := makeMultiAttachmentMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(300),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":300}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "See attached.")
	assertContains(t, result, "1. report.pdf")
	assertContains(t, result, "application/pdf")
	assertContains(t, result, "2. photo.png")
	assertContains(t, result, "image/png")
}

func TestGetMessage_NilEnvelope(t *testing.T) {
	rawBody := makeRawMessage(
		"text/plain",
		"Hello",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(400),
				[]imap.Flag{imap.FlagSeen},
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":400}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Message UID 400")
	assertContains(t, result, "(no envelope data)")
}

func TestGetMessage_NoBodySection(t *testing.T) {
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(700),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				nil,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":700}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Message UID 700")
	assertContains(
		t, result, "no body data available",
	)
}

func TestGetMessage_MissingAccount(t *testing.T) {
	tool := NewGetMessage(&mockMessageGetter{})

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"mailbox":"INBOX","uid":1}`,
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

func TestGetMessage_MissingMailbox(t *testing.T) {
	tool := NewGetMessage(&mockMessageGetter{})

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","uid":1}`,
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

func TestGetMessage_MissingUID(t *testing.T) {
	tool := NewGetMessage(&mockMessageGetter{})

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"gmail","mailbox":"INBOX"}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for missing uid",
		)
	}
	assertContains(t, err.Error(), "uid is required")
}

func TestGetMessage_InvalidJSON(t *testing.T) {
	assertInvalidJSONError(
		t,
		NewGetMessage(&mockMessageGetter{}),
	)
}

func TestGetMessage_NotFound(t *testing.T) {
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{},
	}
	tool := NewGetMessage(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":999}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for not found",
		)
	}
	assertContains(t, err.Error(), "message not found")
}

func TestGetMessage_FetchError(t *testing.T) {
	mock := &mockMessageGetter{
		err: fmt.Errorf("connection timeout"),
	}
	tool := NewGetMessage(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":1}`,
		),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error from fetcher",
		)
	}
	assertContains(t, err.Error(), "connection timeout")
}

func TestGetMessage_Flags(t *testing.T) {
	tests := []struct {
		name  string
		flags []imap.Flag
		want  string
	}{
		{
			"seen only - no flag line",
			[]imap.Flag{imap.FlagSeen},
			"",
		},
		{
			"unread",
			[]imap.Flag{},
			"unread",
		},
		{
			"flagged and unread",
			[]imap.Flag{imap.FlagFlagged},
			"unread, flagged",
		},
		{
			"seen and replied",
			[]imap.Flag{
				imap.FlagSeen,
				imap.FlagAnswered,
			},
			"replied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawBody := makeRawMessage(
				"text/plain",
				"Test body",
			)
			mock := &mockMessageGetter{
				messages: []*imapclient.FetchMessageBuffer{
					mockMsg(
						imap.UID(1),
						tt.flags,
						standardEnvelope(),
						rawBody,
					),
				},
			}
			tool := NewGetMessage(mock)

			result, err := tool.Execute(
				context.Background(),
				json.RawMessage(
					`{"account":"a",`+
						`"mailbox":"INBOX",`+
						`"uid":1}`,
				),
			)
			if err != nil {
				t.Fatalf(
					"Execute() unexpected error: %v",
					err,
				)
			}

			if tt.want == "" {
				if strings.Contains(
					result,
					"Flags:",
				) {
					t.Error(
						"seen-only message should " +
							"not have Flags line",
					)
				}
			} else {
				assertContains(t, result, tt.want)
			}
		})
	}
}

func TestGetMessage_CCHeader(t *testing.T) {
	rawBody := makeRawMessage(
		"text/plain",
		"Hello",
	)
	env := &imap.Envelope{
		Date: time.Date(
			2025, 1, 1, 0, 0, 0, 0, time.UTC,
		),
		Subject: "CC Test",
		From: []imap.Address{
			{Mailbox: "a", Host: "b.com"},
		},
		To: []imap.Address{
			{Mailbox: "c", Host: "d.com"},
		},
		Cc: []imap.Address{
			{
				Name:    "Eve",
				Mailbox: "eve",
				Host:    "example.com",
			},
		},
	}
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(500),
				[]imap.Flag{imap.FlagSeen},
				env,
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":500}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "CC:")
	assertContains(
		t,
		result,
		"Eve <eve@example.com>",
	)
}

func TestGetMessage_MultipleAddresses(t *testing.T) {
	rawBody := makeRawMessage(
		"text/plain",
		"Hello",
	)
	env := &imap.Envelope{
		Date: time.Date(
			2025, 1, 1, 0, 0, 0, 0, time.UTC,
		),
		Subject: "Multi To",
		From: []imap.Address{
			{Mailbox: "sender", Host: "a.com"},
		},
		To: []imap.Address{
			{Mailbox: "one", Host: "a.com"},
			{
				Name:    "Two",
				Mailbox: "two",
				Host:    "b.com",
			},
		},
	}
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(600),
				[]imap.Flag{imap.FlagSeen},
				env,
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":600}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "one@a.com")
	assertContains(t, result, "Two <two@b.com>")
}

func TestGetMessage_AttachmentByMediaType(t *testing.T) {
	// Attachment detected by media type heuristic (no
	// Content-Disposition header). The parse logic should
	// recognize non-text, non-multipart parts as attachments
	// when they are nested (path != nil).
	rawBody := []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"detect\"\r\n" +
			"\r\n" +
			"--detect\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"See image below.\r\n" +
			"--detect\r\n" +
			"Content-Type: image/png\r\n" +
			"\r\n" +
			"PNGdata12345\r\n" +
			"--detect--\r\n",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(801),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":801}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "See image below.")
	assertContains(t, result, "Attachments:")
	assertContains(t, result, "image/png")
}

func TestGetMessage_UnnamedAttachment(t *testing.T) {
	// Attachment without a filename in either
	// Content-Disposition or Content-Type params should
	// fall back to "unnamed".
	rawBody := []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"noname\"\r\n" +
			"\r\n" +
			"--noname\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"Check this.\r\n" +
			"--noname\r\n" +
			"Content-Type: application/octet-stream\r\n" +
			"Content-Disposition: attachment\r\n" +
			"\r\n" +
			"binarydata\r\n" +
			"--noname--\r\n",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(802),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":802}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Attachments:")
	assertContains(t, result, "unnamed")
	assertContains(
		t, result, "application/octet-stream",
	)
}

func TestGetMessage_AttachmentOnlyWithDetails(t *testing.T) {
	// When a message has only attachments and no text parts,
	// the attachments must still be listed in the output.
	rawBody := []byte(
		"From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Test\r\n" +
			"Content-Type: multipart/mixed; " +
			"boundary=\"attonly\"\r\n" +
			"\r\n" +
			"--attonly\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Disposition: attachment; " +
			"filename=\"report.pdf\"\r\n" +
			"\r\n" +
			"PDFdata\r\n" +
			"--attonly--\r\n",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(803),
				[]imap.Flag{imap.FlagSeen},
				standardEnvelope(),
				rawBody,
			),
		},
	}
	tool := NewGetMessage(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a",`+
				`"mailbox":"INBOX","uid":803}`,
		),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(
		t, result, "no readable body content",
	)
	assertContains(t, result, "Attachments:")
	assertContains(t, result, "report.pdf")
	assertContains(t, result, "application/pdf")
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 500, "500 B"},
		{"just under KB", 1023, "1023 B"},
		{"one KB", 1024, "1 KB"},
		{"several KB", 43008, "42 KB"},
		{"one MB", 1048576, "1.0 MB"},
		{"large MB", 5242880, "5.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf(
					"formatSize(%d) = %q, want %q",
					tt.bytes,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestParseBody_Truncation(t *testing.T) {
	// Build a body exceeding maxBodySize (1 MB).
	bigBody := strings.Repeat("A", maxBodySize+100)
	raw := makeRawMessage("text/plain", bigBody)

	parsed, err := parseBody(raw)
	if err != nil {
		t.Fatalf("parseBody() unexpected error: %v", err)
	}

	if len(parsed.text) <= maxBodySize {
		t.Errorf(
			"expected body length > %d "+
				"(includes suffix), got %d",
			maxBodySize,
			len(parsed.text),
		)
	}

	assertContains(
		t, parsed.text, "[body truncated at 1 MB]",
	)
}

func TestFormatAddresses(t *testing.T) {
	tests := []struct {
		name  string
		addrs []imap.Address
		want  string
	}{
		{
			"empty",
			[]imap.Address{},
			"(unknown)",
		},
		{
			"nil",
			nil,
			"(unknown)",
		},
		{
			"single",
			[]imap.Address{
				{Mailbox: "a", Host: "b.com"},
			},
			"a@b.com",
		},
		{
			"multiple",
			[]imap.Address{
				{Mailbox: "a", Host: "b.com"},
				{
					Name:    "C",
					Mailbox: "c",
					Host:    "d.com",
				},
			},
			"a@b.com, C <c@d.com>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAddresses(tt.addrs)
			if got != tt.want {
				t.Errorf(
					"formatAddresses() = %q, "+
						"want %q",
					got,
					tt.want,
				)
			}
		})
	}
}
