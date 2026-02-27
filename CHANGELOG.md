# Changelog

## v0.2.0

### Added
- **SMTP sending support**: New per-account SMTP configuration (`smtp_enabled`, `smtp_host`, `smtp_port`, `smtp_tls`, `smtp_from`, `save_sent`)
- **`send_message` tool**: Send email via SMTP with support for To/CC/BCC recipients and file attachments
- **`save_draft` tool**: Compose and save messages to the Drafts folder via IMAP APPEND
- **`reply_message` tool**: Reply, reply-all, or forward messages with threading headers and quoted text
- SMTP tools are feature-gated — only registered when at least one account has `smtp_enabled = true`
- Optional save-to-Sent-folder via IMAP APPEND when `save_sent = true`
- Drafts folder detection via SPECIAL-USE `\Drafts` attribute
- Sent folder detection via SPECIAL-USE `\Sent` attribute

### Changed
- SMTP manager (`internal/smtp/`) uses injectable `smtpClient` interface for testability
- `server.New` now accepts an SMTP manager as a second argument (internal-only breaking change)
- Renamed `internal/smtp` package to `smtpmanager` to avoid conflict with the `go-smtp` stdlib dependency
- Extracted shared message composition into `compose.go` (`composeMessage`, `detectMediaType`, `toMailAddresses`)
- Extracted shared SMTP helpers into `smtp_helpers.go` (`resolveSMTPAccount`, `trySaveToSent`, `collectRecipients`)
- Moved `formatAddresses` and `formatSize` to `format.go` with other shared formatting helpers
- Split `reply_message.go` into `reply_message.go` (struct/Execute) and `reply_helpers.go` (building/quoting)

## v0.1.0

Initial release.

### Features
- MCP server implementation via JSON-RPC 2.0 over stdio
- Multi-account IMAP support with TOML configuration
- Lazy connection initialization with automatic reconnection
- 12 IMAP tools: `list_accounts`, `list_mailboxes`, `list_messages`, `get_message`, `get_attachment`, `search_messages`, `mark_messages`, `move_messages`, `copy_messages`, `delete_messages`, `create_mailbox`, `delete_mailbox`
