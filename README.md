# imap-mcp

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that provides IMAP email access to Claude and other AI assistants.

## Features

- **Complete MCP Implementation**: Full JSON-RPC 2.0 over stdio
- **Multi-Account Support**: Configure multiple IMAP accounts in a single TOML file
- **Lazy Connections**: IMAP connections are established on first use and reused
- **TLS Support**: Configurable TLS or plaintext connections per account
- **Tool System**: Clean interface for adding new email capabilities

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
├── config.example.toml  # Example configuration
├── Makefile             # Build automation
└── README.md            # This file
```

## Getting Started

### Prerequisites

- Go 1.24 or later
- Make (optional, but recommended)

### Installation

```bash
git clone https://github.com/chrisallenlane/imap-mcp.git
cd imap-mcp
make build
```

### Configuration

Copy the example config and fill in your IMAP account details:

```bash
cp config.example.toml config.toml
```

Edit `config.toml`:

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

Each `[accounts.<name>]` section defines an IMAP account. All fields (host, port, username, password) are required. The `tls` field controls whether to use TLS or plaintext.

`config.toml` is gitignored because it contains credentials.

### Running

The MCP server communicates via stdin/stdout:

```bash
./dist/imap-mcp --config /path/to/config.toml
```

The `--config` flag is required. See SETUP.md for integration with Claude Desktop and Claude Code.

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

## License

MIT License - see LICENSE file for details

## Resources

- [MCP Specification](https://modelcontextprotocol.io/)
- [Go Documentation](https://golang.org/doc/)
- [go-imap/v2](https://pkg.go.dev/github.com/emersion/go-imap/v2)

For detailed development guidance, see `CLAUDE.md`.
