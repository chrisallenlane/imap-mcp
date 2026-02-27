package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

type mockAccountLister struct {
	config    *config.Config
	connected map[string]bool
}

func (m *mockAccountLister) Config() *config.Config {
	return m.config
}

func (m *mockAccountLister) IsConnected(
	accountName string,
) bool {
	return m.connected[accountName]
}

func TestListAccounts_InputSchema(t *testing.T) {
	assertSchema(
		t,
		NewListAccounts(&mockAccountLister{
			config: &config.Config{},
		}).InputSchema(),
		nil,
		nil,
	)
}

func TestListAccounts_NoAccounts(t *testing.T) {
	tool := NewListAccounts(&mockAccountLister{
		config: &config.Config{
			Accounts: map[string]config.Account{},
		},
	})

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	if result != "No accounts configured." {
		t.Errorf(
			"result = %q, want %q",
			result,
			"No accounts configured.",
		)
	}
}

func TestListAccounts_SingleAccount(t *testing.T) {
	tool := NewListAccounts(&mockAccountLister{
		config: &config.Config{
			Accounts: map[string]config.Account{
				"gmail": {
					Host:     "imap.gmail.com",
					Port:     993,
					Username: "user@gmail.com",
					Password: "pass",
					TLS:      true,
				},
			},
		},
	})

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Configured accounts:")
	assertContains(t, result, "1. gmail")
	assertContains(t, result, "Host: imap.gmail.com:993 (TLS)")
	assertContains(t, result, "User: user@gmail.com")
	assertContains(t, result, "Status: not connected")
}

func TestListAccounts_MultipleAccounts_Sorted(t *testing.T) {
	tool := NewListAccounts(&mockAccountLister{
		config: &config.Config{
			Accounts: map[string]config.Account{
				"protonmail": {
					Host:     "127.0.0.1",
					Port:     1143,
					Username: "user@proton.me",
					Password: "pass",
					TLS:      false,
				},
				"gmail": {
					Host:     "imap.gmail.com",
					Port:     993,
					Username: "user@gmail.com",
					Password: "pass",
					TLS:      true,
				},
			},
		},
	})

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// gmail should come before protonmail (sorted)
	assertContains(t, result, "1. gmail")
	assertContains(t, result, "2. protonmail")

	gmailIdx := strings.Index(result, "1. gmail")
	protonIdx := strings.Index(result, "2. protonmail")
	if gmailIdx >= protonIdx {
		t.Error(
			"gmail should appear before protonmail " +
				"(alphabetical sort)",
		)
	}
}

func TestListAccounts_NoTLS(t *testing.T) {
	tool := NewListAccounts(&mockAccountLister{
		config: &config.Config{
			Accounts: map[string]config.Account{
				"local": {
					Host:     "127.0.0.1",
					Port:     143,
					Username: "user",
					Password: "pass",
					TLS:      false,
				},
			},
		},
	})

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Host: 127.0.0.1:143")
	// Should NOT contain (TLS) marker
	if strings.Contains(result, "(TLS)") {
		t.Error(
			"result should not contain (TLS) " +
				"for non-TLS account",
		)
	}
}

func TestListAccounts_NilArgs(t *testing.T) {
	tool := NewListAccounts(&mockAccountLister{
		config: &config.Config{
			Accounts: map[string]config.Account{
				"test": {
					Host:     "localhost",
					Port:     993,
					Username: "user",
					Password: "pass",
					TLS:      true,
				},
			},
		},
	})

	// Execute with nil args should still work
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Configured accounts:")
}

func TestListAccounts_Connected(t *testing.T) {
	tool := NewListAccounts(&mockAccountLister{
		config: &config.Config{
			Accounts: map[string]config.Account{
				"gmail": {
					Host:     "imap.gmail.com",
					Port:     993,
					Username: "user@gmail.com",
					Password: "pass",
					TLS:      true,
				},
			},
		},
		connected: map[string]bool{"gmail": true},
	})

	result, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Status: connected")
}
