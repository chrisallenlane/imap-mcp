// Package imap manages persistent IMAP client connections.
package imap

import (
	"fmt"
	"log"
	"sync"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Manager maintains persistent IMAP connections per account.
type Manager struct {
	config *config.Config
	conns  map[string]*imapclient.Client
	mu     sync.Mutex
}

// NewManager creates a new connection manager from the given config.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config: cfg,
		conns:  make(map[string]*imapclient.Client),
	}
}

// GetClient returns an IMAP client for the named account,
// connecting lazily on first use. If a cached connection is
// dead, it is evicted and a fresh connection is established.
func (m *Manager) GetClient(
	accountName string,
) (*imapclient.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.conns[accountName]; ok {
		if !isConnectionClosed(c) {
			return c, nil
		}

		// Connection is dead -- close and evict.
		log.Printf(
			"connection to %q lost, reconnecting...",
			accountName,
		)
		c.Close()
		delete(m.conns, accountName)
	}

	acct, ok := m.config.Accounts[accountName]
	if !ok {
		return nil, fmt.Errorf(
			"unknown account: %q",
			accountName,
		)
	}

	c, err := connect(acct)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to connect to %q: %w",
			accountName,
			err,
		)
	}

	m.conns[accountName] = c
	return c, nil
}

// IsConnected reports whether the named account currently has an
// open IMAP connection. It does not attempt to connect. Returns
// false if the cached connection is dead.
func (m *Manager) IsConnected(accountName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[accountName]
	if !ok {
		return false
	}
	return !isConnectionClosed(c)
}

// Config returns the manager's configuration.
func (m *Manager) Config() *config.Config {
	return m.config
}

// isConnectionClosed performs a fast, non-blocking check on
// whether the client's connection has been closed.
func isConnectionClosed(c *imapclient.Client) bool {
	select {
	case <-c.Closed():
		return true
	default:
		return false
	}
}

// withRetry executes fn with an IMAP client. If fn returns an
// error and the connection appears dead, it reconnects once and
// retries. Only a single retry is attempted.
func (m *Manager) withRetry(
	account string,
	fn func(c *imapclient.Client) error,
) error {
	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := fn(client); err != nil {
		if !isConnectionClosed(client) {
			return err
		}

		// Evict the dead connection under lock.
		m.mu.Lock()
		if m.conns[account] == client {
			client.Close()
			delete(m.conns, account)
		}
		m.mu.Unlock()

		log.Printf(
			"connection to %q lost, reconnecting...",
			account,
		)

		// GetClient will establish a new connection.
		client, err = m.GetClient(account)
		if err != nil {
			return fmt.Errorf("reconnect failed: %w", err)
		}

		return fn(client)
	}

	return nil
}

// withRetryResult is like withRetry but for operations that
// return a value alongside an error.
func withRetryResult[T any](
	m *Manager,
	account string,
	fn func(c *imapclient.Client) (T, error),
) (T, error) {
	client, err := m.GetClient(account)
	if err != nil {
		var zero T
		return zero, err
	}

	result, err := fn(client)
	if err != nil {
		if !isConnectionClosed(client) {
			var zero T
			return zero, err
		}

		// Evict the dead connection under lock.
		m.mu.Lock()
		if m.conns[account] == client {
			client.Close()
			delete(m.conns, account)
		}
		m.mu.Unlock()

		log.Printf(
			"connection to %q lost, reconnecting...",
			account,
		)

		// GetClient will establish a new connection.
		client, err = m.GetClient(account)
		if err != nil {
			var zero T
			return zero, fmt.Errorf(
				"reconnect failed: %w",
				err,
			)
		}

		return fn(client)
	}

	return result, nil
}

