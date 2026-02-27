package tools

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	gomail "github.com/emersion/go-message/mail"
)

// rawAttachment holds an attachment's content and metadata
// for inclusion in a composed message. Used for forwarding
// attachments from a source message.
type rawAttachment struct {
	Filename  string
	MediaType string
	Data      []byte
}

// composeParams holds parameters for composing an email.
type composeParams struct {
	From        string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	Attachments []string // file paths

	// Threading headers (for reply/forward).
	InReplyTo  string
	References []string

	// Pre-loaded attachments (for forwarding from source).
	RawAttachments []rawAttachment
}

// composeMessage creates an RFC 5322 message from the given
// parameters. Returns the raw message bytes.
func composeMessage(p composeParams) ([]byte, error) {
	var buf bytes.Buffer

	var h gomail.Header
	h.SetDate(time.Now())
	h.SetSubject(p.Subject)

	if err := h.GenerateMessageID(); err != nil {
		return nil, fmt.Errorf(
			"failed to generate message ID: %w",
			err,
		)
	}

	h.SetAddressList("From", toMailAddresses(p.From))

	if len(p.To) > 0 {
		h.SetAddressList(
			"To",
			toMailAddresses(p.To...),
		)
	}

	if len(p.CC) > 0 {
		h.SetAddressList(
			"Cc",
			toMailAddresses(p.CC...),
		)
	}

	// BCC is set for message composition but stripped by
	// the SMTP server during delivery.
	if len(p.BCC) > 0 {
		h.SetAddressList(
			"Bcc",
			toMailAddresses(p.BCC...),
		)
	}

	// Threading headers for reply/forward.
	if p.InReplyTo != "" {
		h.Set("In-Reply-To", "<"+p.InReplyTo+">")
	}
	if len(p.References) > 0 {
		h.SetMsgIDList("References", p.References)
	}

	hasAttachments := len(p.Attachments) > 0 ||
		len(p.RawAttachments) > 0
	if !hasAttachments {
		return composePlainMessage(&buf, h, p.Body)
	}

	return composeMultipartMessage(
		&buf, h, p.Body, p.Attachments, p.RawAttachments,
	)
}

// composePlainMessage creates a simple text/plain message.
func composePlainMessage(
	buf *bytes.Buffer,
	h gomail.Header,
	body string,
) ([]byte, error) {
	h.SetContentType("text/plain", map[string]string{
		"charset": "utf-8",
	})

	wc, err := gomail.CreateSingleInlineWriter(buf, h)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create message writer: %w",
			err,
		)
	}

	if _, err := io.WriteString(wc, body); err != nil {
		return nil, fmt.Errorf(
			"failed to write message body: %w",
			err,
		)
	}

	if err := wc.Close(); err != nil {
		return nil, fmt.Errorf(
			"failed to close message writer: %w",
			err,
		)
	}

	return buf.Bytes(), nil
}

// composeMultipartMessage creates a multipart/mixed message
// with a text body, file attachments, and/or raw attachments.
func composeMultipartMessage(
	buf *bytes.Buffer,
	h gomail.Header,
	body string,
	attachments []string,
	rawAttachments []rawAttachment,
) ([]byte, error) {
	mw, err := gomail.CreateWriter(buf, h)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create multipart writer: %w",
			err,
		)
	}

	// Write the text body as an inline part.
	var ih gomail.InlineHeader
	ih.SetContentType("text/plain", map[string]string{
		"charset": "utf-8",
	})

	bodyWriter, err := mw.CreateSingleInline(ih)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create body part: %w",
			err,
		)
	}

	if _, err := io.WriteString(bodyWriter, body); err != nil {
		return nil, fmt.Errorf(
			"failed to write body part: %w",
			err,
		)
	}

	if err := bodyWriter.Close(); err != nil {
		return nil, fmt.Errorf(
			"failed to close body part: %w",
			err,
		)
	}

	// Write file attachments.
	for _, path := range attachments {
		if err := writeAttachment(mw, path); err != nil {
			return nil, err
		}
	}

	// Write raw attachments (forwarded from source).
	for _, att := range rawAttachments {
		if err := writeAttachmentData(
			mw,
			att.Filename,
			att.MediaType,
			att.Data,
		); err != nil {
			return nil, err
		}
	}

	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf(
			"failed to close multipart writer: %w",
			err,
		)
	}

	return buf.Bytes(), nil
}

// writeAttachment reads a file from disk and writes it as a
// MIME attachment part.
func writeAttachment(
	mw *gomail.Writer,
	path string,
) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf(
			"failed to read attachment %q: %w",
			path,
			err,
		)
	}

	filename := filepath.Base(path)
	mediaType := detectMediaType(filename, data)

	return writeAttachmentData(
		mw, filename, mediaType, data,
	)
}

// writeAttachmentData writes a MIME attachment part with the
// given metadata and content.
func writeAttachmentData(
	mw *gomail.Writer,
	filename, mediaType string,
	data []byte,
) error {
	var ah gomail.AttachmentHeader
	ah.SetFilename(filename)
	ah.SetContentType(mediaType, nil)

	aw, err := mw.CreateAttachment(ah)
	if err != nil {
		return fmt.Errorf(
			"failed to create attachment %q: %w",
			filename,
			err,
		)
	}

	if _, err := aw.Write(data); err != nil {
		return fmt.Errorf(
			"failed to write attachment %q: %w",
			filename,
			err,
		)
	}

	if err := aw.Close(); err != nil {
		return fmt.Errorf(
			"failed to close attachment %q: %w",
			filename,
			err,
		)
	}

	return nil
}

// detectMediaType determines the MIME type of a file using
// its extension first, falling back to content sniffing.
func detectMediaType(filename string, data []byte) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		if mt := mime.TypeByExtension(ext); mt != "" {
			return mt
		}
	}
	return http.DetectContentType(data)
}

// toMailAddresses converts email address strings to
// go-message mail.Address pointers.
func toMailAddresses(addrs ...string) []*gomail.Address {
	result := make([]*gomail.Address, len(addrs))
	for i, addr := range addrs {
		result[i] = &gomail.Address{Address: addr}
	}
	return result
}
