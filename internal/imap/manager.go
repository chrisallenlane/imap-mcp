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

// ConnectionManager maintains persistent IMAP connections per account.
type ConnectionManager struct {
	config *config.Config
	conns  map[string]*imapclient.Client
	mu     sync.Mutex
}

// NewConnectionManager creates a new connection manager from the given config.
func NewConnectionManager(cfg *config.Config) *ConnectionManager {
	return &ConnectionManager{
		config: cfg,
		conns:  make(map[string]*imapclient.Client),
	}
}

// GetClient returns an IMAP client for the named account,
// connecting lazily on first use. If a cached connection is
// dead, it is evicted and a fresh connection is established.
func (m *ConnectionManager) GetClient(
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
func isConnectionClosed(c *imapclient.Client) bool {
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
	client *imapclient.Client,
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
