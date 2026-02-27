package tools

import (
	imap "github.com/emersion/go-imap/v2"
)

// messageMover is a narrow interface for moving messages
// between mailboxes.
// *imapmanager.Manager satisfies this implicitly.
type messageMover interface {
	MoveMessages(
		account, mailbox string,
		uids []imap.UID,
		destMailbox string,
	) error
}

// NewMoveMessages creates a new move-messages tool.
func NewMoveMessages(
	mover messageMover,
) Tool {
	return newTransferTool(
		"move", "Moved",
		mover.MoveMessages,
	)
}
