package imap

import (
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

func TestGetClient_UnknownAccount(t *testing.T) {
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

	_, err := mgr.GetClient("nonexistent")
	if err == nil {
		t.Fatal("GetClient() expected error for unknown account")
	}
}

func TestConfig(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:     "localhost",
				Port:     993,
				Username: "user",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	mgr := NewManager(cfg)

	if mgr.Config() != cfg {
		t.Error("Config() should return the same config")
	}
}

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

func TestNewManager_StoresConfig(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"a": {
				Host:     "host-a",
				Port:     993,
				Username: "user-a",
				Password: "pass-a",
			},
			"b": {
				Host:     "host-b",
				Port:     143,
				Username: "user-b",
				Password: "pass-b",
			},
		},
	}
	mgr := NewManager(cfg)

	got := mgr.Config()
	if len(got.Accounts) != 2 {
		t.Fatalf(
			"expected 2 accounts, got %d",
			len(got.Accounts),
		)
	}

	if got.Accounts["a"].Host != "host-a" {
		t.Errorf(
			"account a host = %q, want host-a",
			got.Accounts["a"].Host,
		)
	}

	if got.Accounts["b"].Port != 143 {
		t.Errorf(
			"account b port = %d, want 143",
			got.Accounts["b"].Port,
		)
	}
}

func TestClose_Idempotent(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{},
	}
	mgr := NewManager(cfg)

	// Closing twice should not error
	if err := mgr.Close(); err != nil {
		t.Errorf("first Close() unexpected error: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Errorf("second Close() unexpected error: %v", err)
	}
}
