package tools

import (
	imap "github.com/emersion/go-imap/v2"
)

// messageCopier is a narrow interface for copying messages
// between mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type messageCopier interface {
	CopyMessages(
		account, mailbox string,
		uids []imap.UID,
		destMailbox string,
	) error
}

// NewCopyMessages creates a new copy-messages tool.
func NewCopyMessages(
	copier messageCopier,
) Tool {
	return newTransferTool(
		"copy", "Copied",
		copier.CopyMessages,
	)
}
