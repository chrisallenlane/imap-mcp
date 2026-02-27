package imap

import "testing"

func TestCreateMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.CreateMailbox("nonexistent", "NewFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestDeleteMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.DeleteMailbox("nonexistent", "OldFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}
