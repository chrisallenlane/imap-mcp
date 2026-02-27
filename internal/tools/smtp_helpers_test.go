package tools

import (
	"errors"
	"testing"

	imap "github.com/emersion/go-imap/v2"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// mockSentSaverHelper is a local mock for sentSaver used
// in smtp_helpers tests. (mockSentSaver is defined in
// send_message_test.go and is not redeclared here.)

// resolveConfig builds a *config.Config for use in
// resolveSMTPAccount tests.
func resolveConfig() *config.Config {
	return &config.Config{
		Accounts: map[string]config.Account{
			"smtp-on": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user@example.com",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
			},
			"smtp-on-with-from": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user@example.com",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
				SMTPFrom:    "custom@example.com",
			},
			"smtp-off": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user@example.com",
				Password:    "pass",
				SMTPEnabled: false,
			},
		},
	}
}

// TestResolveSMTPAccount covers the four scenarios of
// resolveSMTPAccount: unknown account, SMTP disabled,
// SMTPFrom override, and username fallback.
func TestResolveSMTPAccount(t *testing.T) {
	cfg := resolveConfig()

	tests := []struct {
		name        string
		accountName string
		wantFrom    string
		wantErr     bool
		errContains string
	}{
		{
			name:        "unknown account",
			accountName: "does-not-exist",
			wantErr:     true,
			errContains: "unknown account",
		},
		{
			name:        "smtp disabled",
			accountName: "smtp-off",
			wantErr:     true,
			errContains: "not enabled",
		},
		{
			name:        "smtp_from override",
			accountName: "smtp-on-with-from",
			wantFrom:    "custom@example.com",
		},
		{
			name:        "username fallback when smtp_from empty",
			accountName: "smtp-on",
			wantFrom:    "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acct, from, err := resolveSMTPAccount(
				cfg, tt.accountName,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatalf(
						"expected error, got nil",
					)
				}
				if tt.errContains != "" {
					assertContains(
						t,
						err.Error(),
						tt.errContains,
					)
				}
				return
			}

			if err != nil {
				t.Fatalf(
					"unexpected error: %v", err,
				)
			}

			if from != tt.wantFrom {
				t.Errorf(
					"from = %q, want %q",
					from,
					tt.wantFrom,
				)
			}

			// The returned account should match the
			// requested one.
			if acct.Username == "" {
				t.Error(
					"returned account should not " +
						"be zero value",
				)
			}
		})
	}
}

// TestCollectRecipients covers various combinations of
// to/cc/bcc slices.
func TestCollectRecipients(t *testing.T) {
	tests := []struct {
		name string
		to   []string
		cc   []string
		bcc  []string
		want []string
	}{
		{
			name: "all empty",
			to:   nil,
			cc:   nil,
			bcc:  nil,
			want: []string{},
		},
		{
			name: "to only",
			to:   []string{"a@example.com"},
			cc:   nil,
			bcc:  nil,
			want: []string{"a@example.com"},
		},
		{
			name: "cc only",
			to:   nil,
			cc:   []string{"b@example.com"},
			bcc:  nil,
			want: []string{"b@example.com"},
		},
		{
			name: "bcc only",
			to:   nil,
			cc:   nil,
			bcc:  []string{"c@example.com"},
			want: []string{"c@example.com"},
		},
		{
			name: "to and cc",
			to:   []string{"a@example.com"},
			cc:   []string{"b@example.com"},
			bcc:  nil,
			want: []string{
				"a@example.com",
				"b@example.com",
			},
		},
		{
			name: "to and bcc",
			to:   []string{"a@example.com"},
			cc:   nil,
			bcc:  []string{"c@example.com"},
			want: []string{
				"a@example.com",
				"c@example.com",
			},
		},
		{
			name: "cc and bcc",
			to:   nil,
			cc:   []string{"b@example.com"},
			bcc:  []string{"c@example.com"},
			want: []string{
				"b@example.com",
				"c@example.com",
			},
		},
		{
			name: "all three fields populated",
			to:   []string{"a@example.com"},
			cc:   []string{"b@example.com"},
			bcc:  []string{"c@example.com"},
			want: []string{
				"a@example.com",
				"b@example.com",
				"c@example.com",
			},
		},
		{
			name: "multiple recipients per field",
			to: []string{
				"a1@example.com",
				"a2@example.com",
			},
			cc: []string{
				"b1@example.com",
				"b2@example.com",
			},
			bcc: []string{
				"c1@example.com",
			},
			want: []string{
				"a1@example.com",
				"a2@example.com",
				"b1@example.com",
				"b2@example.com",
				"c1@example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectRecipients(tt.to, tt.cc, tt.bcc)

			if len(got) != len(tt.want) {
				t.Fatalf(
					"len = %d, want %d: got %v",
					len(got),
					len(tt.want),
					got,
				)
			}

			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf(
						"[%d] = %q, want %q",
						i,
						got[i],
						want,
					)
				}
			}
		})
	}
}

