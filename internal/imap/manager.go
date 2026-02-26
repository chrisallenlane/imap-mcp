// Package imap manages persistent IMAP client connections.
package imap

import (
	"fmt"
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
// connecting lazily on first use.
func (m *Manager) GetClient(
	accountName string,
) (*imapclient.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.conns[accountName]; ok {
		return c, nil
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
// open IMAP connection. It does not attempt to connect.
func (m *Manager) IsConnected(accountName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.conns[accountName]
	return ok
}

// Config returns the manager's configuration.
func (m *Manager) Config() *config.Config {
	return m.config
}

// ListMailboxes returns all mailboxes for the named account.
// It connects lazily if needed, then issues an IMAP LIST command.
func (m *Manager) ListMailboxes(
	account string,
) ([]*imap.ListData, error) {
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}

	listCmd := client.List("", "*", nil)
	return listCmd.Collect()
}

// ExamineMailbox selects a mailbox in read-only mode (EXAMINE)
// and returns metadata.
func (m *Manager) ExamineMailbox(
	account, mailbox string,
) (*imap.SelectData, error) {
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}
	return client.Select(
		mailbox,
		&imap.SelectOptions{ReadOnly: true},
	).Wait()
}

// FetchMessages fetches message data for the given sequence
// set with the given options.
func (m *Manager) FetchMessages(
	account string,
	seqSet imap.SeqSet,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}
	return client.Fetch(seqSet, options).Collect()
}

// FetchMessageByUID fetches message data for the given UID
// in the specified mailbox.
func (m *Manager) FetchMessageByUID(
	account string,
	mailbox string,
	uid imap.UID,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}

	if err := selectReadOnly(client, mailbox); err != nil {
		return nil, err
	}

	uidSet := imap.UIDSetNum(uid)
	return client.Fetch(uidSet, options).Collect()
}

// SearchMessages selects a mailbox in read-only mode and
// runs an IMAP UID SEARCH with the given criteria.
func (m *Manager) SearchMessages(
	account, mailbox string,
	criteria *imap.SearchCriteria,
) ([]imap.UID, error) {
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}

	if err := selectReadOnly(client, mailbox); err != nil {
		return nil, err
	}

	searchData, err := client.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return searchData.AllUIDs(), nil
}

// FetchMessagesByUID fetches messages for multiple UIDs
// in the specified mailbox.
func (m *Manager) FetchMessagesByUID(
	account, mailbox string,
	uids []imap.UID,
	options *imap.FetchOptions,
) ([]*imapclient.FetchMessageBuffer, error) {
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}

	if err := selectReadOnly(client, mailbox); err != nil {
		return nil, err
	}

	uidSet := imap.UIDSetNum(uids...)
	return client.Fetch(uidSet, options).Collect()
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
	client, err := m.GetClient(account)
	if err != nil {
		return nil, err
	}
	return client.Select(
		mailbox,
		&imap.SelectOptions{ReadOnly: false},
	).Wait()
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

	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := selectReadWrite(client, mailbox); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(uids...)
	storeFlags := &imap.StoreFlags{
		Op:     op,
		Silent: true,
		Flags:  flags,
	}

	cmd := client.Store(uidSet, storeFlags, nil)
	if err := cmd.Close(); err != nil {
		return fmt.Errorf("failed to store flags: %w", err)
	}

	return nil
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

	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := selectReadWrite(client, mailbox); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(uids...)
	if _, err := client.Move(uidSet, destMailbox).Wait(); err != nil {
		return fmt.Errorf(
			"failed to move messages: %w",
			err,
		)
	}

	return nil
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

	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := selectReadWrite(client, mailbox); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(uids...)
	if _, err := client.Copy(uidSet, destMailbox).Wait(); err != nil {
		return fmt.Errorf(
			"failed to copy messages: %w",
			err,
		)
	}

	return nil
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

	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := selectReadWrite(client, mailbox); err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(uids...)

	// Set \Deleted flag first — UID EXPUNGE (RFC 4315) only
	// removes messages that already have \Deleted.
	storeFlags := &imap.StoreFlags{
		Op:     imap.StoreFlagsAdd,
		Silent: true,
		Flags:  []imap.Flag{imap.FlagDeleted},
	}
	if err := client.Store(
		uidSet, storeFlags, nil,
	).Close(); err != nil {
		return fmt.Errorf(
			"failed to mark messages as deleted: %w",
			err,
		)
	}

	if err := client.UIDExpunge(uidSet).Close(); err != nil {
		return fmt.Errorf(
			"failed to expunge messages: %w",
			err,
		)
	}

	return nil
}

// CreateMailbox creates a new mailbox on the server.
func (m *Manager) CreateMailbox(
	account, name string,
) error {
	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := client.Create(name, nil).Wait(); err != nil {
		return fmt.Errorf(
			"failed to create mailbox: %w",
			err,
		)
	}

	return nil
}

// DeleteMailbox deletes a mailbox from the server.
func (m *Manager) DeleteMailbox(
	account, name string,
) error {
	client, err := m.GetClient(account)
	if err != nil {
		return err
	}

	if err := client.Delete(name).Wait(); err != nil {
		return fmt.Errorf(
			"failed to delete mailbox: %w",
			err,
		)
	}

	return nil
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
