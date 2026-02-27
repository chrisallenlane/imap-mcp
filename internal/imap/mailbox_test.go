package imap

import (
	"errors"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

func TestCreateMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.CreateMailbox("nonexistent", "NewFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestCreateMailbox_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	injectMock(mgr, "gmail", mock)

	err := mgr.CreateMailbox("gmail", "NewFolder")
	if err != nil {
		t.Fatalf("CreateMailbox() unexpected error: %v", err)
	}

	if len(mock.createCalls) != 1 {
		t.Fatalf(
			"Create called %d times, want 1",
			len(mock.createCalls),
		)
	}
	if mock.createCalls[0] != "NewFolder" {
		t.Errorf(
			"Create mailbox = %q, want %q",
			mock.createCalls[0],
			"NewFolder",
		)
	}
}

func TestCreateMailbox_PropagatesError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.createErr = errors.New("CREATE rejected")
	injectMock(mgr, "gmail", mock)

	err := mgr.CreateMailbox("gmail", "NewFolder")
	if err == nil {
		t.Fatal("expected error from Create failure")
	}
}

func TestDeleteMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.DeleteMailbox("nonexistent", "OldFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestDeleteMailbox_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	injectMock(mgr, "gmail", mock)

	err := mgr.DeleteMailbox("gmail", "OldFolder")
	if err != nil {
		t.Fatalf("DeleteMailbox() unexpected error: %v", err)
	}

	if len(mock.deleteCalls) != 1 {
		t.Fatalf(
			"Delete called %d times, want 1",
			len(mock.deleteCalls),
		)
	}
	if mock.deleteCalls[0] != "OldFolder" {
		t.Errorf(
			"Delete mailbox = %q, want %q",
			mock.deleteCalls[0],
			"OldFolder",
		)
	}
}

func TestDeleteMailbox_PropagatesError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.deleteErr = errors.New("DELETE rejected")
	injectMock(mgr, "gmail", mock)

	err := mgr.DeleteMailbox("gmail", "OldFolder")
	if err == nil {
		t.Fatal("expected error from Delete failure")
	}
}

func TestListMailboxes_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.listData = []*imap.ListData{
		{Mailbox: "INBOX"},
		{
			Mailbox: "Sent",
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrSent},
		},
	}
	injectMock(mgr, "gmail", mock)

	got, err := mgr.ListMailboxes("gmail")
	if err != nil {
		t.Fatalf("ListMailboxes() unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf(
			"ListMailboxes() returned %d mailboxes, want 2",
			len(got),
		)
	}
	if got[0].Mailbox != "INBOX" {
		t.Errorf(
			"mailbox[0] = %q, want %q",
			got[0].Mailbox,
			"INBOX",
		)
	}
}

func TestListMailboxes_PropagatesError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.listErr = errors.New("LIST failed")
	injectMock(mgr, "gmail", mock)

	_, err := mgr.ListMailboxes("gmail")
	if err == nil {
		t.Fatal("expected error from List failure")
	}
}

func TestExamineMailbox_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.selectData = &imap.SelectData{NumMessages: 42}
	injectMock(mgr, "gmail", mock)

	data, err := mgr.ExamineMailbox("gmail", "INBOX")
	if err != nil {
		t.Fatalf(
			"ExamineMailbox() unexpected error: %v",
			err,
		)
	}
	if data == nil {
		t.Fatal("ExamineMailbox() returned nil data")
	}

	// Verify mailbox was selected read-only.
	if len(mock.selectCalls) != 1 {
		t.Fatalf(
			"Select called %d times, want 1",
			len(mock.selectCalls),
		)
	}
	if !mock.selectCalls[0].readOnly {
		t.Error(
			"Select readOnly = false, " +
				"want true for ExamineMailbox",
		)
	}
	if mock.selectCalls[0].mailbox != "INBOX" {
		t.Errorf(
			"Select mailbox = %q, want %q",
			mock.selectCalls[0].mailbox,
			"INBOX",
		)
	}
}

func TestExamineMailbox_PropagatesError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.selectErr = errors.New("mailbox not found")
	injectMock(mgr, "gmail", mock)

	_, err := mgr.ExamineMailbox("gmail", "Nonexistent")
	if err == nil {
		t.Fatal("expected error from Select failure")
	}
}

func TestMailboxStatus_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	numMsg := uint32(10)
	numUnseen := uint32(3)
	mock.statusData = &imap.StatusData{
		NumMessages: &numMsg,
		NumUnseen:   &numUnseen,
	}
	injectMock(mgr, "gmail", mock)

	data, err := mgr.MailboxStatus("gmail", "INBOX")
	if err != nil {
		t.Fatalf(
			"MailboxStatus() unexpected error: %v",
			err,
		)
	}
	if data == nil {
		t.Fatal("MailboxStatus() returned nil data")
	}
}

func TestMailboxStatus_PropagatesError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.statusErr = errors.New("STATUS failed")
	injectMock(mgr, "gmail", mock)

	_, err := mgr.MailboxStatus("gmail", "INBOX")
	if err == nil {
		t.Fatal("expected error from Status failure")
	}
}
