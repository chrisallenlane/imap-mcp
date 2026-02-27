package smtpmanager

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

// mockDataWriter is a controllable io.WriteCloser for testing
// the DATA phase.
type mockDataWriter struct {
	buf      bytes.Buffer
	writeErr error
	closeErr error
}

func (w *mockDataWriter) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return w.buf.Write(p)
}

func (w *mockDataWriter) Close() error {
	return w.closeErr
}

// mockClient implements smtpClient for testing.
type mockClient struct {
	authErr  error
	mailErr  error
	rcptErr  error
	dataErr  error
	quitErr  error
	closeErr error

	dataWriter *mockDataWriter

	authCalls  int
	mailFrom   string
	rcptAddrs  []string
	dataCalls  int
	quitCalls  int
	closeCalls int
}

func (c *mockClient) Auth(_ sasl.Client) error {
	c.authCalls++
	return c.authErr
}

func (c *mockClient) Mail(
	from string, _ *gosmtp.MailOptions,
) error {
	c.mailFrom = from
	return c.mailErr
}

func (c *mockClient) Rcpt(
	to string, _ *gosmtp.RcptOptions,
) error {
	c.rcptAddrs = append(c.rcptAddrs, to)
	return c.rcptErr
}

func (c *mockClient) Data() (io.WriteCloser, error) {
	c.dataCalls++
	if c.dataErr != nil {
		return nil, c.dataErr
	}
	return c.dataWriter, nil
}

func (c *mockClient) Quit() error {
	c.quitCalls++
	return c.quitErr
}

func (c *mockClient) Close() error {
	c.closeCalls++
	return c.closeErr
}

// smtpEnabledConfig returns a config with one SMTP-enabled
// account.
func smtpEnabledConfig() *config.Config {
	return &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:        "imap.example.com",
				Port:        993,
				Username:    "user",
				Password:    "pass",
				SMTPEnabled: true,
				SMTPHost:    "smtp.example.com",
				SMTPPort:    587,
				SMTPTLS:     "starttls",
			},
		},
	}
}

// newMockManager creates a Manager with an injectable mock
// client.
func newMockManager(
	cfg *config.Config,
	mc *mockClient,
) *Manager {
	return &Manager{
		config: cfg,
		dial: func(_ config.Account) (smtpClient, error) {
			return mc, nil
		},
	}
}

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
	if mgr.dial == nil {
		t.Error("dial should be set to defaultDial")
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

func TestSend_DialFailure(t *testing.T) {
	cfg := smtpEnabledConfig()
	mgr := &Manager{
		config: cfg,
		dial: func(_ config.Account) (smtpClient, error) {
			return nil, errors.New("connection refused")
		},
	}

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for dial failure")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Errorf(
			"error should mention connection failure, got: %v",
			err,
		)
	}
}

func TestSend_AuthFailure(t *testing.T) {
	mc := &mockClient{
		authErr:    errors.New("invalid credentials"),
		dataWriter: &mockDataWriter{},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for auth failure")
	}
	if !strings.Contains(
		err.Error(),
		"SMTP authentication failed",
	) {
		t.Errorf("unexpected error: %v", err)
	}
	if mc.authCalls != 1 {
		t.Errorf("Auth called %d times, want 1", mc.authCalls)
	}
	if mc.closeCalls != 1 {
		t.Errorf(
			"Close called %d times, want 1 (defer)",
			mc.closeCalls,
		)
	}
}

func TestSend_MailFailure(t *testing.T) {
	mc := &mockClient{
		mailErr:    errors.New("sender rejected"),
		dataWriter: &mockDataWriter{},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for mail failure")
	}
	if !strings.Contains(
		err.Error(),
		"failed to set envelope sender",
	) {
		t.Errorf("unexpected error: %v", err)
	}
	if mc.mailFrom != "from@example.com" {
		t.Errorf(
			"Mail called with %q, want %q",
			mc.mailFrom,
			"from@example.com",
		)
	}
}

func TestSend_RcptFailure(t *testing.T) {
	mc := &mockClient{
		rcptErr:    errors.New("recipient rejected"),
		dataWriter: &mockDataWriter{},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"bad@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for rcpt failure")
	}
	if !strings.Contains(
		err.Error(),
		"failed to add recipient",
	) {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "bad@example.com") {
		t.Errorf(
			"error should mention failing address, got: %v",
			err,
		)
	}
}

