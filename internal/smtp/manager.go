// Package smtpmanager manages SMTP connections for sending
// email.
package smtpmanager

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

// Sentinel errors returned by Send and dial. Callers may use
// errors.Is to identify specific failure modes.
var (
	// ErrUnknownAccount is returned when the named account is
	// not present in the config.
	ErrUnknownAccount = errors.New("unknown account")

	// ErrSMTPDisabled is returned when smtp_enabled is false
	// for the named account.
	ErrSMTPDisabled = errors.New("SMTP not enabled for account")

	// ErrNoRecipients is returned when Send is called with an
	// empty to slice.
	ErrNoRecipients = errors.New("no recipients specified")

	// ErrDialFailed is returned when the SMTP connection cannot
	// be established.
	ErrDialFailed = errors.New("failed to connect to SMTP server")

	// ErrAuthFailed is returned when SMTP AUTH is rejected.
	ErrAuthFailed = errors.New("SMTP authentication failed")

	// ErrMailFailed is returned when the MAIL FROM command
	// is rejected.
	ErrMailFailed = errors.New("failed to set envelope sender")

	// ErrRcptFailed is returned when a RCPT TO command is
	// rejected.
	ErrRcptFailed = errors.New("failed to add recipient")

	// ErrDataFailed is returned when the DATA command is
	// rejected.
	ErrDataFailed = errors.New("failed to start DATA command")

	// ErrWriteFailed is returned when writing the message body
	// fails.
	ErrWriteFailed = errors.New("failed to write message data")

	// ErrFinalizeFailed is returned when closing the DATA
	// writer fails.
	ErrFinalizeFailed = errors.New("failed to finalize message")

	// ErrQuitFailed is returned when the QUIT command fails.
	ErrQuitFailed = errors.New("SMTP QUIT failed")

	// ErrInvalidTLSMode is returned by dial when smtp_tls
	// contains an unrecognised value.
	ErrInvalidTLSMode = errors.New("invalid smtp_tls mode")
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
		return fmt.Errorf("%w: %q", ErrUnknownAccount, account)
	}

	if !acct.SMTPEnabled {
		return fmt.Errorf(
			"%w %q: set smtp_enabled = true in config",
			ErrSMTPDisabled,
			account,
		)
	}

	if len(to) == 0 {
		return ErrNoRecipients
	}

	c, err := m.dial(acct)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDialFailed, err)
	}
	defer c.Close()

	auth := sasl.NewPlainClient(
		"",
		acct.Username,
		acct.Password,
	)
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("%w: %w", ErrAuthFailed, err)
	}

	if err := c.Mail(from, nil); err != nil {
		return fmt.Errorf("%w: %w", ErrMailFailed, err)
	}

	for _, rcpt := range to {
		if err := c.Rcpt(rcpt, nil); err != nil {
			return fmt.Errorf(
				"%w %q: %w",
				ErrRcptFailed,
				rcpt,
				err,
			)
		}
	}

	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDataFailed, err)
	}

	if _, err := io.Copy(wc, msg); err != nil {
		return fmt.Errorf("%w: %w", ErrWriteFailed, err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("%w: %w", ErrFinalizeFailed, err)
	}

	if err := c.Quit(); err != nil {
		return fmt.Errorf("%w: %w", ErrQuitFailed, err)
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
		return nil, fmt.Errorf("%w: %q", ErrInvalidTLSMode, tlsMode)
	}
}
