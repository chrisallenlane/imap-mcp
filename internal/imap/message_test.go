package imap

import (
	"errors"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

// --- StoreFlags ---

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

func TestStoreFlags_HappyPath(t *testing.T) {
	tests := []struct {
		name    string
		mailbox string
		uids    []imap.UID
		op      imap.StoreFlagsOp
		flags   []imap.Flag
	}{
		{
			name:    "add seen flag",
			mailbox: "INBOX",
			uids:    []imap.UID{1, 2, 3},
			op:      imap.StoreFlagsAdd,
			flags:   []imap.Flag{imap.FlagSeen},
		},
		{
			name:    "remove flagged",
			mailbox: "Archive",
			uids:    []imap.UID{42},
			op:      imap.StoreFlagsDel,
			flags:   []imap.Flag{imap.FlagFlagged},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestConnectionManager()
			mock := newMockImapClient()
			injectMock(mgr, "gmail", mock)

			err := mgr.StoreFlags(
				"gmail", tt.mailbox,
				tt.uids, tt.op, tt.flags,
			)
			if err != nil {
				t.Fatalf(
					"StoreFlags() unexpected error: %v",
					err,
				)
			}

			// Mailbox was selected read-write.
			if len(mock.selectCalls) != 1 {
				t.Fatalf(
					"Select called %d times, want 1",
					len(mock.selectCalls),
				)
			}
			sel := mock.selectCalls[0]
			if sel.mailbox != tt.mailbox {
				t.Errorf(
					"Select mailbox = %q, want %q",
					sel.mailbox,
					tt.mailbox,
				)
			}
			if sel.readOnly {
				t.Error(
					"Select readOnly = true, " +
						"want false for StoreFlags",
				)
			}

			// Store was called with correct UID set.
			if len(mock.storeCalls) != 1 {
				t.Fatalf(
					"Store called %d times, want 1",
					len(mock.storeCalls),
				)
			}
			sc := mock.storeCalls[0]
			if sc.op != tt.op {
				t.Errorf(
					"Store op = %v, want %v",
					sc.op,
					tt.op,
				)
			}
		})
	}
}

func TestStoreFlags_PropagatesSelectError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.selectErr = errors.New("mailbox not found")
	injectMock(mgr, "gmail", mock)

	err := mgr.StoreFlags(
		"gmail", "INBOX",
		[]imap.UID{1}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error from Select failure")
	}
}

func TestStoreFlags_PropagatesStoreError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.storeErr = errors.New("store rejected")
	injectMock(mgr, "gmail", mock)

	err := mgr.StoreFlags(
		"gmail", "INBOX",
		[]imap.UID{1}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error from Store failure")
	}
}

// --- MoveMessages ---

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

func TestMoveMessages_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	injectMock(mgr, "gmail", mock)

	uids := []imap.UID{10, 20}
	err := mgr.MoveMessages(
		"gmail", "INBOX", uids, "Trash",
	)
	if err != nil {
		t.Fatalf("MoveMessages() unexpected error: %v", err)
	}

	// Mailbox was selected read-write.
	if len(mock.selectCalls) != 1 {
		t.Fatalf(
			"Select called %d times, want 1",
			len(mock.selectCalls),
		)
	}
	if mock.selectCalls[0].readOnly {
		t.Error(
			"Select readOnly = true, want false for MoveMessages",
		)
	}
	if mock.selectCalls[0].mailbox != "INBOX" {
		t.Errorf(
			"Select mailbox = %q, want %q",
			mock.selectCalls[0].mailbox,
			"INBOX",
		)
	}

	// Move was issued.
	if len(mock.moveCalls) != 1 {
		t.Fatalf(
			"Move called %d times, want 1",
			len(mock.moveCalls),
		)
	}
	if mock.moveCalls[0].destMailbox != "Trash" {
		t.Errorf(
			"Move dest = %q, want %q",
			mock.moveCalls[0].destMailbox,
			"Trash",
		)
	}
}

func TestMoveMessages_PropagesMoveError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.moveErr = errors.New("MOVE failed")
	injectMock(mgr, "gmail", mock)

	err := mgr.MoveMessages(
		"gmail", "INBOX",
		[]imap.UID{1}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error from Move failure")
	}
}

// --- CopyMessages ---

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

func TestCopyMessages_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	injectMock(mgr, "gmail", mock)

	uids := []imap.UID{5}
	err := mgr.CopyMessages(
		"gmail", "INBOX", uids, "Archive",
	)
	if err != nil {
		t.Fatalf("CopyMessages() unexpected error: %v", err)
	}

	// Mailbox was selected read-write.
	if len(mock.selectCalls) != 1 {
		t.Fatalf(
			"Select called %d times, want 1",
			len(mock.selectCalls),
		)
	}
	if mock.selectCalls[0].readOnly {
		t.Error(
			"Select readOnly = true, want false for CopyMessages",
		)
	}

	// Copy was issued.
	if len(mock.copyCalls) != 1 {
		t.Fatalf(
			"Copy called %d times, want 1",
			len(mock.copyCalls),
		)
	}
	if mock.copyCalls[0].destMailbox != "Archive" {
		t.Errorf(
			"Copy dest = %q, want %q",
			mock.copyCalls[0].destMailbox,
			"Archive",
		)
	}
}