func TestSend_DataStartFailure(t *testing.T) {
	mc := &mockClient{
		dataErr:    errors.New("data rejected"),
		dataWriter: &mockDataWriter{},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test"),
	)
	if err == nil {
		t.Fatal("expected error for data start failure")
	}
	if !strings.Contains(
		err.Error(),
		"failed to start DATA command",
	) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSend_DataWriteFailure(t *testing.T) {
	mc := &mockClient{
		dataWriter: &mockDataWriter{
			writeErr: errors.New("write failed"),
		},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test body"),
	)
	if err == nil {
		t.Fatal("expected error for data write failure")
	}
	if !strings.Contains(
		err.Error(),
		"failed to write message data",
	) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSend_DataCloseFailure(t *testing.T) {
	mc := &mockClient{
		dataWriter: &mockDataWriter{
			closeErr: errors.New("finalize failed"),
		},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test body"),
	)
	if err == nil {
		t.Fatal("expected error for data close failure")
	}
	if !strings.Contains(
		err.Error(),
		"failed to finalize message",
	) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSend_QuitFailure(t *testing.T) {
	mc := &mockClient{
		quitErr:    errors.New("quit failed"),
		dataWriter: &mockDataWriter{},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com"},
		strings.NewReader("test body"),
	)
	if err == nil {
		t.Fatal("expected error for quit failure")
	}
	if !strings.Contains(err.Error(), "SMTP QUIT failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSend_HappyPath(t *testing.T) {
	mc := &mockClient{
		dataWriter: &mockDataWriter{},
	}
	mgr := newMockManager(smtpEnabledConfig(), mc)

	body := "Subject: Test\r\n\r\nHello"
	err := mgr.Send(
		"test",
		"from@example.com",
		[]string{"to@example.com", "cc@example.com"},
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mc.authCalls != 1 {
		t.Errorf("Auth called %d times, want 1", mc.authCalls)
	}
	if mc.mailFrom != "from@example.com" {
		t.Errorf(
			"Mail from = %q, want %q",
			mc.mailFrom,
			"from@example.com",
		)
	}
	if len(mc.rcptAddrs) != 2 {
		t.Fatalf(
			"Rcpt called %d times, want 2",
			len(mc.rcptAddrs),
		)
	}
	if mc.rcptAddrs[0] != "to@example.com" {
		t.Errorf(
			"Rcpt[0] = %q, want %q",
			mc.rcptAddrs[0],
			"to@example.com",
		)
	}
	if mc.rcptAddrs[1] != "cc@example.com" {
		t.Errorf(
			"Rcpt[1] = %q, want %q",
			mc.rcptAddrs[1],
			"cc@example.com",
		)
	}
	if mc.dataCalls != 1 {
		t.Errorf("Data called %d times, want 1", mc.dataCalls)
	}
	if mc.dataWriter.buf.String() != body {
		t.Errorf(
			"written data = %q, want %q",
			mc.dataWriter.buf.String(),
			body,
		)
	}
	if mc.quitCalls != 1 {
		t.Errorf("Quit called %d times, want 1", mc.quitCalls)
	}
	if mc.closeCalls != 1 {
		t.Errorf(
			"Close called %d times, want 1 (defer)",
			mc.closeCalls,
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

func TestDial_TLSModes(t *testing.T) {
	// Each mode exercises a different branch in dial().
	// Connections fail (no server), but the code paths are
	// covered.
	modes := []struct {
		name    string
		tlsMode string
	}{
		{"starttls", "starttls"},
		{"implicit", "implicit"},
		{"none", "none"},
		{"empty defaults to starttls", ""},
	}

	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			acct := config.Account{
				SMTPHost: "127.0.0.1",
				SMTPPort: 1, // closed port
				SMTPTLS:  tc.tlsMode,
			}
			_, err := dial(acct)
			if err == nil {
				t.Fatal("expected connection error")
			}
		})
	}
}

func TestDefaultDial_ConnectionFailure(t *testing.T) {
	acct := config.Account{
		SMTPHost: "127.0.0.1",
		SMTPPort: 1,
		SMTPTLS:  "none",
	}
	_, err := defaultDial(acct)
	if err == nil {
		t.Fatal("expected connection error from defaultDial")
	}
}
