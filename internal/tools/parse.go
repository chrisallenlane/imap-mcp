package tools

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // register charsets
)

// maxBodySize is the maximum number of bytes to read from a
// plain-text message body. Bodies exceeding this limit are
// truncated.
const maxBodySize = 1 << 20 // 1 MB

// attachment holds metadata about a MIME attachment.
type attachment struct {
	filename  string
	size      int
	mediaType string
}

// parsedBody holds the result of parsing a message body.
type parsedBody struct {
	text        string
	attachments []attachment
	fromHTML    bool
}

// parseBody parses raw RFC 2822 message bytes, extracting the
// plain text body (or HTML fallback) and attachment metadata.
func parseBody(raw []byte) (parsedBody, error) {
	entity, err := message.Read(bytes.NewReader(raw))
	if err != nil && !message.IsUnknownCharset(err) {
		return parsedBody{}, fmt.Errorf(
			"failed to read message: %w",
			err,
		)
	}

	var plainText string
	var htmlText string
	var attachments []attachment
	var plainFound bool
	var htmlFound bool

	walkErr := entity.Walk(
		func(
			path []int,
			part *message.Entity,
			err error,
		) error {
			if err != nil {
				return nil
			}

			mediaType, ctParams, _ := part.Header.ContentType()
			disp, dispParams, _ := part.Header.ContentDisposition()

			// Collect the first text/plain or text/html part.
			for _, tp := range []struct {
				media string
				text  *string
				found *bool
			}{
				{"text/plain", &plainText, &plainFound},
				{"text/html", &htmlText, &htmlFound},
			} {
				if mediaType == tp.media &&
					disp != "attachment" &&
					!*tp.found {
					data, readErr := readBodyPart(
						part.Body,
					)
					if readErr != nil {
						return nil
					}
					*tp.text = data
					*tp.found = true
					return nil
				}
			}

			isAttachment := disp == "attachment" ||
				(path != nil &&
					!strings.HasPrefix(
						mediaType,
						"text/",
					) &&
					!strings.HasPrefix(
						mediaType,
						"multipart/",
					))

			if isAttachment {
				filename := dispParams["filename"]
				if filename == "" {
					filename = ctParams["name"]
				}
				if filename == "" {
					filename = "unnamed"
				}

				n, copyErr := io.Copy(
					io.Discard,
					part.Body,
				)
				if copyErr != nil {
					return nil
				}

				attachments = append(
					attachments,
					attachment{
						filename:  filename,
						size:      int(n),
						mediaType: mediaType,
					},
				)
			}

			return nil
		},
	)
	if walkErr != nil {
		return parsedBody{}, fmt.Errorf(
			"failed to walk message parts: %w",
			walkErr,
		)
	}

	// Prefer text/plain; fall back to text/html.
	if plainFound {
		return parsedBody{
			text:        plainText,
			attachments: attachments,
		}, nil
	}
	if htmlFound {
		converted := HTMLToText(htmlText)
		return parsedBody{
			text:        converted,
			attachments: attachments,
			fromHTML:    true,
		}, nil
	}

	return parsedBody{attachments: attachments}, nil
}

// readBodyPart reads a MIME body part up to maxBodySize,
// appending a truncation notice if the limit is exceeded.
func readBodyPart(r io.Reader) (string, error) {
	lr := &io.LimitedReader{
		R: r,
		N: maxBodySize + 1,
	}
	data, err := io.ReadAll(lr)
	if err != nil {
		return "", err
	}
	if len(data) > maxBodySize {
		return string(
			data[:maxBodySize],
		) + "\n\n[body truncated at 1 MB]", nil
	}
	return string(data), nil
}
