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
	}

	return nil
}
