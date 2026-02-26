package tools

import (
	"strings"

	imap "github.com/emersion/go-imap/v2"
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
