package tools

import (
	"fmt"
	"strings"
	"time"

	imap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// flagLabels maps IMAP flags to human-readable labels.
var flagLabels = []struct {
	flag  imap.Flag
	label string
}{
	{imap.FlagSeen, ""},
	{imap.FlagFlagged, "flagged"},
	{imap.FlagAnswered, "replied"},
	{imap.FlagDraft, "draft"},
	{imap.FlagDeleted, "deleted"},
}

// formatFlags returns a comma-separated string of
// human-readable flag labels. The absence of \Seen produces
// "unread".
func formatFlags(flags []imap.Flag) string {
	seen := false
	var labels []string

	for _, fl := range flagLabels {
		for _, f := range flags {
			if f == fl.flag {
				if fl.flag == imap.FlagSeen {
					seen = true
				} else if fl.label != "" {
					labels = append(labels, fl.label)
				}
				break
			}
		}
	}

	if !seen {
		labels = append([]string{"unread"}, labels...)
	}

	return strings.Join(labels, ", ")
}

// formatMessage formats a single message envelope line.
func formatMessage(
	b *strings.Builder,
	msg *imapclient.FetchMessageBuffer,
) {
	if msg.Envelope == nil {
		fmt.Fprintf(
			b,
			"  UID %-5d  (no envelope data)\n",
			msg.UID,
		)
		return
	}

	env := msg.Envelope

	from := "(unknown)"
	if len(env.From) > 0 {
		addr := env.From[0].Addr()
		if addr != "" {
			from = addr
		}
	}

	date := env.Date.Format("2006-01-02")

	suffix := ""
	if flags := formatFlags(msg.Flags); flags != "" {
		suffix = fmt.Sprintf("  [%s]", flags)
	}

	fmt.Fprintf(
		b,
		"  UID %-5d  %s  %-25s  %s%s\n",
		msg.UID,
		date,
		from,
		env.Subject,
		suffix,
	)
}

// envelopeDate returns the envelope date from a message,
// or the zero time if the envelope is nil.
func envelopeDate(
	msg *imapclient.FetchMessageBuffer,
) time.Time {
	if msg.Envelope != nil {
		return msg.Envelope.Date
	}
	return time.Time{}
}
