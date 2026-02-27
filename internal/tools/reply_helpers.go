package tools

import (
	"fmt"
	"strings"

	imaplib "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// buildReplyParams constructs the composeParams and all
// envelope recipients for a reply/reply_all/forward.
func buildReplyParams(
	mode, from string,
	overrideTo, cc, bcc []string,
	userBody string,
	srcMsg *imapclient.FetchMessageBuffer,
	srcBody sourceBody,
) (composeParams, []string, error) {
	env := srcMsg.Envelope
	if env == nil {
		return composeParams{}, nil, fmt.Errorf(
			"source message has no envelope data",
		)
	}

	var cp composeParams
	cp.From = from
	cp.BCC = bcc

	switch mode {
	case "reply":
		cp.To = replyTo(env, overrideTo)
		cp.CC = cc

	case "reply_all":
		cp.To, cp.CC = replyAllRecipients(
			env, from, overrideTo, cc,
		)

	case "forward":
		cp.To = overrideTo
		cp.CC = cc
	}

	// Set subject, body, and threading headers by mode.
	if mode == "forward" {
		cp.Subject = addPrefix("Fwd", env.Subject)
		cp.Body = quoteForward(userBody, env, srcBody)
	} else {
		cp.Subject = addPrefix("Re", env.Subject)
		cp.InReplyTo = env.MessageID
		// References: built from Message-ID only
		// (not directly available in Envelope).
		if env.MessageID != "" {
			cp.References = []string{env.MessageID}
		}
		cp.Body = quoteReply(userBody, env, srcBody)
	}

	return cp, collectRecipients(
		cp.To, cp.CC, cp.BCC,
	), nil
}

// replyTo returns the To addresses for a reply. Uses override
// if provided, otherwise replies to the original sender.
func replyTo(
	env *imaplib.Envelope,
	override []string,
) []string {
	if len(override) > 0 {
		return override
	}
	return envelopeAddrs(env.From)
}

// replyAllRecipients calculates To and CC for reply_all,
// excluding self from both lists.
func replyAllRecipients(
	env *imaplib.Envelope,
	self string,
	overrideTo, overrideCC []string,
) (to, cc []string) {
	if len(overrideTo) > 0 {
		return overrideTo, overrideCC
	}

	selfLower := strings.ToLower(self)

	// To = original From + original To (minus self).
	for _, addr := range env.From {
		email := addr.Addr()
		if strings.ToLower(email) != selfLower {
			to = append(to, email)
		}
	}
	for _, addr := range env.To {
		email := addr.Addr()
		if strings.ToLower(email) != selfLower {
			to = append(to, email)
		}
	}

	// CC = original CC (minus self), merged with override.
	ccSet := make(map[string]bool)
	for _, addr := range overrideCC {
		ccLower := strings.ToLower(addr)
		if ccLower != selfLower {
			cc = append(cc, addr)
			ccSet[ccLower] = true
		}
	}
	for _, addr := range env.Cc {
		email := addr.Addr()
		emailLower := strings.ToLower(email)
		if emailLower != selfLower &&
			!ccSet[emailLower] {
			cc = append(cc, email)
		}
	}

	return to, cc
}

// envelopeAddrs extracts email addresses from IMAP addresses.
func envelopeAddrs(addrs []imaplib.Address) []string {
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if email := addr.Addr(); email != "" {
			result = append(result, email)
		}
	}
	return result
}

// addPrefix adds "Re: " or "Fwd: " to a subject if not
// already present.
func addPrefix(prefix, subject string) string {
	check := prefix + ": "
	if strings.HasPrefix(
		strings.ToLower(subject),
		strings.ToLower(check),
	) {
		return subject
	}
	return check + subject
}

// quoteReply formats the reply body with the original quoted.
func quoteReply(
	userBody string,
	env *imaplib.Envelope,
	srcBody sourceBody,
) string {
	var b strings.Builder
	b.WriteString(userBody)
	b.WriteString("\n\n")

	sender := "(unknown)"
	if len(env.From) > 0 {
		sender = formatAddresses(env.From)
	}

	date := env.Date.Format(
		dateFormatRFC2822,
	)

	fmt.Fprintf(
		&b,
		"On %s, %s wrote:\n",
		date,
		sender,
	)

	if srcBody.text != "" {
		for _, line := range strings.Split(
			srcBody.text, "\n",
		) {
			fmt.Fprintf(&b, "> %s\n", line)
		}
	}

	return b.String()
}

// quoteForward formats the forward body with the original
// message metadata and content.
func quoteForward(
	userBody string,
	env *imaplib.Envelope,
	srcBody sourceBody,
) string {
	var b strings.Builder
	b.WriteString(userBody)

	b.WriteString(
		"\n\n---------- Forwarded message ----------\n",
	)
	fmt.Fprintf(
		&b,
		"From: %s\n",
		formatAddresses(env.From),
	)
	fmt.Fprintf(
		&b,
		"Date: %s\n",
		env.Date.Format(
			dateFormatRFC2822,
		),
	)
	fmt.Fprintf(&b, "Subject: %s\n", env.Subject)
	fmt.Fprintf(
		&b,
		"To: %s\n",
		formatAddresses(env.To),
	)
	b.WriteString("\n")

	if srcBody.text != "" {
		b.WriteString(srcBody.text)
		b.WriteString("\n")
	}

	return b.String()
}

// extractAllAttachments extracts all attachments from a raw
// message as rawAttachment values.
func extractAllAttachments(
	raw []byte,
	count int,
) ([]rawAttachment, error) {
	var result []rawAttachment
	for i := 1; i <= count; i++ {
		att, err := extractAttachment(raw, i)
		if err != nil {
			return nil, fmt.Errorf(
				"attachment %d: %w",
				i,
				err,
			)
		}

		filename := att.filename
		if filename == "" {
			filename = fmt.Sprintf(
				"attachment_%d",
				i,
			)
		}

		result = append(result, rawAttachment{
			Filename:  filename,
			MediaType: att.mediaType,
			Data:      att.data,
		})
	}
	return result, nil
}
