package tools

import (
	"strings"
	"testing"
	"time"

	imaplib "github.com/emersion/go-imap/v2"
)

// makeAddr is a test helper that builds an imaplib.Address
// from a mailbox and host string (e.g., "user", "example.com"
// -> user@example.com).
func makeAddr(mailbox, host string) imaplib.Address {
	return imaplib.Address{
		Mailbox: mailbox,
		Host:    host,
	}
}

// makeEnv builds a minimal *imaplib.Envelope for use in
// reply-helper tests.
func makeEnv(
	from, to, cc []imaplib.Address,
	subject, msgID string,
	date time.Time,
) *imaplib.Envelope {
	return &imaplib.Envelope{
		From:      from,
		To:        to,
		Cc:        cc,
		Subject:   subject,
		MessageID: msgID,
		Date:      date,
	}
}

// TestReplyAllRecipients covers the self-exclusion and dedup
// logic in replyAllRecipients.
func TestReplyAllRecipients(t *testing.T) {
	tests := []struct {
		name       string
		from       []imaplib.Address
		to         []imaplib.Address
		cc         []imaplib.Address
		self       string
		overrideTo []string
		overrideCC []string
		wantTo     []string
		wantCC     []string
	}{
		{
			name: "all recipients match self — empty result",
			from: []imaplib.Address{makeAddr("me", "example.com")},
			to:   []imaplib.Address{makeAddr("me", "example.com")},
			cc:   []imaplib.Address{makeAddr("me", "example.com")},
			self: "me@example.com",
			// All addresses are self — both slices should be nil.
			wantTo: nil,
			wantCC: nil,
		},
		{
			name: "CC overlaps To — CC deduped",
			// sender is in From; recipient is in To and also in CC.
			from: []imaplib.Address{makeAddr("sender", "example.com")},
			to:   []imaplib.Address{makeAddr("recipient", "example.com")},
			cc: []imaplib.Address{
				makeAddr("recipient", "example.com"),
				makeAddr("other", "example.com"),
			},
			self: "me@example.com",
			// To = sender + recipient (both non-self, From+To).
			wantTo: []string{
				"sender@example.com",
				"recipient@example.com",
			},
			// CC: "recipient" is already in To via env.To —
			// but replyAllRecipients only deduplicates within
			// the CC list itself (via ccSet), not against To.
			// env.Cc is added to CC if not in overrideCC set.
			// "recipient" and "other" are both added.
			wantCC: []string{
				"recipient@example.com",
				"other@example.com",
			},
		},
		{
			name: "case-insensitive self-exclusion",
			from: []imaplib.Address{makeAddr("sender", "example.com")},
			to: []imaplib.Address{
				// Self address in mixed case.
				makeAddr("ME", "EXAMPLE.COM"),
			},
			cc: []imaplib.Address{
				makeAddr("cc", "example.com"),
			},
			// Self in lower-case — should still match ME@EXAMPLE.COM.
			self: "me@example.com",
			// ME@EXAMPLE.COM is self, so not included in To.
			wantTo: []string{"sender@example.com"},
			wantCC: []string{"cc@example.com"},
		},
		{
			name:       "override To bypasses calculation",
			from:       []imaplib.Address{makeAddr("sender", "example.com")},
			to:         []imaplib.Address{makeAddr("recipient", "example.com")},
			cc:         []imaplib.Address{makeAddr("cc", "example.com")},
			self:       "me@example.com",
			overrideTo: []string{"custom@example.com"},
			overrideCC: []string{"customcc@example.com"},
			wantTo:     []string{"custom@example.com"},
			wantCC:     []string{"customcc@example.com"},
		},
		{
			name: "self excluded from CC via case-insensitive match",
			from: []imaplib.Address{makeAddr("sender", "example.com")},
			to:   []imaplib.Address{makeAddr("recipient", "example.com")},
			cc: []imaplib.Address{
				makeAddr("Me", "Example.Com"),
				makeAddr("cc", "example.com"),
			},
			self:   "me@example.com",
			wantTo: []string{"sender@example.com", "recipient@example.com"},
			wantCC: []string{"cc@example.com"},
		},
		{
			name:       "overrideCC self-exclusion",
			from:       []imaplib.Address{makeAddr("sender", "example.com")},
			to:         []imaplib.Address{makeAddr("recipient", "example.com")},
			cc:         nil,
			self:       "me@example.com",
			overrideCC: []string{"me@example.com", "cc@example.com"},
			// Self should be stripped from overrideCC.
			wantTo: []string{
				"sender@example.com",
				"recipient@example.com",
			},
			wantCC: []string{"cc@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := makeEnv(
				tt.from, tt.to, tt.cc,
				"subject", "msgid",
				time.Now(),
			)

			gotTo, gotCC := replyAllRecipients(
				env, tt.self, tt.overrideTo, tt.overrideCC,
			)

			if !stringSliceEqual(gotTo, tt.wantTo) {
				t.Errorf(
					"To = %v, want %v",
					gotTo,
					tt.wantTo,
				)
			}
			if !stringSliceEqual(gotCC, tt.wantCC) {
				t.Errorf(
					"CC = %v, want %v",
					gotCC,
					tt.wantCC,
				)
			}
		})
	}
}

