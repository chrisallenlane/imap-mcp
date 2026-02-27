package tools

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gomail "github.com/emersion/go-message/mail"
)

// parsedMsg holds the header and parts of a parsed mail message
// for use in compose tests.
type parsedMsg struct {
	header *gomail.Header
	parts  []*gomail.Part
}

// parseMsgForTest parses raw message bytes into a parsedMsg,
// reading all parts eagerly so callers can inspect them freely.
func parseMsgForTest(t *testing.T, msg []byte) *parsedMsg {
	t.Helper()

	r, err := gomail.CreateReader(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("failed to parse composed message: %v", err)
	}
	defer r.Close()

	h := r.Header
	var parts []*gomail.Part
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read message part: %v", err)
		}
		// Drain the body so the reader advances correctly.
		if _, err := io.Copy(io.Discard, p.Body); err != nil {
			t.Fatalf("failed to drain part body: %v", err)
		}
		parts = append(parts, p)
	}

	return &parsedMsg{header: &h, parts: parts}
}

// parseMsgBodyForTest parses raw message bytes like parseMsgForTest
// but returns body text for the first inline text/plain part.
func parseMsgBodyForTest(
	t *testing.T,
	msg []byte,
) (*parsedMsg, string) {
	t.Helper()

	r, err := gomail.CreateReader(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("failed to parse composed message: %v", err)
	}
	defer r.Close()

	h := r.Header
	var parts []*gomail.Part
	var bodyText string

	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read message part: %v", err)
		}

		data, err := io.ReadAll(p.Body)
		if err != nil {
			t.Fatalf("failed to read part body: %v", err)
		}

		if _, ok := p.Header.(*gomail.InlineHeader); ok &&
			bodyText == "" {
			bodyText = string(data)
		}

		parts = append(parts, p)
	}

	return &parsedMsg{header: &h, parts: parts}, bodyText
}

func TestComposeMessage_PlainText(t *testing.T) {
	msg, err := composeMessage(composeParams{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Body:    "Hello, world!",
	})
	if err != nil {
		t.Fatalf("composeMessage() error: %v", err)
	}

	pm, body := parseMsgBodyForTest(t, msg)
	h := pm.header

	// Verify From address.
	from, err := h.AddressList("From")
	if err != nil || len(from) != 1 {
		t.Fatalf(
			"AddressList(From) = %v, %v; want 1 address",
			from,
			err,
		)
	}
	if from[0].Address != "sender@example.com" {
		t.Errorf(
			"From address = %q, want %q",
			from[0].Address,
			"sender@example.com",
		)
	}

	// Verify To address.
	to, err := h.AddressList("To")
	if err != nil || len(to) != 1 {
		t.Fatalf(
			"AddressList(To) = %v, %v; want 1 address",
			to,
			err,
		)
	}
	if to[0].Address != "recipient@example.com" {
		t.Errorf(
			"To address = %q, want %q",
			to[0].Address,
			"recipient@example.com",
		)
	}

	// Verify Subject.
	subject, err := h.Subject()
	if err != nil {
		t.Fatalf("Subject() error: %v", err)
	}
	if subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", subject, "Test Subject")
	}

	// Verify Message-ID is present (non-empty).
	msgID, err := h.MessageID()
	if err != nil {
		t.Fatalf("MessageID() error: %v", err)
	}
	if msgID == "" {
		t.Error("Message-ID is empty, want a generated ID")
	}

	// Verify Date is present (non-zero).
	date, err := h.Date()
	if err != nil {
		t.Fatalf("Date() error: %v", err)
	}
	if date.IsZero() {
		t.Error("Date is zero, want a valid timestamp")
	}

	// Verify body content.
	if !strings.Contains(body, "Hello, world!") {
		t.Errorf(
			"body = %q, want it to contain %q",
			body,
			"Hello, world!",
		)
	}
}

func TestComposeMessage_WithCC(t *testing.T) {
	msg, err := composeMessage(composeParams{
		From:    "sender@example.com",
		To:      []string{"to@example.com"},
		CC:      []string{"cc@example.com"},
		Subject: "CC Test",
		Body:    "body",
	})
	if err != nil {
		t.Fatalf("composeMessage() error: %v", err)
	}

	pm := parseMsgForTest(t, msg)
	cc, err := pm.header.AddressList("Cc")
	if err != nil || len(cc) != 1 {
		t.Fatalf(
			"AddressList(Cc) = %v, %v; want 1 address",
			cc,
			err,
		)
	}
	if cc[0].Address != "cc@example.com" {
		t.Errorf(
			"Cc address = %q, want %q",
			cc[0].Address,
			"cc@example.com",
		)
	}
}

