package imap

import (
	"fmt"
	"slices"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// FetchMessages fetches message data for the given sequence
// set with the given options.
func (m *ConnectionManager) FetchMessages(
	account string,
	seqSet imap.SeqSet,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	return withRetryResult(
		m,
		account,
		func(c imapClient) ([]*imapclient.FetchMessageBuffer, error) {
			return c.Fetch(seqSet, options).Collect()
		},
	)
}

// SearchMessages selects a mailbox in read-only mode and
// runs an IMAP UID SEARCH with the given criteria.
func (m *ConnectionManager) SearchMessages(
	account, mailbox string,
	criteria *imap.SearchCriteria,
) ([]imap.UID, error) {
	return withRetryResult(
		m,
		account,
		func(c imapClient) ([]imap.UID, error) {
			if _, err := selectMailbox(c, mailbox, true); err != nil {
				return nil, err
			}
			searchData, err := c.UIDSearch(
				criteria, nil,
			).Wait()
			if err != nil {
				return nil, fmt.Errorf(
					"search failed: %w",
					err,
				)
			}
			return searchData.AllUIDs(), nil
		},
	)
}

// FetchMessagesByUID fetches messages for multiple UIDs
// in the specified mailbox.
func (m *ConnectionManager) FetchMessagesByUID(
	account, mailbox string,
	uids []imap.UID,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	return withRetryResult(
		m,
		account,
		func(c imapClient) ([]*imapclient.FetchMessageBuffer, error) {
			if _, err := selectMailbox(c, mailbox, true); err != nil {
				return nil, err
			}
			uidSet := imap.UIDSetNum(uids...)
			return c.Fetch(uidSet, options).Collect()
		},
	)
}

// StoreFlags sets or clears flags on messages identified by UIDs.
func (m *ConnectionManager) StoreFlags(
	account, mailbox string,
	uids []imap.UID,
	op imap.StoreFlagsOp,
	flags []imap.Flag,
) error {
	if len(uids) == 0 {
		return fmt.Errorf("no UIDs provided")
	}

	return m.withRetry(
		account,
		func(c imapClient) error {
			if _, err := selectMailbox(c, mailbox, false); err != nil {
				return err
			}

			uidSet := imap.UIDSetNum(uids...)
			storeFlags := &imap.StoreFlags{
				Op:     op,
				Silent: true,
				Flags:  flags,
			}

			cmd := c.Store(uidSet, storeFlags, nil)
			if err := cmd.Close(); err != nil {
				return fmt.Errorf(
					"failed to store flags: %w",
					err,
				)
			}

			return nil
		},
	)
}

// MoveMessages moves messages identified by UIDs from one mailbox
// to another.
func (m *ConnectionManager) MoveMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	return m.transferMessages(
		account, mailbox, uids, destMailbox, "move",
		func(c imapClient, s imap.UIDSet, d string) error {
			_, err := c.Move(s, d).Wait()
			return err
		},
	)
}

// CopyMessages copies messages identified by UIDs from one mailbox
// to another.
func (m *ConnectionManager) CopyMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	return m.transferMessages(
		account, mailbox, uids, destMailbox, "copy",
		func(c imapClient, s imap.UIDSet, d string) error {
			_, err := c.Copy(s, d).Wait()
			return err
		},
	)
}

// transferMessages is a shared helper for move/copy operations.
func (m *ConnectionManager) transferMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox, verb string,
	op func(imapClient, imap.UIDSet, string) error,
) error {
	if len(uids) == 0 {
		return fmt.Errorf("no UIDs provided")
	}

	return m.withRetry(
		account,
		func(c imapClient) error {
			if _, err := selectMailbox(c, mailbox, false); err != nil {
				return err
			}

			uidSet := imap.UIDSetNum(uids...)
			if err := op(c, uidSet, destMailbox); err != nil {
				return fmt.Errorf(
					"failed to %s messages: %w",
					verb,
					err,
				)
			}

			return nil
		},
	)
}

// ExpungeMessages permanently removes messages identified by UIDs
// from a mailbox.
func (m *ConnectionManager) ExpungeMessages(
	account, mailbox string,
	uids []imap.UID,
) error {
	if len(uids) == 0 {
		return fmt.Errorf("no UIDs provided")
	}

	return m.withRetry(
		account,
		func(c imapClient) error {
			if _, err := selectMailbox(c, mailbox, false); err != nil {
				return err
			}

			uidSet := imap.UIDSetNum(uids...)

			// Set \Deleted flag first -- UID EXPUNGE (RFC 4315)
			// only removes messages that already have \Deleted.
			storeFlags := &imap.StoreFlags{
				Op:     imap.StoreFlagsAdd,
				Silent: true,
				Flags:  []imap.Flag{imap.FlagDeleted},
			}
			if err := c.Store(
				uidSet, storeFlags, nil,
			).Close(); err != nil {
				return fmt.Errorf(
					"failed to mark messages as deleted: %w",
					err,
				)
			}

			if err := c.UIDExpunge(uidSet).Close(); err != nil {
				return fmt.Errorf(
					"failed to expunge messages: %w",
					err,
				)
			}

			return nil
		},
	)
}

// FindTrashMailbox scans the account's mailboxes for one with
// the \Trash special-use attribute and returns its name.
func (m *ConnectionManager) FindTrashMailbox(
	account string,
) (string, error) {
	return m.findSpecialMailbox(
		account,
		imap.MailboxAttrTrash,
		"trash",
	)
}

// FindSentMailbox scans the account's mailboxes for one with
// the \Sent special-use attribute and returns its name.
func (m *ConnectionManager) FindSentMailbox(
	account string,
) (string, error) {
	return m.findSpecialMailbox(
		account,
		imap.MailboxAttrSent,
		"sent",
	)
}

// FindDraftsMailbox scans the account's mailboxes for one with
// the \Drafts special-use attribute and returns its name.
func (m *ConnectionManager) FindDraftsMailbox(
	account string,
) (string, error) {
	return m.findSpecialMailbox(
		account,
		imap.MailboxAttrDrafts,
		"drafts",
	)
}

// findSpecialMailbox scans the account's mailboxes for one with
// the given special-use attribute and returns its name.
func (m *ConnectionManager) findSpecialMailbox(
	account string,
	attr imap.MailboxAttr,
	label string,
) (string, error) {
	mailboxes, err := m.ListMailboxes(account)
	if err != nil {
		return "", fmt.Errorf(
			"failed to list mailboxes: %w",
			err,
		)
	}

	for _, mb := range mailboxes {
		if slices.Contains(mb.Attrs, attr) {
			return mb.Mailbox, nil
		}
	}

	return "", fmt.Errorf(
		"no %s mailbox found for account %q",
		label,
		account,
	)
}

// AppendMessage appends a message to the specified mailbox
// with the given flags via IMAP APPEND.
func (m *ConnectionManager) AppendMessage(
	account, mailbox string,
	msg []byte,
	flags []imap.Flag,
) error {
	return m.withRetry(
		account,
		func(c imapClient) error {
			size := int64(len(msg))
			appendCmd := c.Append(
				mailbox,
				size,
				&imap.AppendOptions{Flags: flags},
			)

			if _, err := appendCmd.Write(msg); err != nil {
				return fmt.Errorf(
					"failed to write append data: %w",
					err,
				)
			}

			if err := appendCmd.Close(); err != nil {
				return fmt.Errorf(
					"failed to close append command: %w",
					err,
				)
			}

			if _, err := appendCmd.Wait(); err != nil {
				return fmt.Errorf(
					"failed to append message: %w",
					err,
				)
			}

			return nil
		},
	)
}
