package config

import (
	"os"
	"path/filepath"
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
	}{
		{
			name: "missing host",
			account: Account{
				Port:     993,
				Username: "user",
				Password: "pass",
			},
		},
		{
			name: "missing port",
			account: Account{
				Host:     "imap.example.com",
				Username: "user",
				Password: "pass",
			},
		},
		{
			name: "missing username",
			account: Account{
				Host:     "imap.example.com",
				Port:     993,
				Password: "pass",
			},
		},
		{
			name: "missing password",
			account: Account{
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
			},
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
}

func TestValidate_SMTPEnabled_Valid(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_SMTPEnabled_WithTLSMode(t *testing.T) {
	for _, mode := range []string{"starttls", "implicit", "none", ""} {
		t.Run("mode_"+mode, func(t *testing.T) {
			cfg := &Config{
				Accounts: map[string]Account{
					"test": {
						Host:        "imap.example.com",
						Port:        993,
						Username:    "user",
						Password:    "pass",
						SMTPEnabled: true,
						SMTPHost:    "smtp.example.com",
						SMTPPort:    587,
						SMTPTLS:     mode,
					},
				},
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf(
					"Validate() unexpected error for mode %q: %v",
					mode,
					err,
				)
			}
		})
	}
}

func TestValidate_SMTPEnabled_MissingHost(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPPort:    587,
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing smtp_host")
	}
}

func TestValidate_SMTPEnabled_MissingPort(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for missing smtp_port")
	}
}

func TestValidate_SMTPEnabled_InvalidTLSMode(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
				SMTPTLS:     "invalid",
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for invalid smtp_tls")
	}
}

func TestValidate_SMTPDisabled_SkipsValidation(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
				Password: "pass",
				// SMTPEnabled defaults to false; SMTP fields missing
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestValidate_SMTPDisabled_WithPartialFields(t *testing.T) {
	cfg := &Config{
		Accounts: map[string]Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
				Password: "pass",
				SMTPHost: "smtp.example.com",
				// SMTPEnabled false, SMTPPort missing — should still pass
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestHasSMTPEnabled(t *testing.T) {
	tests := []struct {
		name     string
		accounts map[string]Account
		want     bool
	}{
		{
			name: "no accounts have smtp",
			accounts: map[string]Account{
				"a": {Host: "h", Port: 1, Username: "u", Password: "p"},
			},
			want: false,
		},
		{
			name: "one account has smtp",
			accounts: map[string]Account{
				"a": {Host: "h", Port: 1, Username: "u", Password: "p"},
				"b": {
					Host: "h", Port: 1, Username: "u", Password: "p",
					SMTPEnabled: true, SMTPHost: "s", SMTPPort: 587,
				},
			},
			want: true,
		},
		{
			name: "all accounts have smtp",
			accounts: map[string]Account{
				"a": {
					Host: "h", Port: 1, Username: "u", Password: "p",
					SMTPEnabled: true, SMTPHost: "s", SMTPPort: 587,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Accounts: tt.accounts}
			if got := cfg.HasSMTPEnabled(); got != tt.want {
				t.Errorf("HasSMTPEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_SMTPConfig(t *testing.T) {
	content := `
[accounts.test]
host         = "imap.example.com"
port         = 993
username     = "user"
password     = "pass"
tls          = true
smtp_enabled = true
smtp_host    = "smtp.example.com"
smtp_port    = 587
smtp_tls     = "starttls"
smtp_from    = "sender@example.com"
save_sent    = true
`
	path := writeTempConfig(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}

	acct := cfg.Accounts["test"]
	if !acct.SMTPEnabled {
		t.Error("SMTPEnabled should be true")
	}
	if acct.SMTPHost != "smtp.example.com" {
		t.Errorf("SMTPHost = %q, want %q", acct.SMTPHost, "smtp.example.com")
	}
	if acct.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want %d", acct.SMTPPort, 587)
	}
	if acct.SMTPTLS != "starttls" {
		t.Errorf("SMTPTLS = %q, want %q", acct.SMTPTLS, "starttls")
	}
	if acct.SMTPFrom != "sender@example.com" {
		t.Errorf("SMTPFrom = %q, want %q", acct.SMTPFrom, "sender@example.com")
	}
	if !acct.SaveSent {
		t.Error("SaveSent should be true")
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
