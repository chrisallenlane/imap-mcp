package tools

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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
	{imap.FlagFlagged, "flagged"},
	{imap.FlagAnswered, "replied"},
	{imap.FlagDraft, "draft"},
	{imap.FlagDeleted, "deleted"},
}

// formatFlags returns a comma-separated string of
// human-readable flag labels. The absence of \Seen produces
// "unread".
func formatFlags(flags []imap.Flag) string {
	var labels []string
	for _, fl := range flagLabels {
		if slices.Contains(flags, fl.flag) {
			labels = append(labels, fl.label)
		}
	}

	if !slices.Contains(flags, imap.FlagSeen) {
		labels = append(
			[]string{"unread"}, labels...,
		)
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

// formatUIDs formats a slice of uint32 UIDs as a
// comma-separated string.
func formatUIDs(uids []uint32) string {
	strs := make([]string, len(uids))
	for i, u := range uids {
		strs[i] = fmt.Sprintf("%d", u)
	}
	return strings.Join(strs, ", ")
}

// toIMAPUIDs converts a slice of uint32 to a slice of
// imap.UID.
func toIMAPUIDs(ids []uint32) []imap.UID {
	uids := make([]imap.UID, len(ids))
	for i, u := range ids {
		uids[i] = imap.UID(u)
	}
	return uids
}

// formatFlagNames converts a slice of imap.Flag to a
// comma-separated string of flag names.
func formatFlagNames(flags []imap.Flag) string {
	names := make([]string, len(flags))
	for i, f := range flags {
		names[i] = string(f)
	}
	return strings.Join(names, ", ")
}

// parseUID parses a UID from a JSON value that may be either
// a JSON number or a JSON string containing an integer.
// This handles LLMs that pass UIDs as quoted strings.
func parseUID(data json.RawMessage) (imap.UID, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("uid is required")
	}

	// Try as a number first.
	var n uint32
	if err := json.Unmarshal(data, &n); err == nil {
		return imap.UID(n), nil
	}

	// Fall back to string.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return 0, fmt.Errorf("uid must be an integer")
	}

	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf(
			"uid must be a valid integer: %w", err,
		)
	}

	return imap.UID(v), nil
}

// parseUIDs parses UIDs from JSON that may be:
//   - a JSON array of numbers: [57086, 57093]
//   - a JSON array of strings: ["57086", "57093"]
//   - a JSON array of mixed:   [57086, "57093"]
//   - a JSON string of comma-separated values: "57086, 57093"
//   - a single JSON number: 57086
//
// This handles LLMs that serialize UID lists in various forms.
func parseUIDs(data json.RawMessage) ([]uint32, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("uids is required")
	}

	// Try as []uint32 first (fast path).
	var nums []uint32
	if err := json.Unmarshal(data, &nums); err == nil {
		return nums, nil
	}

	// Try as array of mixed number/string elements.
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		result := make([]uint32, len(raw))
		for i, elem := range raw {
			uid, err := parseUID(elem)
			if err != nil {
				return nil, fmt.Errorf(
					"uids[%d]: %w", i, err,
				)
			}
			result[i] = uint32(uid)
		}
		return result, nil
	}

	// Try as a comma-separated string
	// (e.g., "57086, 57093").
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		return parseCommaSeparatedUIDs(s)
	}

	// Try as a single number (e.g., 57086).
	uid, err := parseUID(data)
	if err != nil {
		return nil, fmt.Errorf(
			"uids must be an array of integers",
		)
	}
	return []uint32{uint32(uid)}, nil
}

// parseCommaSeparatedUIDs parses a comma-separated string
// of UIDs (e.g., "57086, 57093, 57096"). Also handles
// stringified JSON array literals like "[57086, 57093]".
func parseCommaSeparatedUIDs(
	s string,
) ([]uint32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	parts := strings.Split(s, ",")
	result := make([]uint32, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		v, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid UID %q: %w", part, err,
			)
		}
		result = append(result, uint32(v))
	}

	if len(result) == 0 {
		return nil, fmt.Errorf(
			"uids must not be empty",
		)
	}

	return result, nil
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
