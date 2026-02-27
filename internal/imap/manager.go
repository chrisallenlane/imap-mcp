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

// The following narrow interfaces represent the result of each
// IMAP command.  Using interfaces here (rather than concrete
// command types from imapclient) allows tests to inject mock
// implementations without requiring a live IMAP server.

// selectResult is returned by Select.
type selectResult interface {
	Wait() (*imap.SelectData, error)
}

// storeResult is returned by Store.
type storeResult interface {
	Close() error
}

// moveResult is returned by Move.
type moveResult interface {
	Wait() (*imapclient.MoveData, error)
}

// copyResult is returned by Copy.
type copyResult interface {
	Wait() (*imap.CopyData, error)
}

// expungeResult is returned by UIDExpunge.
type expungeResult interface {
	Close() error
}

// appendResult is returned by Append.
type appendResult interface {
	Write(b []byte) (int, error)
	Close() error
	Wait() (*imap.AppendData, error)
}

// listResult is returned by List.
type listResult interface {
	Collect() ([]*imap.ListData, error)
}

// statusResult is returned by Status.
type statusResult interface {
	Wait() (*imap.StatusData, error)
}

// commandResult is returned by Create and Delete.
type commandResult interface {
	Wait() error
}

// fetchResult is returned by Fetch.
type fetchResult interface {
	Collect() ([]*imapclient.FetchMessageBuffer, error)
}

// searchResult is returned by UIDSearch.
type searchResult interface {
	Wait() (*imap.SearchData, error)
}

// imapClient is a narrow interface over *imapclient.Client.
// It covers all methods called by ConnectionManager operations.
// *imapclient.Client satisfies this via clientAdapter.
type imapClient interface {
	Select(
		mailbox string,
		options *imap.SelectOptions,
	) selectResult
	Store(
		numSet imap.NumSet,
		store *imap.StoreFlags,
		options *imap.StoreOptions,
	) storeResult
	Move(numSet imap.NumSet, mailbox string) moveResult
	Copy(numSet imap.NumSet, mailbox string) copyResult
	UIDExpunge(uids imap.UIDSet) expungeResult
	Append(
		mailbox string,
		size int64,
		options *imap.AppendOptions,
	) appendResult
	List(
		ref, pattern string,
		options *imap.ListOptions,
	) listResult
	Status(
		mailbox string,
		options *imap.StatusOptions,
	) statusResult
	Create(
		mailbox string,
		options *imap.CreateOptions,
	) commandResult
	Delete(mailbox string) commandResult
	Fetch(
		numSet imap.NumSet,
		options *imap.FetchOptions,
	) fetchResult
	UIDSearch(
		criteria *imap.SearchCriteria,
		options *imap.SearchOptions,
	) searchResult
	Closed() <-chan struct{}
	Close() error
}

// clientAdapter wraps *imapclient.Client so that it satisfies
// the imapClient interface.  The concrete library methods return
// specific command types; the adapter promotes them to the
// narrow result interfaces declared above.
type clientAdapter struct {
	*imapclient.Client
}

func (a *clientAdapter) Select(
	mailbox string,
	options *imap.SelectOptions,
) selectResult {
	return a.Client.Select(mailbox, options)
}

func (a *clientAdapter) Store(
	numSet imap.NumSet,
	store *imap.StoreFlags,
	options *imap.StoreOptions,
) storeResult {
	return a.Client.Store(numSet, store, options)
}

func (a *clientAdapter) Move(
	numSet imap.NumSet,
	mailbox string,
) moveResult {
	return a.Client.Move(numSet, mailbox)
}

func (a *clientAdapter) Copy(
	numSet imap.NumSet,
	mailbox string,
) copyResult {
	return a.Client.Copy(numSet, mailbox)
}

func (a *clientAdapter) UIDExpunge(
	uids imap.UIDSet,
) expungeResult {
	return a.Client.UIDExpunge(uids)
}

func (a *clientAdapter) Append(
	mailbox string,
	size int64,
	options *imap.AppendOptions,
) appendResult {
	return a.Client.Append(mailbox, size, options)
}

func (a *clientAdapter) List(
	ref, pattern string,
	options *imap.ListOptions,
) listResult {
	return a.Client.List(ref, pattern, options)
}

func (a *clientAdapter) Status(
	mailbox string,
	options *imap.StatusOptions,
) statusResult {
	return a.Client.Status(mailbox, options)
}

func (a *clientAdapter) Create(
	mailbox string,
	options *imap.CreateOptions,
) commandResult {
	return a.Client.Create(mailbox, options)
}

func (a *clientAdapter) Delete(mailbox string) commandResult {
	return a.Client.Delete(mailbox)
}

func (a *clientAdapter) Fetch(
	numSet imap.NumSet,
	options *imap.FetchOptions,
) fetchResult {
	return a.Client.Fetch(numSet, options)
}

func (a *clientAdapter) UIDSearch(
	criteria *imap.SearchCriteria,
	options *imap.SearchOptions,
) searchResult {
	return a.Client.UIDSearch(criteria, options)
}

// ConnectionManager maintains persistent IMAP connections per account.
type ConnectionManager struct {
	config *config.Config
	conns  map[string]imapClient
	mu     sync.Mutex
}

// NewConnectionManager creates a new connection manager from the given config.
func NewConnectionManager(cfg *config.Config) *ConnectionManager {
	return &ConnectionManager{
		config: cfg,
		conns:  make(map[string]imapClient),
	}
}

// GetClient returns an IMAP client for the named account,
// connecting lazily on first use. If a cached connection is
// dead, it is evicted and a fresh connection is established.
func (m *ConnectionManager) GetClient(
	accountName string,
) (imapClient, error) {
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
func (m *ConnectionManager) IsConnected(accountName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[accountName]
	if !ok {
		return false
	}
	return !isConnectionClosed(c)
}

// Config returns the manager's configuration.
func (m *ConnectionManager) Config() *config.Config {
	return m.config
}

// Close closes all open IMAP connections.
func (m *ConnectionManager) Close() error {
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

// isConnectionClosed performs a fast, non-blocking check on
// whether the client's connection has been closed.
func isConnectionClosed(c imapClient) bool {
	select {
	case <-c.Closed():
		return true
	default:
		return false
	}
}

// selectMailbox selects a mailbox in read-only or read-write
// mode and returns the mailbox metadata.
func selectMailbox(
	client imapClient,
	mailbox string,
	readOnly bool,
) (*imap.SelectData, error) {
	data, err := client.Select(
		mailbox,
		&imap.SelectOptions{ReadOnly: readOnly},
	).Wait()
	if err != nil {
		verb := "select"
		if readOnly {
			verb = "examine"
		}
		return nil, fmt.Errorf(
			"failed to %s mailbox: %w",
			verb,
			err,
		)
	}
	return data, nil
}

// connect dials and authenticates an IMAP connection for the
// given account.
func connect(acct config.Account) (imapClient, error) {
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

	return &clientAdapter{c}, nil
}