func TestCopyMessages_PropagatesCopyError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.copyErr = errors.New("COPY failed")
	injectMock(mgr, "gmail", mock)

	err := mgr.CopyMessages(
		"gmail", "INBOX",
		[]imap.UID{1}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error from Copy failure")
	}
}

// --- ExpungeMessages ---

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

func TestExpungeMessages_HappyPath(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	injectMock(mgr, "gmail", mock)

	uids := []imap.UID{1, 2}
	err := mgr.ExpungeMessages("gmail", "INBOX", uids)
	if err != nil {
		t.Fatalf(
			"ExpungeMessages() unexpected error: %v",
			err,
		)
	}

	// Mailbox was selected read-write.
	if len(mock.selectCalls) != 1 {
		t.Fatalf(
			"Select called %d times, want 1",
			len(mock.selectCalls),
		)
	}
	if mock.selectCalls[0].readOnly {
		t.Error(
			"Select readOnly = true, want false for ExpungeMessages",
		)
	}

	// Store was called first to set \Deleted.
	if len(mock.storeCalls) != 1 {
		t.Fatalf(
			"Store called %d times, want 1",
			len(mock.storeCalls),
		)
	}
	sc := mock.storeCalls[0]
	if sc.op != imap.StoreFlagsAdd {
		t.Errorf(
			"Store op = %v, want StoreFlagsAdd",
			sc.op,
		)
	}
	if len(sc.flags) != 1 || sc.flags[0] != imap.FlagDeleted {
		t.Errorf(
			"Store flags = %v, want [\\Deleted]",
			sc.flags,
		)
	}

	// UIDExpunge was called after Store.
	if len(mock.expungeCalls) != 1 {
		t.Fatalf(
			"UIDExpunge called %d times, want 1",
			len(mock.expungeCalls),
		)
	}
}

func TestExpungeMessages_PropagatesStoreError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.storeErr = errors.New("STORE failed")
	injectMock(mgr, "gmail", mock)

	err := mgr.ExpungeMessages(
		"gmail", "INBOX", []imap.UID{1},
	)
	if err == nil {
		t.Fatal("expected error from Store failure")
	}
	// UIDExpunge should not have been attempted.
	if len(mock.expungeCalls) != 0 {
		t.Error(
			"UIDExpunge called despite Store failure",
		)
	}
}

func TestExpungeMessages_PropagatesExpungeError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.expungeErr = errors.New("EXPUNGE failed")
	injectMock(mgr, "gmail", mock)

	err := mgr.ExpungeMessages(
		"gmail", "INBOX", []imap.UID{1},
	)
	if err == nil {
		t.Fatal("expected error from UIDExpunge failure")
	}
}

// --- AppendMessage ---

func TestAppendMessage_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	err := mgr.AppendMessage(
		"nonexistent", "INBOX",
		[]byte("data"), nil,
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestAppendMessage_HappyPath(t *testing.T) {
	tests := []struct {
		name    string
		mailbox string
		msg     []byte
		flags   []imap.Flag
	}{
		{
			name:    "append to drafts with draft flag",
			mailbox: "Drafts",
			msg:     []byte("Subject: test\r\n\r\nbody"),
			flags:   []imap.Flag{imap.FlagDraft},
		},
		{
			name:    "append to sent without flags",
			mailbox: "Sent",
			msg:     []byte("Subject: sent\r\n\r\nbody"),
			flags:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestConnectionManager()
			mock := newMockImapClient()
			injectMock(mgr, "gmail", mock)

			err := mgr.AppendMessage(
				"gmail", tt.mailbox, tt.msg, tt.flags,
			)
			if err != nil {
				t.Fatalf(
					"AppendMessage() unexpected error: %v",
					err,
				)
			}

			if len(mock.appendCalls) != 1 {
				t.Fatalf(
					"Append called %d times, want 1",
					len(mock.appendCalls),
				)
			}
			ac := mock.appendCalls[0]
			if ac.mailbox != tt.mailbox {
				t.Errorf(
					"Append mailbox = %q, want %q",
					ac.mailbox,
					tt.mailbox,
				)
			}
			if string(ac.data) != string(tt.msg) {
				t.Errorf(
					"Append data = %q, want %q",
					ac.data,
					tt.msg,
				)
			}
		})
	}
}

