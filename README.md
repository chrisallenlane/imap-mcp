# imap-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that provides IMAP email access to Claude and other AI assistants.

## Features

- **Complete MCP Implementation**: Full JSON-RPC 2.0 over stdio
- **Multi-Account Support**: Configure multiple IMAP accounts in a single TOML file
- **Lazy Connections**: IMAP connections are established on first use and reused
- **TLS Support**: Configurable TLS or plaintext connections per account
- **SMTP Sending**: Optional per-account SMTP support for sending, drafting, and replying
- **Tool System**: Clean interface for adding new email capabilities

## Available Tools

| Tool | Description |
|------|-------------|
| `list_accounts` | Lists all configured IMAP accounts with connection status |
| `list_mailboxes` | Lists mailboxes for an account with special-use annotations and message counts |
| `list_messages` | Lists message envelopes with pagination (100 per page, newest first) |
| `get_message` | Retrieves a full message by UID (headers, body, attachments) |
| `get_attachment` | Downloads an email attachment to disk by message UID and attachment number |
| `search_messages` | Searches messages by from, to, subject, body, date range, and flags |
| `mark_messages` | Sets or clears read/flagged status on messages |
| `move_messages` | Moves messages between mailboxes |
| `copy_messages` | Copies messages between mailboxes |
| `delete_messages` | Deletes messages (move to Trash or permanent expunge) |
| `create_mailbox` | Creates a new mailbox (folder) |
| `delete_mailbox` | Deletes a mailbox (with special-use protection) |
| `send_message` | Sends an email via SMTP (requires `smtp_enabled = true`) |
| `save_draft` | Composes and saves a message to the Drafts folder (requires `smtp_enabled = true`) |
| `reply_message` | Replies to, reply-alls, or forwards a message (requires `smtp_enabled = true`) |

## Project Structure

```
imap-mcp/
├── cmd/
│   └── imap-mcp/        # Main application entry point
├── internal/
│   ├── config/          # TOML configuration parsing
│   ├── imap/            # IMAP connection manager
│   ├── smtp/            # SMTP sending manager
│   ├── server/          # MCP JSON-RPC server
│   └── tools/           # Tool implementations
├── scripts/             # Helper scripts (fuzz testing)
├── config.example.toml  # Example configuration
├── Makefile             # Build automation
├── CLAUDE.md            # Development guidance for AI assistants
├── SETUP.md             # Setup and integration guide
└── README.md            # This file
```

## Quick Start

1. **Clone and build:**
   ```bash
   git clone https://github.com/chrisallenlane/imap-mcp.git
   cd imap-mcp
   make build
   ```

2. **Configure:**
   ```bash
   cp config.example.toml config.toml
   # Edit config.toml with your IMAP credentials
   ```

3. **Register with Claude Code:**
   ```bash
   make setup
   ```

That's it. The MCP tools are now available in Claude Code.

See [SETUP.md](SETUP.md) for Claude Desktop integration and manual setup options.

### Prerequisites

- Go 1.24 or later
- Make (optional, but recommended)

### Configuration

Each `[accounts.<name>]` section in `config.toml` defines an IMAP account:

```toml
[accounts.gmail]
host     = "imap.gmail.com"
port     = 993
username = "user@gmail.com"
password = "app-password"
tls      = true

[accounts.protonmail]
host     = "127.0.0.1"
port     = 1143
username = "user@proton.me"
password = "bridge-password"
tls      = false
```

The IMAP fields (host, port, username, password) are required. The `tls` field controls whether to use TLS or plaintext.

To enable sending, add SMTP fields to an account:

```toml
[accounts.gmail]
# ... IMAP fields above ...
smtp_enabled = true
smtp_host    = "smtp.gmail.com"
smtp_port    = 587
smtp_tls     = "starttls"   # "starttls", "implicit", or "none"
# smtp_from = "user@gmail.com"  # defaults to username
# save_sent = false             # save sent messages via IMAP APPEND
```

When `smtp_enabled = true`, `smtp_host` and `smtp_port` are required. `smtp_tls` defaults to `"starttls"` if omitted; valid values are `"starttls"`, `"implicit"`, or `"none"`. SMTP tools (`send_message`, `save_draft`, `reply_message`) are only registered when at least one account has `smtp_enabled = true`.

`config.toml` is gitignored because it contains credentials. See `config.example.toml` for a full example.

### Running

The MCP server communicates via stdin/stdout:

```bash
./dist/imap-mcp --config /path/to/config.toml
```

The `--config` flag is required. Use `--version` to print the version and exit.

### Tested Providers

This project has been tested against the following providers:

- [Gmail](https://mail.google.com/)
- [Protonmail](https://proton.me/mail) (via [Protonmail Mail Bridge](https://proton.me/mail/bridge))

Other standard IMAP/SMTP providers should work but have not been verified.

## Development

### Build

```bash
make build
```

### Test

```bash
# Run all tests
make test

# Run tests with coverage
make coverage
```

### Code Quality

```bash
# Format code
make fmt

# Lint code
make lint

# Vet code
make vet

# Run all checks
make check
```

## Project Conventions

- **Formatting**: 80-column line wrapping with golines + gofumpt
- **Testing**: Standard library `testing` package
- **Error Handling**: Always wrap errors with context

## Architecture

### MCP Protocol Flow

```
Claude -> stdin -> JSON-RPC Request -> Tool Execution -> JSON-RPC Response -> stdout -> Claude
```

### Key Components

- **Server** (`internal/server`): Handles JSON-RPC protocol
- **Config** (`internal/config`): Parses TOML configuration
- **IMAP Manager** (`internal/imap`): Manages persistent IMAP connections
- **SMTP Manager** (`internal/smtp`): Manages SMTP sending (per-operation connections)
- **Tools** (`internal/tools`): Implements MCP tools

## Resources

- [MCP Specification](https://modelcontextprotocol.io/)
- [Go Documentation](https://golang.org/doc/)
- [go-imap/v2](https://pkg.go.dev/github.com/emersion/go-imap/v2)

For detailed development guidance, see `CLAUDE.md`.
