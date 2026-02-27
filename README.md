# imap-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that provides IMAP email access to Claude and other AI assistants.

## Features

- **Complete MCP Implementation**: Full JSON-RPC 2.0 over stdio
- **Multi-Account Support**: Configure multiple IMAP accounts in a single TOML file
- **Lazy Connections**: IMAP connections are established on first use and reused
- **TLS Support**: Configurable TLS or plaintext connections per account
- **Tool System**: Clean interface for adding new email capabilities

## Available Tools

| Tool | Description |
|------|-------------|
| `list_accounts` | Lists all configured IMAP accounts with connection status |
| `list_mailboxes` | Lists mailboxes for an account with special-use annotations and message counts |
| `list_messages` | Lists message envelopes with pagination (100 per page, newest first) |
| `get_message` | Retrieves a full message by UID (headers, body, attachments) |
| `search_messages` | Searches messages by from, to, subject, body, date range, and flags |
| `mark_messages` | Sets or clears read/flagged status on messages |
| `move_messages` | Moves messages between mailboxes |
| `copy_messages` | Copies messages between mailboxes |
| `delete_messages` | Deletes messages (move to Trash or permanent expunge) |
| `create_mailbox` | Creates a new mailbox (folder) |
| `delete_mailbox` | Deletes a mailbox (with special-use protection) |

## Project Structure

```
imap-mcp/
├── cmd/
│   └── imap-mcp/        # Main application entry point
├── internal/
│   ├── config/          # TOML configuration parsing
│   ├── imap/            # IMAP connection manager
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

All fields (host, port, username, password) are required. The `tls` field controls whether to use TLS or plaintext. `config.toml` is gitignored because it contains credentials.

### Running

The MCP server communicates via stdin/stdout:

```bash
./dist/imap-mcp --config /path/to/config.toml
```

The `--config` flag is required. Use `--version` to print the version and exit.

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
- **Tools** (`internal/tools`): Implements MCP tools

## Resources

- [MCP Specification](https://modelcontextprotocol.io/)
- [Go Documentation](https://golang.org/doc/)
- [go-imap/v2](https://pkg.go.dev/github.com/emersion/go-imap/v2)

For detailed development guidance, see `CLAUDE.md`.
