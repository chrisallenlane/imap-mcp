package imap

import (
	"errors"
	"testing"
	"time"
)

func TestWithRetry_SucceedsFirstAttempt(t *testing.T) {
	mgr := newTestConnectionManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	injectClient(mgr, "gmail", client)

	calls := 0
	err := mgr.withRetry(
		"gmail",
		func(_ imapClient) error {
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
	mgr := newTestConnectionManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	injectClient(mgr, "gmail", client)

	opErr := errors.New("operation failed")
	calls := 0
	err := mgr.withRetry(
		"gmail",
		func(_ imapClient) error {
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
	mgr := newTestConnectionManager()
	client, serverConn := newTestClient(t)

	injectClient(mgr, "gmail", client)

	calls := 0
	err := mgr.withRetry(
		"gmail",
		func(c imapClient) error {
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
	mgr := newTestConnectionManager()
	err := mgr.withRetry(
		"nonexistent",
		func(_ imapClient) error {
			t.Fatal("fn should not be called")
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestWithRetryResult_SucceedsFirstAttempt(t *testing.T) {
	mgr := newTestConnectionManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	injectClient(mgr, "gmail", client)

	calls := 0
	result, err := withRetryResult(
		mgr,
		"gmail",
		func(_ imapClient) (string, error) {
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
	mgr := newTestConnectionManager()
	client, serverConn := newTestClient(t)
	defer serverConn.Close()
	defer client.Close()

	injectClient(mgr, "gmail", client)

	opErr := errors.New("operation failed")
	calls := 0
	result, err := withRetryResult(
		mgr,
		"gmail",
		func(_ imapClient) (string, error) {
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
	mgr := newTestConnectionManager()
	_, err := withRetryResult(
		mgr,
		"nonexistent",
		func(_ imapClient) (string, error) {
			t.Fatal("fn should not be called")
			return "", nil
		},
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}