func TestComposeMessage_WithBCC(t *testing.T) {
	msg, err := composeMessage(composeParams{
		From:    "sender@example.com",
		To:      []string{"to@example.com"},
		BCC:     []string{"bcc@example.com"},
		Subject: "BCC Test",
		Body:    "body",
	})
	if err != nil {
		t.Fatalf("composeMessage() error: %v", err)
	}

	pm := parseMsgForTest(t, msg)
	bcc, err := pm.header.AddressList("Bcc")
	if err != nil || len(bcc) != 1 {
		t.Fatalf(
			"AddressList(Bcc) = %v, %v; want 1 address",
			bcc,
			err,
		)
	}
	if bcc[0].Address != "bcc@example.com" {
		t.Errorf(
			"Bcc address = %q, want %q",
			bcc[0].Address,
			"bcc@example.com",
		)
	}
}

func TestComposeMessage_WithAttachments(t *testing.T) {
	dir := t.TempDir()
	attPath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(
		attPath,
		[]byte("attachment content"),
		0o600,
	); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	msg, err := composeMessage(composeParams{
		From:        "sender@example.com",
		To:          []string{"to@example.com"},
		Subject:     "Attachment Test",
		Body:        "See attached.",
		Attachments: []string{attPath},
	})
	if err != nil {
		t.Fatalf("composeMessage() error: %v", err)
	}

	r, err := gomail.CreateReader(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}
	defer r.Close()

	var foundBody bool
	var foundAttachment bool

	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read part: %v", err)
		}

		data, err := io.ReadAll(p.Body)
		if err != nil {
			t.Fatalf("failed to read part body: %v", err)
		}

		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			if strings.Contains(string(data), "See attached.") {
				foundBody = true
			}
		case *gomail.AttachmentHeader:
			filename, err := h.Filename()
			if err != nil {
				t.Fatalf("Filename() error: %v", err)
			}
			if filename == "test.txt" {
				foundAttachment = true
			}
		}
	}

	if !foundBody {
		t.Error("body part not found or did not contain expected text")
	}
	if !foundAttachment {
		t.Error(
			"attachment part with filename \"test.txt\" not found",
		)
	}
}

func TestComposeMessage_AttachmentNotFound(t *testing.T) {
	_, err := composeMessage(composeParams{
		From:        "sender@example.com",
		To:          []string{"to@example.com"},
		Subject:     "Missing File",
		Body:        "body",
		Attachments: []string{"/nonexistent/file.txt"},
	})
	if err == nil {
		t.Fatal("expected error for missing attachment")
	}
	if !strings.Contains(
		err.Error(),
		"failed to read attachment",
	) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComposeMessage_MultipleRecipients(t *testing.T) {
	msg, err := composeMessage(composeParams{
		From: "sender@example.com",
		To: []string{
			"to1@example.com",
			"to2@example.com",
		},
		Subject: "Multi-recipient",
		Body:    "body",
	})
	if err != nil {
		t.Fatalf("composeMessage() error: %v", err)
	}

	pm := parseMsgForTest(t, msg)
	to, err := pm.header.AddressList("To")
	if err != nil {
		t.Fatalf("AddressList(To) error: %v", err)
	}

	wantAddrs := []string{"to1@example.com", "to2@example.com"}
	if len(to) != len(wantAddrs) {
		t.Fatalf(
			"To address count = %d, want %d",
			len(to),
			len(wantAddrs),
		)
	}
	for i, want := range wantAddrs {
		if to[i].Address != want {
			t.Errorf(
				"To[%d] = %q, want %q",
				i,
				to[i].Address,
				want,
			)
		}
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		data     []byte
		want     string
	}{
		{
			name:     "pdf by extension",
			filename: "report.pdf",
			data:     []byte("%PDF-1.4"),
			want:     "application/pdf",
		},
		{
			name:     "text by extension",
			filename: "notes.txt",
			data:     []byte("hello"),
			want:     "text/plain",
		},
		{
			name:     "fallback to content sniffing",
			filename: "unknown",
			data:     []byte("<html><body>hi</body></html>"),
			want:     "text/html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectMediaType(tt.filename, tt.data)
			if !strings.HasPrefix(got, tt.want) {
				t.Errorf(
					"detectMediaType() = %q, want prefix %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func TestToMailAddresses(t *testing.T) {
	addrs := toMailAddresses("a@b.com", "c@d.com")
	if len(addrs) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(addrs))
	}
	if addrs[0].Address != "a@b.com" {
		t.Errorf(
			"addr[0] = %q, want %q",
			addrs[0].Address,
			"a@b.com",
		)
	}
	if addrs[1].Address != "c@d.com" {
		t.Errorf(
			"addr[1] = %q, want %q",
			addrs[1].Address,
			"c@d.com",
		)
	}
}
