package imap

import (
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	imap "github.com/emersion/go-imap/v2"
)

func TestClose_NoConnections(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{},
	}
	mgr := NewManager(cfg)

	if err := mgr.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
}

func TestGetClient_UnknownAccountErrorMessage(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"gmail": {
				Host:     "imap.gmail.com",
				Port:     993,
				Username: "user@gmail.com",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	mgr := NewManager(cfg)

	_, err := mgr.GetClient("doesnotexist")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}

	if !strings.Contains(
		err.Error(),
		"unknown account",
	) {
		t.Errorf(
			"error = %q, should mention 'unknown account'",
			err.Error(),
		)
	}

	if !strings.Contains(err.Error(), "doesnotexist") {
		t.Errorf(
			"error = %q, should mention account name",
			err.Error(),
		)
	}
}

func TestIsConnected_UnknownAccount(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"gmail": {
				Host:     "imap.gmail.com",
				Port:     993,
				Username: "user@gmail.com",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	mgr := NewManager(cfg)

	if mgr.IsConnected("nonexistent") {
		t.Error(
			"IsConnected() should return false " +
				"for unknown account",
		)
	}
}

func TestIsConnected_KnownButNotConnected(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"gmail": {
				Host:     "imap.gmail.com",
				Port:     993,
				Username: "user@gmail.com",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	mgr := NewManager(cfg)

	// Account exists in config but GetClient was never called,
	// so no connection should exist.
	if mgr.IsConnected("gmail") {
		t.Error(
			"IsConnected() should return false " +
				"for known but unconnected account",
		)
	}
}

// newTestManager creates a Manager with one configured account
// but no live connections. Useful for testing input validation.
func newTestManager() *Manager {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"gmail": {
				Host:     "imap.gmail.com",
				Port:     993,
				Username: "user@gmail.com",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	return NewManager(cfg)
}

func TestSelectMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.SelectMailbox("nonexistent", "INBOX")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestStoreFlags_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.StoreFlags(
		"nonexistent", "INBOX",
		[]imap.UID{1}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestStoreFlags_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.StoreFlags(
		"gmail", "INBOX",
		[]imap.UID{}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
	if !strings.Contains(err.Error(), "no UIDs") {
		t.Errorf(
			"error = %q, want 'no UIDs'",
			err.Error(),
		)
	}
}

func TestMoveMessages_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.MoveMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestMoveMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.MoveMessages(
		"gmail", "INBOX",
		[]imap.UID{}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
	if !strings.Contains(err.Error(), "no UIDs") {
		t.Errorf(
			"error = %q, want 'no UIDs'",
			err.Error(),
		)
	}
}

func TestCopyMessages_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.CopyMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestCopyMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.CopyMessages(
		"gmail", "INBOX",
		[]imap.UID{}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
	if !strings.Contains(err.Error(), "no UIDs") {
		t.Errorf(
			"error = %q, want 'no UIDs'",
			err.Error(),
		)
	}
}

func TestExpungeMessages_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.ExpungeMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestExpungeMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.ExpungeMessages(
		"gmail", "INBOX",
		[]imap.UID{},
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
	if !strings.Contains(err.Error(), "no UIDs") {
		t.Errorf(
			"error = %q, want 'no UIDs'",
			err.Error(),
		)
	}
}

func TestCreateMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.CreateMailbox("nonexistent", "NewFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestDeleteMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.DeleteMailbox("nonexistent", "OldFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}

func TestFindTrashMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.FindTrashMailbox("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	// GetClient fails first, so the error surfaces from
	// ListMailboxes wrapping.
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error = %q, want 'unknown account'",
			err.Error(),
		)
	}
}
