package imap

import (
	"fmt"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// ListMailboxes returns all mailboxes for the named account.
// It connects lazily if needed, then issues an IMAP LIST command.
func (m *ConnectionManager) ListMailboxes(
	account string,
) ([]*imap.ListData, error) {
	return withRetryResult(
		m,
		account,
		func(c *imapclient.Client) ([]*imap.ListData, error) {
			return c.List("", "*", nil).Collect()
		},
	)
}

// ExamineMailbox selects a mailbox in read-only mode (EXAMINE)
// and returns metadata.
func (m *ConnectionManager) ExamineMailbox(
	account, mailbox string,
) (*imap.SelectData, error) {
	return withRetryResult(
		m,
		account,
		func(c *imapclient.Client) (*imap.SelectData, error) {
			return selectMailbox(c, mailbox, true)
		},
	)
}

// MailboxStatus issues an IMAP STATUS command for the named
// mailbox and returns message and unseen counts.
func (m *ConnectionManager) MailboxStatus(
	account, mailbox string,
) (*imap.StatusData, error) {
	return withRetryResult(
		m,
		account,
		func(
			c *imapclient.Client,
		) (*imap.StatusData, error) {
			return c.Status(
				mailbox,
				&imap.StatusOptions{
					NumMessages: true,
					NumUnseen:   true,
				},
			).Wait()
		},
	)
}

// CreateMailbox creates a new mailbox on the server.
func (m *ConnectionManager) CreateMailbox(
	account, name string,
) error {
	return m.withRetry(
		account,
		func(c *imapclient.Client) error {
			if err := c.Create(name, nil).Wait(); err != nil {
				return fmt.Errorf(
					"failed to create mailbox: %w",
					err,
				)
			}
			return nil
		},
	)
}

// DeleteMailbox deletes a mailbox from the server.
func (m *ConnectionManager) DeleteMailbox(
	account, name string,
) error {
	return m.withRetry(
		account,
		func(c *imapclient.Client) error {
			if err := c.Delete(name).Wait(); err != nil {
				return fmt.Errorf(
					"failed to delete mailbox: %w",
					err,
				)
			}
			return nil
		},
	)
}
