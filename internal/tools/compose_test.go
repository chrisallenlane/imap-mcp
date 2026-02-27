package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	s := string(msg)

	assertContains(t, s, "From: <sender@example.com>")
	assertContains(t, s, "To: <recipient@example.com>")
	assertContains(t, s, "Subject: Test Subject")
	assertContains(t, s, "Message-Id: <")
	assertContains(t, s, "Date: ")
	assertContains(t, s, "Hello, world!")
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

	s := string(msg)
	assertContains(t, s, "Cc: <cc@example.com>")
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

	s := string(msg)
	assertContains(t, s, "Bcc: <bcc@example.com>")
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

	s := string(msg)
	assertContains(t, s, "multipart/mixed")
	assertContains(t, s, "See attached.")
	assertContains(t, s, "test.txt")
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

	s := string(msg)
	assertContains(t, s, "to1@example.com")
	assertContains(t, s, "to2@example.com")
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
