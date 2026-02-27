package config

import "testing"

func FuzzValidate(f *testing.F) {
	// Valid: all fields populated.
	f.Add("work", "imap.example.com", 993, "user", "pass", true)

	// Missing host.
	f.Add("acct", "", 993, "user", "pass", true)

	// Missing port.
	f.Add("acct", "host.com", 0, "user", "pass", true)

	// Missing username.
	f.Add("acct", "host.com", 993, "", "pass", true)

	// Missing password.
	f.Add("acct", "host.com", 993, "user", "", true)

	// Empty account name.
	f.Add("", "host.com", 993, "user", "pass", false)

	// Unicode in fields.
	f.Add("conta", "höst.de", 143, "über", "pässwörd", false)

	// Special characters.
	f.Add(
		"test<>",
		"host with spaces",
		65535,
		"user@domain",
		"p@ss!#$%",
		true,
	)

	f.Fuzz(func(
		t *testing.T,
		name, host string,
		port int,
		username, password string,
		tls bool,
	) {
		cfg := &Config{
			Accounts: map[string]Account{
				name: {
					Host:     host,
					Port:     port,
					Username: username,
					Password: password,
					TLS:      tls,
				},
			},
		}

		err := cfg.Validate()

		// All required fields present must pass.
		allPresent := host != "" &&
			port != 0 &&
			username != "" &&
			password != ""
		if allPresent && err != nil {
			t.Errorf(
				"expected no error with all "+
					"fields present, got: %v",
				err,
			)
		}

		// Any missing field must fail.
		anyMissing := host == "" ||
			port == 0 ||
			username == "" ||
			password == ""
		if anyMissing && err == nil {
			t.Error(
				"expected error with missing " +
					"field, got nil",
			)
		}
	})
}