func TestAppendMessage_PropagatesWriteError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.appendErr = errors.New("write failed")
	injectMock(mgr, "gmail", mock)

	err := mgr.AppendMessage(
		"gmail", "INBOX",
		[]byte("data"), nil,
	)
	if err == nil {
		t.Fatal("expected error from write failure")
	}
}

func TestAppendMessage_PropagatesWaitError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.appendWaitErr = errors.New("server rejected append")
	injectMock(mgr, "gmail", mock)

	err := mgr.AppendMessage(
		"gmail", "INBOX",
		[]byte("data"), nil,
	)
	if err == nil {
		t.Fatal("expected error from Wait failure")
	}
}

// --- FindTrashMailbox ---

func TestFindTrashMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()
	_, err := mgr.FindTrashMailbox("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

// --- findSpecialMailbox (shared helper for Find* methods) ---

func TestFindSpecialMailbox_HappyPath(t *testing.T) {
	tests := []struct {
		name        string
		listData    []*imap.ListData
		findFn      func(*ConnectionManager, string) (string, error)
		wantMailbox string
	}{
		{
			name: "FindTrashMailbox finds \\Trash",
			listData: []*imap.ListData{
				{Mailbox: "INBOX", Attrs: []imap.MailboxAttr{}},
				{
					Mailbox: "Trash",
					Attrs:   []imap.MailboxAttr{imap.MailboxAttrTrash},
				},
			},
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindTrashMailbox(acct)
			},
			wantMailbox: "Trash",
		},
		{
			name: "FindSentMailbox finds \\Sent",
			listData: []*imap.ListData{
				{Mailbox: "INBOX", Attrs: []imap.MailboxAttr{}},
				{
					Mailbox: "Sent",
					Attrs:   []imap.MailboxAttr{imap.MailboxAttrSent},
				},
			},
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindSentMailbox(acct)
			},
			wantMailbox: "Sent",
		},
		{
			name: "FindDraftsMailbox finds \\Drafts",
			listData: []*imap.ListData{
				{Mailbox: "INBOX", Attrs: []imap.MailboxAttr{}},
				{
					Mailbox: "Drafts",
					Attrs:   []imap.MailboxAttr{imap.MailboxAttrDrafts},
				},
			},
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindDraftsMailbox(acct)
			},
			wantMailbox: "Drafts",
		},
		{
			name: "FindTrashMailbox with multiple mailboxes",
			listData: []*imap.ListData{
				{Mailbox: "INBOX", Attrs: []imap.MailboxAttr{}},
				{
					Mailbox: "Sent",
					Attrs:   []imap.MailboxAttr{imap.MailboxAttrSent},
				},
				{
					Mailbox: "Deleted Items",
					Attrs:   []imap.MailboxAttr{imap.MailboxAttrTrash},
				},
				{
					Mailbox: "Archive",
					Attrs:   []imap.MailboxAttr{imap.MailboxAttrArchive},
				},
			},
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindTrashMailbox(acct)
			},
			wantMailbox: "Deleted Items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestConnectionManager()
			mock := newMockImapClient()
			mock.listData = tt.listData
			injectMock(mgr, "gmail", mock)

			got, err := tt.findFn(mgr, "gmail")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantMailbox {
				t.Errorf(
					"got mailbox %q, want %q",
					got,
					tt.wantMailbox,
				)
			}
		})
	}
}

func TestFindSpecialMailbox_NotFound(t *testing.T) {
	tests := []struct {
		name   string
		findFn func(*ConnectionManager, string) (string, error)
	}{
		{
			name: "FindTrashMailbox no trash",
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindTrashMailbox(acct)
			},
		},
		{
			name: "FindSentMailbox no sent",
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindSentMailbox(acct)
			},
		},
		{
			name: "FindDraftsMailbox no drafts",
			findFn: func(
				m *ConnectionManager,
				acct string,
			) (string, error) {
				return m.FindDraftsMailbox(acct)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := newTestConnectionManager()
			mock := newMockImapClient()
			// Only INBOX with no special-use attributes.
			mock.listData = []*imap.ListData{
				{
					Mailbox: "INBOX",
					Attrs:   []imap.MailboxAttr{},
				},
			}
			injectMock(mgr, "gmail", mock)

			_, err := tt.findFn(mgr, "gmail")
			if err == nil {
				t.Fatal(
					"expected error when no matching mailbox",
				)
			}
		})
	}
}

func TestFindSpecialMailbox_ListError(t *testing.T) {
	mgr := newTestConnectionManager()
	mock := newMockImapClient()
	mock.listErr = errors.New("LIST failed")
	injectMock(mgr, "gmail", mock)

	_, err := mgr.FindTrashMailbox("gmail")
	if err == nil {
		t.Fatal("expected error when List fails")
	}
}
