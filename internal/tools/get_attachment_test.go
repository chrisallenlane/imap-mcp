package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func TestGetAttachment_InputSchema(t *testing.T) {
	assertSchema(
		t,
		NewGetAttachment(
			&mockMessageGetter{},
		).InputSchema(),
		[]string{
			"account",
			"mailbox",
			"uid",
			"attachment",
			"directory",
		},
		[]string{
			"account",
			"mailbox",
			"uid",
			"attachment",
		},
	)
}

func TestGetAttachment_Description(t *testing.T) {
	tool := NewGetAttachment(&mockMessageGetter{})
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestGetAttachment_Success(t *testing.T) {
	dir := t.TempDir()
	rawBody := makeMultipartMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":1,`+
				`"directory":%q}`,
			dir,
		)),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Downloaded test.pdf")
	assertContains(t, result, "application/pdf")

	// Verify file was written.
	data, err := os.ReadFile(
		filepath.Join(dir, "test.pdf"),
	)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != "PDFcontent" {
		t.Errorf(
			"file content = %q, want %q",
			string(data),
			"PDFcontent",
		)
	}
}

func TestGetAttachment_UIDAsString(t *testing.T) {
	// Claude sometimes passes UIDs as quoted strings;
	// verify the tool accepts either form.
	dir := t.TempDir()
	rawBody := makeMultipartMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":"100","attachment":1,`+
				`"directory":%q}`,
			dir,
		)),
	)
	if err != nil {
		t.Fatalf(
			"Execute() unexpected error: %v", err,
		)
	}

	assertContains(t, result, "Downloaded test.pdf")
}

func TestGetAttachment_MultipleAttachments(t *testing.T) {
	dir := t.TempDir()
	rawBody := makeMultiAttachmentMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	// Fetch second attachment.
	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":2,`+
				`"directory":%q}`,
			dir,
		)),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Downloaded photo.png")
	assertContains(t, result, "image/png")

	data, err := os.ReadFile(
		filepath.Join(dir, "photo.png"),
	)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	if string(data) != "PNGdata" {
		t.Errorf(
			"file content = %q, want %q",
			string(data),
			"PNGdata",
		)
	}
}

func TestGetAttachment_IndexOutOfRange(t *testing.T) {
	rawBody := makeMultipartMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":5,`+
				`"directory":%q}`,
			t.TempDir(),
		)),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"out of range index",
		)
	}
	assertContains(
		t,
		err.Error(),
		"attachment 5 not found",
	)
	assertContains(t, err.Error(), "1 attachments")
}

func TestGetAttachment_NoAttachments(t *testing.T) {
	rawBody := makeRawMessage(
		"text/plain",
		"Hello, world!",
	)
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":1,`+
				`"directory":%q}`,
			t.TempDir(),
		)),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"no attachments",
		)
	}
	assertContains(
		t,
		err.Error(),
		"message has no attachments",
	)
}

func TestGetAttachment_FilenameCollision(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the file to trigger collision handling.
	if err := os.WriteFile(
		filepath.Join(dir, "test.pdf"),
		[]byte("existing"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	rawBody := makeMultipartMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":1,`+
				`"directory":%q}`,
			dir,
		)),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "test (1).pdf")

	// Original file should be unchanged.
	orig, err := os.ReadFile(
		filepath.Join(dir, "test.pdf"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(orig) != "existing" {
		t.Error("original file was overwritten")
	}

	// New file should have attachment content.
	newData, err := os.ReadFile(
		filepath.Join(dir, "test (1).pdf"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(newData) != "PDFcontent" {
		t.Errorf(
			"collision file content = %q, want %q",
			string(newData),
			"PDFcontent",
		)
	}
}

func TestGetAttachment_DirectoryCreation(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")

	rawBody := makeMultipartMessage()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":1,`+
				`"directory":%q}`,
			nested,
		)),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, nested)

	if _, err := os.Stat(
		filepath.Join(nested, "test.pdf"),
	); err != nil {
		t.Errorf("file not created in nested dir: %v", err)
	}
}

func TestGetAttachment_MissingFilename(t *testing.T) {
	// Build a message with an attachment that has no
	// filename in its MIME headers.
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
			"See attached.\r\n" +
			"--noname\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Disposition: attachment\r\n" +
			"\r\n" +
			"PDFbytes\r\n" +
			"--noname--\r\n",
	)
	dir := t.TempDir()
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				rawBody,
			),
		},
	}
	tool := NewGetAttachment(mock)

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(fmt.Sprintf(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":1,`+
				`"directory":%q}`,
			dir,
		)),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "attachment_1")
	assertContains(t, result, "application/pdf")
}

func TestGetAttachment_MissingAccount(t *testing.T) {
	tool := NewGetAttachment(&mockMessageGetter{})
	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"mailbox":"INBOX","uid":1,`+
				`"attachment":1}`,
		),
	)
	if err == nil {
		t.Fatal("expected error for missing account")
	}
	assertContains(t, err.Error(), "account is required")
}

func TestGetAttachment_MissingMailbox(t *testing.T) {
	tool := NewGetAttachment(&mockMessageGetter{})
	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","uid":1,`+
				`"attachment":1}`,
		),
	)
	if err == nil {
		t.Fatal("expected error for missing mailbox")
	}
	assertContains(t, err.Error(), "mailbox is required")
}