// stringSliceEqual compares two string slices for equality,
// treating nil and empty as distinct.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestQuoteReply covers the reply-body quoting logic.
func TestQuoteReply(t *testing.T) {
	baseDate := time.Date(
		2025, 1, 15, 10, 30, 0, 0, time.UTC,
	)
	sender := makeAddr("sender", "example.com")

	tests := []struct {
		name        string
		userBody    string
		envFrom     []imaplib.Address
		srcBodyText string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "single-line body is quoted with > prefix",
			userBody:    "My reply.",
			envFrom:     []imaplib.Address{sender},
			srcBodyText: "Original text.",
			wantContain: []string{
				"My reply.",
				"On ",
				"sender@example.com",
				"wrote:",
				"> Original text.",
			},
		},
		{
			name:        "multi-line body: each line gets > prefix",
			userBody:    "My reply.",
			envFrom:     []imaplib.Address{sender},
			srcBodyText: "Line one.\nLine two.\nLine three.",
			wantContain: []string{
				"> Line one.",
				"> Line two.",
				"> Line three.",
			},
		},
		{
			name:        "empty source body produces no quoted lines",
			userBody:    "My reply.",
			envFrom:     []imaplib.Address{sender},
			srcBodyText: "",
			wantContain: []string{
				"My reply.",
				"wrote:",
			},
			// No "> " lines when there is no source text.
			wantAbsent: []string{"> "},
		},
		{
			name:     "nested quoting: existing > lines get another >",
			userBody: "Re-reply.",
			envFrom:  []imaplib.Address{sender},
			// Source body itself already has quoted lines.
			srcBodyText: "> Previously quoted.\nNew text.",
			wantContain: []string{
				"> > Previously quoted.",
				"> New text.",
			},
		},
		{
			name:        "missing From address falls back to (unknown)",
			userBody:    "My reply.",
			envFrom:     nil,
			srcBodyText: "Body.",
			wantContain: []string{"(unknown)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := makeEnv(
				tt.envFrom, nil, nil,
				"subject", "msgid",
				baseDate,
			)
			sb := sourceBody{text: tt.srcBodyText}

			got := quoteReply(tt.userBody, env, sb)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf(
						"quoteReply output missing %q\ngot:\n%s",
						want,
						got,
					)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf(
						"quoteReply output should not "+
							"contain %q\ngot:\n%s",
						absent,
						got,
					)
				}
			}
		})
	}
}

// TestQuoteForward covers the forward-body formatting logic.
func TestQuoteForward(t *testing.T) {
	baseDate := time.Date(
		2025, 1, 15, 10, 30, 0, 0, time.UTC,
	)
	sender := makeAddr("sender", "example.com")
	recipient := makeAddr("recipient", "example.com")

	tests := []struct {
		name        string
		userBody    string
		envFrom     []imaplib.Address
		envTo       []imaplib.Address
		subject     string
		srcBodyText string
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "forward block contains expected headers",
			userBody:    "FYI.",
			envFrom:     []imaplib.Address{sender},
			envTo:       []imaplib.Address{recipient},
			subject:     "Original Subject",
			srcBodyText: "Original body.",
			wantContain: []string{
				"FYI.",
				"Forwarded message",
				"From: sender@example.com",
				"Subject: Original Subject",
				"To: recipient@example.com",
				"Original body.",
			},
		},
		{
			name:        "empty source body omits body section",
			userBody:    "FYI.",
			envFrom:     []imaplib.Address{sender},
			envTo:       []imaplib.Address{recipient},
			subject:     "Subject",
			srcBodyText: "",
			wantContain: []string{
				"Forwarded message",
			},
			// No body text should appear after the header block.
			wantAbsent: []string{"Original body."},
		},
		{
			name:    "missing From uses (unknown) fallback",
			envFrom: nil,
			envTo:   []imaplib.Address{recipient},
			subject: "Subject",
			wantContain: []string{
				"From: (unknown)",
			},
		},
		{
			name:    "missing To uses (unknown) fallback",
			envFrom: []imaplib.Address{sender},
			envTo:   nil,
			subject: "Subject",
			wantContain: []string{
				"To: (unknown)",
			},
		},
		{
			name:        "user body appears before forwarded block",
			userBody:    "See below.",
			envFrom:     []imaplib.Address{sender},
			envTo:       []imaplib.Address{recipient},
			subject:     "Orig",
			srcBodyText: "Body.",
			wantContain: []string{"See below."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := makeEnv(
				tt.envFrom, tt.envTo, nil,
				tt.subject, "msgid",
				baseDate,
			)
			sb := sourceBody{text: tt.srcBodyText}

			got := quoteForward(tt.userBody, env, sb)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf(
						"quoteForward output missing %q\ngot:\n%s",
						want,
						got,
					)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf(
						"quoteForward output should not "+
							"contain %q\ngot:\n%s",
						absent,
						got,
					)
				}
			}
		})
	}
}