// ListMailboxes returns all mailboxes for the named account.
// It connects lazily if needed, then issues an IMAP LIST command.
func (m *Manager) ListMailboxes(
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
func (m *Manager) ExamineMailbox(
	account, mailbox string,
) (*imap.SelectData, error) {
	return withRetryResult(
		m,
		account,
		func(c *imapclient.Client) (*imap.SelectData, error) {
			return c.Select(
				mailbox,
				&imap.SelectOptions{ReadOnly: true},
			).Wait()
		},
	)
}

// FetchMessages fetches message data for the given sequence
// set with the given options.
func (m *Manager) FetchMessages(
	account string,
	seqSet imap.SeqSet,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	return withRetryResult(
		m,
		account,
		func(
			c *imapclient.Client,
		) ([]*imapclient.FetchMessageBuffer, error) {
			return c.Fetch(seqSet, options).Collect()
		},
	)
}

// FetchMessageByUID fetches message data for the given UID
// in the specified mailbox.
func (m *Manager) FetchMessageByUID(
	account string,
	mailbox string,
	uid imap.UID,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	return withRetryResult(
		m,
		account,
		func(
			c *imapclient.Client,
		) ([]*imapclient.FetchMessageBuffer, error) {
			if err := selectReadOnly(c, mailbox); err != nil {
				return nil, err
			}
			uidSet := imap.UIDSetNum(uid)
			return c.Fetch(uidSet, options).Collect()
		},
	)
}

// SearchMessages selects a mailbox in read-only mode and
// runs an IMAP UID SEARCH with the given criteria.
func (m *Manager) SearchMessages(
	account, mailbox string,
	criteria *imap.SearchCriteria,
) ([]imap.UID, error) {
	return withRetryResult(
		m,
		account,
		func(c *imapclient.Client) ([]imap.UID, error) {
			if err := selectReadOnly(c, mailbox); err != nil {
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
func (m *Manager) FetchMessagesByUID(
	account, mailbox string,
	uids []imap.UID,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	return withRetryResult(
		m,
		account,
		func(
			c *imapclient.Client,
		) ([]*imapclient.FetchMessageBuffer, error) {
			if err := selectReadOnly(c, mailbox); err != nil {
				return nil, err
			}
			uidSet := imap.UIDSetNum(uids...)
			return c.Fetch(uidSet, options).Collect()
		},
	)
}

// selectReadOnly selects a mailbox in read-only mode.
func selectReadOnly(
	client *imapclient.Client,
	mailbox string,
) error {
	_, err := client.Select(
		mailbox,
		&imap.SelectOptions{ReadOnly: true},
	).Wait()
	if err != nil {
		return fmt.Errorf(
			"failed to examine mailbox: %w",
			err,
		)
	}
	return nil
}

// selectReadWrite selects a mailbox in read-write mode.
func selectReadWrite(
	client *imapclient.Client,
	mailbox string,
) error {
	_, err := client.Select(
		mailbox,
		&imap.SelectOptions{ReadOnly: false},
	).Wait()
	if err != nil {
		return fmt.Errorf(
			"failed to select mailbox: %w",
			err,
		)
	}
	return nil
}

// SelectMailbox opens a mailbox in read-write mode (IMAP SELECT)
// and returns metadata.
func (m *Manager) SelectMailbox(
	account, mailbox string,
) (*imap.SelectData, error) {
	return withRetryResult(
		m,
		account,
		func(c *imapclient.Client) (*imap.SelectData, error) {
			return c.Select(
				mailbox,
				&imap.SelectOptions{ReadOnly: false},
			).Wait()
		},
	)
}

// StoreFlags sets or clears flags on messages identified by UIDs.
func (m *Manager) StoreFlags(
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
		func(c *imapclient.Client) error {
			if err := selectReadWrite(c, mailbox); err != nil {
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
func (m *Manager) MoveMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	if len(uids) == 0 {
		return fmt.Errorf("no UIDs provided")
	}

	return m.withRetry(
		account,
		func(c *imapclient.Client) error {
			if err := selectReadWrite(c, mailbox); err != nil {
				return err
			}

			uidSet := imap.UIDSetNum(uids...)
			if _, err := c.Move(
				uidSet, destMailbox,
			).Wait(); err != nil {
				return fmt.Errorf(
					"failed to move messages: %w",
					err,
				)
			}

			return nil
		},
	)
}

// CopyMessages copies messages identified by UIDs from one mailbox
// to another.
func (m *Manager) CopyMessages(
	account, mailbox string,
	uids []imap.UID,
	destMailbox string,
) error {
	if len(uids) == 0 {
		return fmt.Errorf("no UIDs provided")
	}

	return m.withRetry(
		account,
		func(c *imapclient.Client) error {
			if err := selectReadWrite(c, mailbox); err != nil {
				return err
			}

			uidSet := imap.UIDSetNum(uids...)
			if _, err := c.Copy(
				uidSet, destMailbox,
			).Wait(); err != nil {
				return fmt.Errorf(
					"failed to copy messages: %w",
					err,
				)
			}

			return nil
		},
	)
}

// ExpungeMessages permanently removes messages identified by UIDs
// from a mailbox.
func (m *Manager) ExpungeMessages(
	account, mailbox string,
	uids []imap.UID,
) error {
	if len(uids) == 0 {
		return fmt.Errorf("no UIDs provided")
	}

	return m.withRetry(
		account,
		func(c *imapclient.Client) error {
			if err := selectReadWrite(c, mailbox); err != nil {
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

// CreateMailbox creates a new mailbox on the server.
func (m *Manager) CreateMailbox(
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
func (m *Manager) DeleteMailbox(
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

// FindTrashMailbox scans the account's mailboxes for one with
// the \Trash special-use attribute and returns its name.
func (m *Manager) FindTrashMailbox(
	account string,
) (string, error) {
	mailboxes, err := m.ListMailboxes(account)
	if err != nil {
		return "", fmt.Errorf(
			"failed to list mailboxes: %w",
			err,
		)
	}

	for _, mb := range mailboxes {
		for _, attr := range mb.Attrs {
			if attr == imap.MailboxAttrTrash {
				return mb.Mailbox, nil
			}
		}
	}

	return "", fmt.Errorf(
		"no trash mailbox found for account %q",
		account,
	)
}

// Close closes all open IMAP connections.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for name, c := range m.conns {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf(
				"failed to close %q: %w",
				name,
				err,
			)
		}
		delete(m.conns, name)
	}
	return firstErr
}

// connect dials and authenticates an IMAP connection for the
// given account.
func connect(acct config.Account) (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", acct.Host, acct.Port)

	var c *imapclient.Client
	var err error

	if acct.TLS {
		c, err = imapclient.DialTLS(addr, nil)
	} else {
		c, err = imapclient.DialInsecure(addr, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	if err := c.Login(acct.Username, acct.Password).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("login failed: %w", err)
	}

	return c, nil
}
