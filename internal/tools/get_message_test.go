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

func (m *mockMessageGetter) FetchMessageByUID(
	_ string,
	_ string,
	_ imap.UID,
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

func TestGetMessage_Description(t *testing.T) {
	tool := NewGetMessage(&mockMessageGetter{})

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestGetMessage_InputSchema(t *testing.T) {
	tool := NewGetMessage(&mockMessageGetter{})

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
	if _, ok := props["uid"]; !ok {
		t.Error("schema should have 'uid' property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a []string")
	}
	if len(required) != 3 {
		t.Fatalf(
			"expected 3 required fields, got %d",
			len(required),
		)
	}

	requiredSet := map[string]bool{}
	for _, r := range required {
		requiredSet[r] = true
	}
	if !requiredSet["account"] ||
		!requiredSet["mailbox"] ||
		!requiredSet["uid"] {
		t.Errorf(
			"required = %v, "+
				"want [account, mailbox, uid]",
			required,
		)
	}
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

func TestGetMessage_NoPlainText(t *testing.T) {
	rawBody := makeRawMessage(
		"text/html",
		"<html><body>Hello</body></html>",
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
		"no plain text body",
	)
	assertContains(
		t,
		result,
		"HTML-only content is not yet supported",
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
	assertContains(t, result, "no envelope data")
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
	tool := NewGetMessage(&mockMessageGetter{})

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

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 500, "500 B"},
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

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name string
		addr imap.Address
		want string
	}{
		{
			"email only",
			imap.Address{Mailbox: "alice", Host: "a.com"},
			"alice@a.com",
		},
		{
			"with display name",
			imap.Address{
				Name:    "Alice",
				Mailbox: "alice",
				Host:    "a.com",
			},
			"Alice <alice@a.com>",
		},
		{
			"empty address",
			imap.Address{},
			"(unknown)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAddress(tt.addr)
			if got != tt.want {
				t.Errorf(
					"formatAddress() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
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
