package imap

import (
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

func TestStoreFlags_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.StoreFlags(
		"nonexistent", "INBOX",
		[]imap.UID{1}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestStoreFlags_EmptyUIDs(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.StoreFlags(
		"gmail", "INBOX",
		[]imap.UID{}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestMoveMessages_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.MoveMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestMoveMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.MoveMessages(
		"gmail", "INBOX",
		[]imap.UID{}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestCopyMessages_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.CopyMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestCopyMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.CopyMessages(
		"gmail", "INBOX",
		[]imap.UID{}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestExpungeMessages_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.ExpungeMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestExpungeMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.ExpungeMessages(
		"gmail", "INBOX",
		[]imap.UID{},
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestFindTrashMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	_, err := mgr.FindTrashMailbox("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}
