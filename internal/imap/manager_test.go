package imap

import (
	"net"
	"testing"
	"time"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	"github.com/emersion/go-imap/v2/imapclient"
)

// newTestConnectionManager creates a ConnectionManager with one configured account
// but no live connections. Useful for testing input validation.
func newTestConnectionManager() *ConnectionManager {
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
	return NewConnectionManager(cfg)
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

func TestClose_NoConnections(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{},
	}
	mgr := NewConnectionManager(cfg)

	if err := mgr.Close(); err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}
}

func TestClose_WithConnections(t *testing.T) {
	mgr := newTestConnectionManager()
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
	mgr := NewConnectionManager(cfg)

	if mgr.Config() != cfg {
		t.Error(
			"Config() did not return the config " +
				"passed to NewConnectionManager",
		)
	}
}

func TestGetClient_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()

	_, err := mgr.GetClient("doesnotexist")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestIsConnected_UnknownAccount(t *testing.T) {
	mgr := newTestConnectionManager()

	if mgr.IsConnected("nonexistent") {
		t.Error(
			"IsConnected() should return false " +
				"for unknown account",
		)
	}
}

func TestIsConnected_KnownButNotConnected(t *testing.T) {
	mgr := newTestConnectionManager()

	// Account exists in config but GetClient was never called,
	// so no connection should exist.
	if mgr.IsConnected("gmail") {
		t.Error(
			"IsConnected() should return false " +
				"for known but unconnected account",
		)
	}
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
	mgr := newTestConnectionManager()
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
	mgr := newTestConnectionManager()
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