// mockSentSaverForHelpers is a sentSaver mock local to
// smtp_helpers tests. It is separate from mockSentSaver
// (defined in send_message_test.go) to avoid redeclaring
// that type in the same package.
type mockSentSaverForHelpers struct {
	sentMailbox   string
	findErr       error
	appendErr     error
	appendCalled  bool
	appendedMsg   []byte
	appendedFlags []imap.Flag
}

func (m *mockSentSaverForHelpers) FindSentMailbox(
	_ string,
) (string, error) {
	return m.sentMailbox, m.findErr
}

func (m *mockSentSaverForHelpers) AppendMessage(
	_, _ string,
	msg []byte,
	flags []imap.Flag,
) error {
	m.appendCalled = true
	m.appendedMsg = msg
	m.appendedFlags = flags
	return m.appendErr
}

// TestTrySaveToSent covers the three code paths inside
// trySaveToSent: FindSentMailbox error, AppendMessage
// error, and the happy path. A fourth case documents the
// caller-side SaveSent=false guard (trySaveToSent itself
// always tries; the SaveSent check is the caller's
// responsibility).
func TestTrySaveToSent(t *testing.T) {
	msgBytes := []byte("raw message bytes")

	tests := []struct {
		name         string
		saver        *mockSentSaverForHelpers
		wantResult   bool
		wantAppended bool
		wantSeenFlag bool
	}{
		{
			name: "FindSentMailbox error returns false",
			saver: &mockSentSaverForHelpers{
				findErr: errors.New("no sent folder"),
			},
			wantResult:   false,
			wantAppended: false,
		},
		{
			name: "AppendMessage error returns false",
			saver: &mockSentSaverForHelpers{
				sentMailbox: "Sent",
				appendErr:   errors.New("IMAP error"),
			},
			wantResult:   false,
			wantAppended: true, // called but failed
		},
		{
			name: "happy path returns true",
			saver: &mockSentSaverForHelpers{
				sentMailbox: "Sent",
			},
			wantResult:   true,
			wantAppended: true,
			wantSeenFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trySaveToSent(
				tt.saver, "test", msgBytes,
			)

			if got != tt.wantResult {
				t.Errorf(
					"result = %v, want %v",
					got,
					tt.wantResult,
				)
			}

			if tt.saver.appendCalled != tt.wantAppended {
				t.Errorf(
					"appendCalled = %v, want %v",
					tt.saver.appendCalled,
					tt.wantAppended,
				)
			}

			if !tt.wantSeenFlag {
				return
			}

			foundSeen := false
			for _, f := range tt.saver.appendedFlags {
				if f == imap.FlagSeen {
					foundSeen = true
					break
				}
			}
			if !foundSeen {
				t.Error(
					"expected \\Seen flag in " +
						"appended message",
				)
			}
		})
	}
}

// TestTrySaveToSent_SaveSentGuardIsCallerResponsibility
// documents that trySaveToSent always attempts to append
// when called — it is the caller's responsibility to
// check acct.SaveSent before calling trySaveToSent.
// This test verifies that a direct call (even when the
// intent would be SaveSent=false) always invokes Append.
func TestTrySaveToSent_SaveSentGuardIsCallerResponsibility(
	t *testing.T,
) {
	saver := &mockSentSaverForHelpers{sentMailbox: "Sent"}

	// Calling trySaveToSent unconditionally causes Append
	// to be invoked. The SaveSent flag is checked by the
	// caller before deciding whether to call this helper.
	got := trySaveToSent(saver, "test", []byte("msg"))

	if !got {
		t.Error("expected true from trySaveToSent")
	}
	if !saver.appendCalled {
		t.Error(
			"AppendMessage should have been called; " +
				"SaveSent guard is the caller's " +
				"responsibility",
		)
	}
}
