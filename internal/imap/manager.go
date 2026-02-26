// Package imap manages persistent IMAP client connections.
package imap

import (
	"fmt"
	"sync"

	"github.com/chrisallenlane/imap-mcp/internal/config"
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

// Config returns the manager's configuration.
func (m *Manager) Config() *config.Config {
	return m.config
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