func TestGetAttachment_MissingUID(t *testing.T) {
	tool := NewGetAttachment(&mockMessageGetter{})
	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"attachment":1}`,
		),
	)
	if err == nil {
		t.Fatal("expected error for missing uid")
	}
	assertContains(t, err.Error(), "uid is required")
}

func TestGetAttachment_MissingAttachment(t *testing.T) {
	tool := NewGetAttachment(&mockMessageGetter{})
	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":1}`,
		),
	)
	if err == nil {
		t.Fatal(
			"expected error for missing attachment",
		)
	}
	assertContains(
		t,
		err.Error(),
		"attachment is required",
	)
}

func TestGetAttachment_InvalidJSON(t *testing.T) {
	assertInvalidJSONError(
		t,
		NewGetAttachment(&mockMessageGetter{}),
	)
}

func TestGetAttachment_FetchError(t *testing.T) {
	mock := &mockMessageGetter{
		err: fmt.Errorf("connection timeout"),
	}
	tool := NewGetAttachment(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":1,"attachment":1}`,
		),
	)
	if err == nil {
		t.Fatal("expected error from fetcher")
	}
	assertContains(t, err.Error(), "connection timeout")
}

func TestGetAttachment_MessageNotFound(t *testing.T) {
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{},
	}
	tool := NewGetAttachment(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":999,"attachment":1}`,
		),
	)
	if err == nil {
		t.Fatal("expected error for not found")
	}
	assertContains(t, err.Error(), "message not found")
}

func TestGetAttachment_NoBodyData(t *testing.T) {
	mock := &mockMessageGetter{
		messages: []*imapclient.FetchMessageBuffer{
			mockMsg(
				imap.UID(100),
				nil,
				nil,
				nil,
			),
		},
	}
	tool := NewGetAttachment(mock)

	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(
			`{"account":"a","mailbox":"INBOX",`+
				`"uid":100,"attachment":1}`,
		),
	)
	if err == nil {
		t.Fatal("expected error for no body data")
	}
	assertContains(
		t,
		err.Error(),
		"message has no body data",
	)
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()

	// No collision.
	free := filepath.Join(dir, "free.txt")
	if got := uniquePath(free); got != free {
		t.Errorf("uniquePath() = %q, want %q", got, free)
	}

	// Create a file to cause collision.
	existing := filepath.Join(dir, "taken.txt")
	if err := os.WriteFile(
		existing, []byte("x"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	got := uniquePath(existing)
	want := filepath.Join(dir, "taken (1).txt")
	if got != want {
		t.Errorf("uniquePath() = %q, want %q", got, want)
	}

	// Create the (1) file too.
	if err := os.WriteFile(
		want, []byte("y"), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	got = uniquePath(existing)
	want = filepath.Join(dir, "taken (2).txt")
	if got != want {
		t.Errorf("uniquePath() = %q, want %q", got, want)
	}
}

func TestFallbackFilename(t *testing.T) {
	tests := []struct {
		name      string
		index     int
		mediaType string
		want      string
	}{
		{
			"pdf",
			1,
			"application/pdf",
			"attachment_1.pdf",
		},
		{
			"unknown type",
			3,
			"application/x-unknown-type",
			"attachment_3.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fallbackFilename(
				tt.index, tt.mediaType,
			)
			if got != tt.want {
				t.Errorf(
					"fallbackFilename() = %q, "+
						"want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestExtractAttachment(t *testing.T) {
	rawBody := makeMultiAttachmentMessage()

	t.Run("first attachment", func(t *testing.T) {
		att, err := extractAttachment(rawBody, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if att.filename != "report.pdf" {
			t.Errorf(
				"filename = %q, want %q",
				att.filename,
				"report.pdf",
			)
		}
		if att.mediaType != "application/pdf" {
			t.Errorf(
				"mediaType = %q, want %q",
				att.mediaType,
				"application/pdf",
			)
		}
		if string(att.data) != "PDFdata1234567890" {
			t.Errorf(
				"data = %q, want %q",
				string(att.data),
				"PDFdata1234567890",
			)
		}
	})

	t.Run("second attachment", func(t *testing.T) {
		att, err := extractAttachment(rawBody, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if att.filename != "photo.png" {
			t.Errorf(
				"filename = %q, want %q",
				att.filename,
				"photo.png",
			)
		}
		if att.mediaType != "image/png" {
			t.Errorf(
				"mediaType = %q, want %q",
				att.mediaType,
				"image/png",
			)
		}
	})

	t.Run("out of range", func(t *testing.T) {
		_, err := extractAttachment(rawBody, 3)
		if err == nil {
			t.Fatal("expected error for out of range")
		}
		assertContains(
			t,
			err.Error(),
			"attachment 3 not found",
		)
		assertContains(
			t,
			err.Error(),
			"2 attachments",
		)
	})

	t.Run("no attachments", func(t *testing.T) {
		raw := makeRawMessage(
			"text/plain",
			"no attachments here",
		)
		_, err := extractAttachment(raw, 1)
		if err == nil {
			t.Fatal(
				"expected error for no attachments",
			)
		}
		assertContains(
			t,
			err.Error(),
			"message has no attachments",
		)
	})
}
