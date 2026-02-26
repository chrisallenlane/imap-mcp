package imap

import (
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
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

func TestGetClient_UnknownAccountErrorMessage(t *testing.T) {
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

	_, err := mgr.GetClient("doesnotexist")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}

	if !strings.Contains(
		err.Error(),
		"unknown account",
	) {
		t.Errorf(
			"error = %q, should mention 'unknown account'",
			err.Error(),
		)
	}

	if !strings.Contains(err.Error(), "doesnotexist") {
		t.Errorf(
			"error = %q, should mention account name",
			err.Error(),
		)
	}
}

func TestIsConnected_UnknownAccount(t *testing.T) {
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

	if mgr.IsConnected("nonexistent") {
		t.Error(
			"IsConnected() should return false " +
				"for unknown account",
		)
	}
}

func TestIsConnected_KnownButNotConnected(t *testing.T) {
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

	// Account exists in config but GetClient was never called,
	// so no connection should exist.
	if mgr.IsConnected("gmail") {
		t.Error(
			"IsConnected() should return false " +
				"for known but unconnected account",
		)
	}
}
