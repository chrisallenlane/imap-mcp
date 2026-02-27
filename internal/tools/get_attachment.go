package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	imap "github.com/emersion/go-imap/v2"
)

// GetAttachment is an MCP tool that downloads an email
// attachment to disk.
type GetAttachment struct {
	getter messageGetter
}

// NewGetAttachment creates a new GetAttachment tool.
func NewGetAttachment(
	getter messageGetter,
) *GetAttachment {
	return &GetAttachment{getter: getter}
}

// Description returns a description of what the tool does.
func (t *GetAttachment) Description() string {
	return "Download an email attachment to disk by " +
		"message UID and attachment number"
}

// InputSchema returns the JSON schema for the tool's input.
func (t *GetAttachment) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"account": map[string]interface{}{
				"type": "string",
				"description": "Account name from " +
					"config (e.g., 'gmail', " +
					"'protonmail')",
			},
			"mailbox": map[string]interface{}{
				"type": "string",
				"description": "Mailbox name " +
					"(e.g., 'INBOX', " +
					"'[Gmail]/Sent Mail')",
			},
			"uid": map[string]interface{}{
				"type":        "integer",
				"description": "Message UID",
			},
			"attachment": map[string]interface{}{
				"type": "integer",
				"description": "1-indexed attachment " +
					"number from get_message output",
			},
			"directory": map[string]interface{}{
				"type": "string",
				"description": "Destination directory " +
					"(defaults to server process " +
					"working directory)",
			},
		},
		"required": []string{
			"account",
			"mailbox",
			"uid",
			"attachment",
		},
	}
}

// Execute downloads an attachment and saves it to disk.
func (t *GetAttachment) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	var params struct {
		Account    string          `json:"account"`
		Mailbox    string          `json:"mailbox"`
		UID        json.RawMessage `json:"uid"`
		Attachment int             `json:"attachment"`
		Directory  string          `json:"directory"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf(
			"failed to parse arguments: %w",
			err,
		)
	}

	if params.Account == "" {
		return "", fmt.Errorf("account is required")
	}
	if params.Mailbox == "" {
		return "", fmt.Errorf("mailbox is required")
	}

	uid, err := parseUID(params.UID)
	if err != nil {
		return "", err
	}
	if uid == 0 {
		return "", fmt.Errorf(
			"uid is required and must be > 0",
		)
	}

	if params.Attachment <= 0 {
		return "", fmt.Errorf(
			"attachment is required and must be > 0",
		)
	}

	msg, err := fetchSingleMessage(
		t.getter,
		params.Account,
		params.Mailbox,
		uid,
		&imap.FetchOptions{
			UID: true,
			BodySection: []*imap.FetchItemBodySection{
				{Peek: true},
			},
		},
	)
	if err != nil {
		return "", err
	}

	bodySection := &imap.FetchItemBodySection{}
	bodyBytes := msg.FindBodySection(bodySection)
	if bodyBytes == nil {
		return "", fmt.Errorf(
			"message has no body data",
		)
	}

	att, err := extractAttachment(
		bodyBytes,
		params.Attachment,
	)
	if err != nil {
		return "", err
	}

	dir := params.Directory
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf(
				"failed to get working directory: %w",
				err,
			)
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf(
			"failed to create directory %s: %w",
			dir,
			err,
		)
	}

	filename := att.filename
	if filename == "" {
		filename = fallbackFilename(
			params.Attachment,
			att.mediaType,
		)
	}

	destPath := uniquePath(
		filepath.Join(dir, filename),
	)

	if err := os.WriteFile(
		destPath,
		att.data,
		0o644,
	); err != nil {
		return "", fmt.Errorf(
			"failed to write file %s: %w",
			destPath,
			err,
		)
	}

	return fmt.Sprintf(
		"Downloaded %s to %s (%s, %s)",
		filepath.Base(destPath),
		destPath,
		formatSize(len(att.data)),
		att.mediaType,
	), nil
}

// fallbackFilename generates a filename from the attachment
// index and media type when the MIME headers have no filename.
func fallbackFilename(index int, mediaType string) string {
	ext := ".bin"
	if exts, err := mime.ExtensionsByType(
		mediaType,
	); err == nil && len(exts) > 0 {
		ext = exts[0]
	}
	return fmt.Sprintf("attachment_%d%s", index, ext)
}

// uniquePath returns a path that does not collide with an
// existing file. If the original path is free, it is returned
// as-is. Otherwise a numeric suffix is appended:
// "file (1).ext", "file (2).ext", etc.
func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	for i := 1; ; i++ {
		candidate := fmt.Sprintf(
			"%s (%d)%s",
			base,
			i,
			ext,
		)
		if _, err := os.Stat(candidate); os.IsNotExist(
			err,
		) {
			return candidate
		}
	}
}
