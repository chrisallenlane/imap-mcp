// Package config handles TOML configuration parsing for imap-mcp.
package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Config represents the top-level configuration.
type Config struct {
	Accounts map[string]Account `toml:"accounts"`
}

// Account represents a single IMAP account configuration.
type Account struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
	TLS      bool   `toml:"tls"`

	// SMTP fields (only validated when SMTPEnabled is true).
	SMTPEnabled bool   `toml:"smtp_enabled"`
	SMTPHost    string `toml:"smtp_host"`
	SMTPPort    int    `toml:"smtp_port"`
	SMTPTLS     string `toml:"smtp_tls"`
	SMTPFrom    string `toml:"smtp_from"`
	SaveSent    bool   `toml:"save_sent"`
}

// validSMTPTLS is the set of allowed values for the smtp_tls field.
var validSMTPTLS = map[string]bool{
	"":         true,
	"starttls": true,
	"implicit": true,
	"none":     true,
}

// Load reads and parses a TOML config file from the given path.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks that the config contains all required fields.
func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return fmt.Errorf(
			"config must define at least one account",
		)
	}

	for name, acct := range c.Accounts {
		if acct.Host == "" {
			return fmt.Errorf(
				"account %q: host is required",
				name,
			)
		}
		if acct.Port == 0 {
			return fmt.Errorf(
				"account %q: port is required",
				name,
			)
		}
		if acct.Username == "" {
			return fmt.Errorf(
				"account %q: username is required",
				name,
			)
		}
		if acct.Password == "" {
			return fmt.Errorf(
				"account %q: password is required",
				name,
			)
		}

		if acct.SMTPEnabled {
			if acct.SMTPHost == "" {
				return fmt.Errorf(
					"account %q: smtp_host is required when smtp_enabled is true",
					name,
				)
			}
			if acct.SMTPPort == 0 {
				return fmt.Errorf(
					"account %q: smtp_port is required when smtp_enabled is true",
					name,
				)
			}
			if !validSMTPTLS[acct.SMTPTLS] {
				return fmt.Errorf(
					"account %q: smtp_tls must be \"starttls\", \"implicit\", or \"none\"",
					name,
				)
			}
		}
	}

	return nil
}

// HasSMTPEnabled reports whether at least one account has
// smtp_enabled set to true.
func (c *Config) HasSMTPEnabled() bool {
	for _, acct := range c.Accounts {
		if acct.SMTPEnabled {
			return true
		}
	}
	return false
}
