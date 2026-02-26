package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	imapmanager "github.com/chrisallenlane/imap-mcp/internal/imap"
)

func TestListAccounts_Description(t *testing.T) {
	mgr := imapmanager.NewManager(&config.Config{})
	tool := NewListAccounts(mgr)

	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestListAccounts_InputSchema(t *testing.T) {
	mgr := imapmanager.NewManager(&config.Config{})
	tool := NewListAccounts(mgr)

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Errorf(
			"schema type = %v, want object",
			schema["type"],
		)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}
	if len(props) != 0 {
		t.Errorf(
			"expected 0 properties, got %d",
			len(props),
		)
	}
}

func TestListAccounts_NoAccounts(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{},
	}
	mgr := imapmanager.NewManager(cfg)
	tool := NewListAccounts(mgr)

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
	mgr := imapmanager.NewManager(cfg)
	tool := NewListAccounts(mgr)

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
	cfg := &config.Config{
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
	}
	mgr := imapmanager.NewManager(cfg)
	tool := NewListAccounts(mgr)

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
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"local": {
				Host:     "127.0.0.1",
				Port:     143,
				Username: "user",
				Password: "pass",
				TLS:      false,
			},
		},
	}
	mgr := imapmanager.NewManager(cfg)
	tool := NewListAccounts(mgr)

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
	mgr := imapmanager.NewManager(cfg)
	tool := NewListAccounts(mgr)

	// Execute with nil args should still work
	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	assertContains(t, result, "Configured accounts:")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf(
			"result does not contain %q\ngot:\n%s",
			substr,
			s,
		)
	}
}
