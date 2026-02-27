package imap

import (
	"fmt"
	"log"

	"github.com/emersion/go-imap/v2/imapclient"
)

// withRetryResult executes fn with an IMAP client. If fn
// returns an error and the connection appears dead, it
// reconnects once and retries.
func withRetryResult[T any](
	m *ConnectionManager,
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

// withRetry is like withRetryResult for operations that
// return only an error.
func (m *ConnectionManager) withRetry(
	account string,
	fn func(c *imapclient.Client) error,
) error {
	_, err := withRetryResult(
		m,
		account,
		func(c *imapclient.Client) (struct{}, error) {
			return struct{}{}, fn(c)
		},
	)
	return err
}
