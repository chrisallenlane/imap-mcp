package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_Valid(t *testing.T) {
	content := `
[accounts.gmail]
host     = "imap.gmail.com"
port     = 993
username = "user@gmail.com"
password = "app-password"
tls      = true

[accounts.protonmail]
host     = "127.0.0.1"
port     = 1143
username = "user@proton.me"
password = "bridge-password"
tls      = false
`
	path := writeTempConfig(t, content)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	if len(cfg.Accounts) != 2 {
		t.Fatalf(
			"expected 2 accounts, got %d",
			len(cfg.Accounts),
		)
	}

	gmail := cfg.Accounts["gmail"]
	if gmail.Host != "imap.gmail.com" {
		t.Errorf("gmail host = %q, want %q", gmail.Host, "imap.gmail.com")
	}
	if gmail.Port != 993 {
		t.Errorf("gmail port = %d, want %d", gmail.Port, 993)
	}
	if gmail.Username != "user@gmail.com" {
		t.Errorf(
			"gmail username = %q, want %q",
			gmail.Username,
			"user@gmail.com",
		)
	}
	if !gmail.TLS {
		t.Error("gmail tls should be true")
	}

	pm := cfg.Accounts["protonmail"]
	if pm.TLS {
		t.Error("protonmail tls should be false")
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent file")
	}
}

func TestLoad_MalformedTOML(t *testing.T) {
	path := writeTempConfig(t, `[this is not valid toml`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for malformed TOML")
	}
}

func TestLoad_NoAccounts(t *testing.T) {
	path := writeTempConfig(t, `# empty config`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected error for empty accounts")
	}
}

func TestValidate_MissingHost(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Port:     993,
				Username: "user",
				Password: "pass",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for missing host")
	}
}

func TestValidate_MissingPort(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:     "imap.example.com",
				Username: "user",
				Password: "pass",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for missing port")
	}
}

func TestValidate_MissingUsername(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Password: "pass",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for missing username")
	}
}

func TestValidate_MissingPassword(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for missing password")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_MultipleAccounts(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"acct1": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user1",
				Password: "pass1",
			},
			"acct2": {
				Host:     "imap.other.com",
				Port:     143,
				Username: "user2",
				Password: "pass2",
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_ErrorMessages(t *testing.T) {
	tests := []struct {
		name    string
		account Account
		wantMsg string
	}{
		{
			name: "missing host",
			account: Account{
				Port:     993,
				Username: "user",
				Password: "pass",
			},
			wantMsg: "host is required",
		},
		{
			name: "missing port",
			account: Account{
				Host:     "imap.example.com",
				Username: "user",
				Password: "pass",
			},
			wantMsg: "port is required",
		},
		{
			name: "missing username",
			account: Account{
				Host:     "imap.example.com",
				Port:     993,
				Password: "pass",
			},
			wantMsg: "username is required",
		},
		{
			name: "missing password",
			account: Account{
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
			},
			wantMsg: "password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Accounts: map[string]Account{
					"test": tt.account,
				},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() expected error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf(
					"error = %q, want to contain %q",
					err.Error(),
					tt.wantMsg,
				)
			}
			// Verify account name appears in error
			if !strings.Contains(err.Error(), "test") {
				t.Errorf(
					"error = %q, should mention account name",
					err.Error(),
				)
			}
		})
	}
}

func TestValidate_EmptyAccounts(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for empty accounts")
	}
	if !strings.Contains(
		err.Error(),
		"at least one account",
	) {
		t.Errorf(
			"error = %q, want to mention 'at least one account'",
			err.Error(),
		)
	}
}

func TestLoad_TLSDefault(t *testing.T) {
	content := `
[accounts.test]
host     = "imap.example.com"
port     = 993
username = "user"
password = "pass"
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	acct := cfg.Accounts["test"]
	if acct.TLS {
		t.Error("TLS should default to false when omitted")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
