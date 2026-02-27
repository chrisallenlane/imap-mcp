// Package smtpmanager manages SMTP connections for sending
// email.
package smtpmanager

import (
	"crypto/tls"
	"fmt"
	"io"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

// smtpClient is a narrow interface for SMTP client operations.
// *gosmtp.Client satisfies this via clientAdapter.
type smtpClient interface {
	Auth(a sasl.Client) error
	Mail(from string, opts *gosmtp.MailOptions) error
	Rcpt(to string, opts *gosmtp.RcptOptions) error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
}

// clientAdapter wraps *gosmtp.Client to satisfy smtpClient.
// The only adaptation is promoting Data()'s return type from
// *gosmtp.DataCommand to io.WriteCloser.
type clientAdapter struct {
	*gosmtp.Client
}

func (a *clientAdapter) Data() (io.WriteCloser, error) {
	return a.Client.Data()
}

// Manager handles SMTP sending for configured accounts.
type Manager struct {
	config *config.Config
	dial   func(config.Account) (smtpClient, error)
}

// NewManager creates a new SMTP manager from the given config.
func NewManager(cfg *config.Config) *Manager {
	return &Manager{config: cfg, dial: defaultDial}
}

// defaultDial creates an SMTP connection and wraps it as an
// smtpClient.
func defaultDial(acct config.Account) (smtpClient, error) {
	c, err := dial(acct)
	if err != nil {
		return nil, err
	}
	return &clientAdapter{c}, nil
}

// Config returns the manager's configuration.
func (m *Manager) Config() *config.Config {
	return m.config
}

// Send sends an email via SMTP for the named account.
// The from address specifies the envelope sender, to lists
// all envelope recipients, and msg is the RFC 5322 message.
func (m *Manager) Send(
	account, from string,
	to []string,
	msg io.Reader,
) error {
	acct, ok := m.config.Accounts[account]
	if !ok {
		return fmt.Errorf("unknown account: %q", account)
	}

	if !acct.SMTPEnabled {
		return fmt.Errorf(
			"SMTP is not enabled for account %q. "+
				"Set smtp_enabled = true in your "+
				"config file.",
			account,
		)
	}

	c, err := m.dial(acct)
	if err != nil {
		return fmt.Errorf(
			"failed to connect to SMTP server: %w",
			err,
		)
	}
	defer c.Close()

	auth := sasl.NewPlainClient(
		"",
		acct.Username,
		acct.Password,
	)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	if err := c.Mail(from, nil); err != nil {
		return fmt.Errorf(
			"failed to set envelope sender: %w",
			err,
		)
	}

	for _, rcpt := range to {
		if err := c.Rcpt(rcpt, nil); err != nil {
			return fmt.Errorf(
				"failed to add recipient %q: %w",
				rcpt,
				err,
			)
		}
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf(
			"failed to start DATA command: %w",
			err,
		)
	}

	if _, err := io.Copy(wc, msg); err != nil {
		return fmt.Errorf(
			"failed to write message data: %w",
			err,
		)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf(
			"failed to finalize message: %w",
			err,
		)
	}

	if err := c.Quit(); err != nil {
		return fmt.Errorf("SMTP QUIT failed: %w", err)
	}

	return nil
}

// dial establishes an SMTP connection using the account's
// TLS configuration.
func dial(acct config.Account) (*gosmtp.Client, error) {
	addr := fmt.Sprintf(
		"%s:%d",
		acct.SMTPHost,
		acct.SMTPPort,
	)

	tlsMode := acct.SMTPTLS
	if tlsMode == "" {
		tlsMode = "starttls"
	}

	tlsCfg := &tls.Config{
		ServerName: acct.SMTPHost,
	}

	switch tlsMode {
	case "starttls":
		return gosmtp.DialStartTLS(addr, tlsCfg)
	case "implicit":
		return gosmtp.DialTLS(addr, tlsCfg)
	case "none":
		return gosmtp.Dial(addr)
	default:
		return nil, fmt.Errorf(
			"invalid smtp_tls mode: %q",
			tlsMode,
		)
	}
}
