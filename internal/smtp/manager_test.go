package smtpmanager

import (
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:     "imap.example.com",
				Port:     993,
				Username: "user",
				Password: "pass",
			},
		},
	}
	mgr := NewManager(cfg)
	if mgr.Config() != cfg {
		t.Error("Config() should return the provided config")
	}
}

func TestSend_UnknownAccount(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{},
	}
	mgr := NewManager(cfg)

	err := mgr.Send(
		"nonexistent",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
	if !strings.Contains(err.Error(), "unknown account") {
		t.Errorf(
			"error should mention unknown account, got: %v",
			err,
		)
	}
}

func TestSend_SMTPNotEnabled(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: false,
			},
		},
	}
	mgr := NewManager(cfg)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error when SMTP is not enabled")
	}
	if !strings.Contains(err.Error(), "not enabled") {
		t.Errorf(
			"error should mention not enabled, got: %v",
			err,
		)
	}
}

func TestSend_ConnectionFailure(t *testing.T) {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "nonexistent.invalid",
				SMTPPort:    587,
				SMTPTLS:     "starttls",
			},
		},
	}
	mgr := NewManager(cfg)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
	if !strings.Contains(
		err.Error(),
		"failed to connect",
	) {
		t.Errorf(
			"error should mention connection failure, got: %v",
			err,
		)
	}
}

func TestDial_InvalidTLSMode(t *testing.T) {
	acct := config.Account{
		SMTPHost: "smtp.example.com",
		SMTPPort: 587,
		SMTPTLS:  "bogus",
	}

	_, err := dial(acct)
	if err == nil {
		t.Fatal("expected error for invalid TLS mode")
	}
	if !strings.Contains(err.Error(), "invalid smtp_tls") {
		t.Errorf(
			"error should mention invalid tls mode, got: %v",
			err,
		)
	}
}
