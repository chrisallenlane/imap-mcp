package tools

import (
	"fmt"

	imap "github.com/emersion/go-imap/v2"

	"github.com/chrisallenlane/imap-mcp/internal/config"
)

// resolveSMTPAccount looks up the account in the config,
// verifies that SMTP is enabled, and returns the account
// and resolved from-address.
func resolveSMTPAccount(
	cfg *config.Config,
	accountName string,
) (config.Account, string, error) {
	acct, ok := cfg.Accounts[accountName]
	if !ok {
		return config.Account{}, "", fmt.Errorf(
			"unknown account: %q",
			accountName,
		)
	}

	if !acct.SMTPEnabled {
		return config.Account{}, "", fmt.Errorf(
			"SMTP is not enabled for account %q. "+
				"Set smtp_enabled = true in your "+
				"config file.",
			accountName,
		)
	}

	from := acct.SMTPFrom
	if from == "" {
		from = acct.Username
	}

	return acct, from, nil
}

// trySaveToSent attempts to save a message to the Sent
// folder. Returns true if successful, false otherwise.
// Errors are silently ignored (the send already succeeded).
func trySaveToSent(
	saver sentSaver,
	account string,
	msgBytes []byte,
) bool {
	sentMailbox, err := saver.FindSentMailbox(account)
	if err != nil {
		return false
	}

	if err := saver.AppendMessage(
		account,
		sentMailbox,
		msgBytes,
		[]imap.Flag{imap.FlagSeen},
	); err != nil {
		return false
	}

	return true
}

// collectRecipients merges To, CC, and BCC into a single
// slice of envelope recipients.
func collectRecipients(
	to, cc, bcc []string,
) []string {
	all := make(
		[]string,
		0,
		len(to)+len(cc)+len(bcc),
	)
	all = append(all, to...)
	all = append(all, cc...)
	all = append(all, bcc...)
	return all
}
