package imap

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func TestClose_NoConnections(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{},
	}
	mgr := NewManager(cfg)

	if err := mgr.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
}

func TestClose_WithConnections(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()

	// Inject the client into the manager's connection map.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	if err := mgr.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	// Verify the connection map was cleared.
	mgr.mu.Lock()
	remaining := len(mgr.conns)
	mgr.mu.Unlock()

	if remaining != 0 {
		t.Errorf(
			"connection map has %d entries after Close, "+
				"want 0",
			remaining,
		)
	}
}

func TestConfig_ReturnsCfg(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"gmail": {
				Host:     "imap.gmail.com",
				Port:     993,
				Username: "user@gmail.com",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	mgr := NewManager(cfg)

	if mgr.Config() != cfg {
		t.Error(
			"Config() did not return the config " +
				"passed to NewManager",
		)
	}
}

func TestGetClient_UnknownAccount(t *testing.T) {
	mgr := newTestManager()

	_, err := mgr.GetClient("doesnotexist")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestIsConnected_UnknownAccount(t *testing.T) {
	mgr := newTestManager()

	if mgr.IsConnected("nonexistent") {
		t.Error(
			"IsConnected() should return false " +
				"for unknown account",
		)
	}
}

func TestIsConnected_KnownButNotConnected(t *testing.T) {
	mgr := newTestManager()

	// Account exists in config but GetClient was never called,
	// so no connection should exist.
	if mgr.IsConnected("gmail") {
		t.Error(
			"IsConnected() should return false " +
				"for known but unconnected account",
		)
	}
}

// newTestManager creates a Manager with one configured account
// but no live connections. Useful for testing input validation.
func newTestManager() *Manager {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"gmail": {
				Host:     "imap.gmail.com",
				Port:     993,
				Username: "user@gmail.com",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	return NewManager(cfg)
}

func TestSelectMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.SelectMailbox("nonexistent", "INBOX")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestStoreFlags_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.StoreFlags(
		"nonexistent", "INBOX",
		[]imap.UID{1}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestStoreFlags_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.StoreFlags(
		"gmail", "INBOX",
		[]imap.UID{}, imap.StoreFlagsAdd,
		[]imap.Flag{imap.FlagSeen},
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestMoveMessages_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.MoveMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestMoveMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.MoveMessages(
		"gmail", "INBOX",
		[]imap.UID{}, "Trash",
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestCopyMessages_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.CopyMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestCopyMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.CopyMessages(
		"gmail", "INBOX",
		[]imap.UID{}, "Archive",
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestExpungeMessages_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.ExpungeMessages(
		"nonexistent", "INBOX",
		[]imap.UID{1},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestExpungeMessages_EmptyUIDs(t *testing.T) {
	mgr := newTestManager()
	err := mgr.ExpungeMessages(
		"gmail", "INBOX",
		[]imap.UID{},
	)
	if err == nil {
		t.Fatal("expected error for empty UIDs")
	}
}

func TestCreateMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.CreateMailbox("nonexistent", "NewFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestDeleteMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.DeleteMailbox("nonexistent", "OldFolder")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestFindTrashMailbox_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	_, err := mgr.FindTrashMailbox("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

// newTestClient creates an imapclient.Client backed by a
// net.Pipe. Closing serverConn causes the client's read
// goroutine to exit, which closes the Closed() channel.
// The caller must close serverConn to avoid goroutine leaks.
func newTestClient(
	t *testing.T,
) (*imapclient.Client, net.Conn) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	client := imapclient.New(clientConn, nil)
	return client, serverConn
}

func TestIsConnectionClosed_Open(t *testing.T) {
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	if isConnectionClosed(client) {
		t.Error(
			"isConnectionClosed() = true, " +
				"want false for open connection",
		)
	}
}

func TestIsConnectionClosed_Closed(t *testing.T) {
	client, serverConn := newTestClient(t)

	// Close the server end to kill the read goroutine.
	serverConn.Close()

	// Wait for the client to observe the closed connection.
	select {
	case <-client.Closed():
	case <-time.After(2 * time.Second):
		t.Fatal(
			"isConnectionClosed() did not " +
				"return true within timeout",
		)
	}

	if !isConnectionClosed(client) {
		t.Error(
			"isConnectionClosed() = false after " +
				"Closed() channel signaled",
		)
	}
}

func TestGetClient_EvictsDeadConnection(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)

	// Inject the client into the manager's connection map.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	// Kill the connection.
	serverConn.Close()

	// Wait for the client to observe the closed connection.
	select {
	case <-client.Closed():
	case <-time.After(2 * time.Second):
		t.Fatal("connection did not close within timeout")
	}

	// GetClient should detect the dead connection and
	// attempt reconnection. Since we have no real IMAP
	// server, reconnection will fail -- but the dead
	// client should be evicted from the map.
	_, err := mgr.GetClient("gmail")
	if err == nil {
		t.Fatal("expected error from reconnect attempt")
	}

	// Verify the dead client was evicted.
	mgr.mu.Lock()
	_, exists := mgr.conns["gmail"]
	mgr.mu.Unlock()

	if exists {
		t.Error("dead connection should have been evicted")
	}
}

func TestIsConnected_DeadConnection(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)

	// Inject the client into the manager's connection map.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	// Connection is alive -- should report true.
	if !mgr.IsConnected("gmail") {
		t.Error(
			"IsConnected() = false, " +
				"want true for live connection",
		)
	}

	// Kill the connection.
	serverConn.Close()

	// Wait for the client to observe the closed connection.
	select {
	case <-client.Closed():
	case <-time.After(2 * time.Second):
		t.Fatal("connection did not close within timeout")
	}

	// Should now report false for the dead connection.
	if mgr.IsConnected("gmail") {
		t.Error(
			"IsConnected() = true, " +
				"want false for dead connection",
		)
	}
}

func TestWithRetry_SucceedsFirstAttempt(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	// Inject a live client.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	calls := 0
	err := mgr.withRetry(
		"gmail",
		func(_ *imapclient.Client) error {
			calls++
			return nil
		},
	)
	if err != nil {
		t.Errorf("withRetry() unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1", calls)
	}
}

func TestWithRetry_ReturnsOpErrorWhenConnectionAlive(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	// Inject a live client.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	opErr := errors.New("operation failed")
	calls := 0
	err := mgr.withRetry(
		"gmail",
		func(_ *imapclient.Client) error {
			calls++
			return opErr
		},
	)

	if !errors.Is(err, opErr) {
		t.Errorf(
			"withRetry() error = %v, want %v",
			err,
			opErr,
		)
	}
	if calls != 1 {
		t.Errorf(
			"fn called %d times, want 1 (no retry for "+
				"non-connection error)",
			calls,
		)
	}
}

func TestWithRetry_RetriesOnDeadConnection(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)

	// Inject a live client.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	calls := 0
	err := mgr.withRetry(
		"gmail",
		func(c *imapclient.Client) error {
			calls++
			if calls == 1 {
				// Simulate connection death during
				// the operation.
				serverConn.Close()

				// Wait for the Closed() channel.
				select {
				case <-c.Closed():
				case <-time.After(2 * time.Second):
					t.Fatal(
						"connection did not " +
							"close within timeout",
					)
				}

				return errors.New("read: broken pipe")
			}
			// Second call -- reconnect attempt will have
			// failed (no real server), so we won't reach
			// here.
			return nil
		},
	)

	// Reconnect will fail because there's no real IMAP
	// server.
	if err == nil {
		t.Fatal("expected error from reconnect attempt")
	}
	if calls != 1 {
		t.Errorf(
			"fn called %d times, want 1 "+
				"(reconnect failed before retry)",
			calls,
		)
	}
}

func TestWithRetry_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	err := mgr.withRetry(
		"nonexistent",
		func(_ *imapclient.Client) error {
			t.Fatal("fn should not be called")
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestWithRetryResult_SucceedsFirstAttempt(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	// Inject a live client.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	calls := 0
	result, err := withRetryResult(
		mgr,
		"gmail",
		func(_ *imapclient.Client) (string, error) {
			calls++
			return "ok", nil
		},
	)
	if err != nil {
		t.Errorf(
			"withRetryResult() unexpected error: %v",
			err,
		)
	}
	if result != "ok" {
		t.Errorf(
			"withRetryResult() = %q, want %q",
			result,
			"ok",
		)
	}
	if calls != 1 {
		t.Errorf("fn called %d times, want 1", calls)
	}
}

func TestWithRetryResult_ReturnsOpErrorWhenAlive(t *testing.T) {
	mgr := newTestManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	// Inject a live client.
	mgr.mu.Lock()
	mgr.conns["gmail"] = client
	mgr.mu.Unlock()

	opErr := errors.New("operation failed")
	calls := 0
	result, err := withRetryResult(
		mgr,
		"gmail",
		func(_ *imapclient.Client) (string, error) {
			calls++
			return "", opErr
		},
	)

	if !errors.Is(err, opErr) {
		t.Errorf(
			"withRetryResult() error = %v, want %v",
			err,
			opErr,
		)
	}
	if result != "" {
		t.Errorf(
			"withRetryResult() = %q, want empty",
			result,
		)
	}
	if calls != 1 {
		t.Errorf(
			"fn called %d times, want 1 (no retry)",
			calls,
		)
	}
}

func TestWithRetryResult_UnknownAccount(t *testing.T) {
	mgr := newTestManager()
	_, err := withRetryResult(
		mgr,
		"nonexistent",
		func(_ *imapclient.Client) (string, error) {
			t.Fatal("fn should not be called")
			return "", nil
		},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}
