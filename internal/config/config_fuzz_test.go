package config

import "testing"

func FuzzValidate(f *testing.F) {
	// Valid: all fields populated.
	f.Add("work", "imap.example.com", 993, "user", "pass", true,
		false, "", 0, "")

	// Missing host.
	f.Add("acct", "", 993, "user", "pass", true,
		false, "", 0, "")

	// Missing port.
	f.Add("acct", "host.com", 0, "user", "pass", true,
		false, "", 0, "")

	// Missing username.
	f.Add("acct", "host.com", 993, "", "pass", true,
		false, "", 0, "")

	// Missing password.
	f.Add("acct", "host.com", 993, "user", "", true,
		false, "", 0, "")

	// Empty account name.
	f.Add("", "host.com", 993, "user", "pass", false,
		false, "", 0, "")

	// Unicode in fields.
	f.Add("conta", "höst.de", 143, "über", "pässwörd", false,
		false, "", 0, "")

	// Special characters.
	f.Add(
		"test<>",
		"host with spaces",
		65535,
		"user@domain",
		"p@ss!#$%",
		true,
		false, "", 0, "",
	)

	// SMTP enabled with valid fields.
	f.Add("acct", "host.com", 993, "user", "pass", true,
		true, "smtp.host.com", 587, "starttls")

	// SMTP enabled missing host.
	f.Add("acct", "host.com", 993, "user", "pass", true,
		true, "", 587, "starttls")

	// SMTP enabled missing port.
	f.Add("acct", "host.com", 993, "user", "pass", true,
		true, "smtp.host.com", 0, "starttls")

	// SMTP enabled invalid TLS mode.
	f.Add("acct", "host.com", 993, "user", "pass", true,
		true, "smtp.host.com", 587, "invalid")

	f.Fuzz(func(
		t *testing.T,
		name, host string,
		port int,
		username, password string,
		tls bool,
		smtpEnabled bool,
		smtpHost string,
		smtpPort int,
		smtpTLS string,
	) {
		cfg := &Config{
			Accounts: map[string]Account{
				name: {
					Host:        host,
					Port:        port,
					Username:    username,
					Password:    password,
					TLS:         tls,
					SMTPEnabled: smtpEnabled,
					SMTPHost:    smtpHost,
					SMTPPort:    smtpPort,
					SMTPTLS:     smtpTLS,
				},
			},
		}

		err := cfg.Validate()

		// Any missing IMAP field must fail.
		imapMissing := host == "" ||
			port == 0 ||
			username == "" ||
			password == ""
		if imapMissing && err == nil {
			t.Error(
				"expected error with missing " +
					"IMAP field, got nil",
			)
		}

		// SMTP enabled with missing SMTP fields must fail.
		if !imapMissing && smtpEnabled {
			smtpMissing := smtpHost == "" || smtpPort == 0
			smtpBadTLS := !validSMTPTLS[smtpTLS]
			if (smtpMissing || smtpBadTLS) && err == nil {
				t.Error(
					"expected error with invalid " +
						"SMTP config, got nil",
				)
			}
			if !smtpMissing && !smtpBadTLS && err != nil {
				t.Errorf(
					"expected no error with valid "+
						"SMTP config, got: %v",
					err,
				)
			}
		}

		// All IMAP fields present, SMTP disabled must pass.
		if !imapMissing && !smtpEnabled && err != nil {
			t.Errorf(
				"expected no error with all IMAP "+
					"fields present and SMTP disabled, got: %v",
				err,
			)
		}
	})
}
