# CLAUDE.md

This file provides guidance to Claude Code when working with this codebase.

## Project Overview

**imap-mcp** is a Model Context Protocol (MCP) server that provides IMAP email access to Claude and other AI assistants. It connects to one or more IMAP accounts and exposes email operations as MCP tools.

**Tech Stack:**
- **Language**: Go 1.24+
- **Protocol**: MCP (Model Context Protocol) via JSON-RPC 2.0 over stdio
- **Config**: TOML via `github.com/BurntSushi/toml`
- **IMAP**: `github.com/emersion/go-imap/v2`
- **MIME Parsing**: `github.com/emersion/go-message`

## Project Structure

```
imap-mcp/
├── cmd/
│   └── imap-mcp/            # Main application
│       └── main.go              # Entry point, flag parsing, initialization
├── internal/                    # Private application packages
│   ├── config/                  # TOML configuration parsing
│   │   ├── config.go            # Config types and loader
│   │   └── config_test.go       # Config parsing and validation tests
│   ├── imap/                    # IMAP connection manager
│   │   ├── manager.go           # Lazy connection pooling per account
│   │   └── manager_test.go      # Connection manager tests
│   ├── server/                  # MCP server implementation
│   │   ├── server.go            # JSON-RPC server, request routing
│   │   ├── server_test.go       # Protocol tests
│   │   └── types.go             # JSON-RPC request/response types
│   └── tools/                   # MCP tool implementations
│       ├── tool.go              # Tool interface definition
│       ├── format.go            # Shared formatting helpers (formatFlags, flagLabels)
│       ├── helpers_test.go      # Shared test helpers (assertContains)
│       ├── list_accounts.go     # list_accounts tool
│       ├── list_accounts_test.go
│       ├── list_mailboxes.go     # list_mailboxes tool
│       ├── list_mailboxes_test.go
│       ├── list_messages.go      # list_messages tool
│       ├── list_messages_test.go
│       ├── get_message.go        # get_message tool
│       ├── get_message_test.go
│       ├── search_messages.go    # search_messages tool
│       └── search_messages_test.go
├── config.example.toml          # Example configuration file
├── Makefile                     # Build automation
├── CLAUDE.md                    # This file
├── README.md                    # User-facing documentation
└── SETUP.md                     # Setup instructions
```

This follows the **standard Go project layout**:
- `cmd/` - Main application entry points
- `internal/` - Private packages that cannot be imported by external projects

## Architecture

### MCP Protocol Implementation

The server implements MCP via **JSON-RPC 2.0 over stdio**:

1. **Stdin** - JSON-RPC requests from Claude
2. **Process** - Route to handlers, execute tools
3. **Stdout** - JSON-RPC responses back to Claude

**Key Methods:**
- `initialize` - Handshake, declare capabilities
- `tools/list` - Return available tools and their schemas
- `tools/call` - Execute a specific tool

**Flow:**
```
Claude -> stdin -> Scanner -> JSON unmarshal -> handleRequest() -> execute tool -> JSON marshal -> stdout -> Claude
```

### Configuration (`internal/config/`)

TOML-based configuration supporting multiple IMAP accounts.

**Config structure:**
```go
type Config struct {
    Accounts map[string]Account `toml:"accounts"`
}

type Account struct {
    Host     string `toml:"host"`
    Port     int    `toml:"port"`
    Username string `toml:"username"`
    Password string `toml:"password"`
    TLS      bool   `toml:"tls"`
}
```

**Validation** checks that at least one account exists and all required fields (host, port, username, password) are set.

The config file path is passed via the `--config` flag. `config.toml` is gitignored because it contains credentials. `config.example.toml` is committed as a reference.

### IMAP Connection Manager (`internal/imap/`)

Manages persistent IMAP connections per account with lazy initialization:

**Connection & Config:**
- **`NewManager(cfg)`** - Creates a manager from config
- **`GetClient(accountName)`** - Returns an IMAP client, connecting on first use
- **`IsConnected(accountName)`** - Checks if an account has an open connection (no side effects)
- **`Config()`** - Returns the manager's config
- **`Close()`** - Closes all open connections

**Read Operations (read-only mailbox selection):**
- **`ListMailboxes(accountName)`** - Returns all mailboxes for an account (connects lazily if needed, issues IMAP LIST command)
- **`ExamineMailbox(account, mailbox)`** - Selects a mailbox in read-only mode (IMAP EXAMINE) and returns metadata including message count
- **`FetchMessages(account, seqSet, options)`** - Fetches message data (envelopes, flags, UIDs, etc.) for a given sequence set
- **`FetchMessageByUID(account, mailbox, uid, options)`** - Fetches message data for a single message by UID (selects mailbox in read-only mode, then fetches via UID set)
- **`SearchMessages(account, mailbox, criteria)`** - Selects a mailbox in read-only mode and runs an IMAP UID SEARCH with the given criteria, returning matching UIDs
- **`FetchMessagesByUID(account, mailbox, uids, options)`** - Fetches message data for multiple UIDs in a mailbox (selects mailbox in read-only mode, then fetches via UID set)
- **`FindTrashMailbox(account)`** - Scans mailboxes for the `\Trash` special-use attribute and returns its name

**Write Operations (read-write mailbox selection):**
- **`SelectMailbox(account, mailbox)`** - Opens a mailbox in read-write mode (IMAP SELECT) and returns metadata
- **`StoreFlags(account, mailbox, uids, op, flags)`** - Sets or clears flags on messages identified by UIDs (selects mailbox read-write, then issues STORE with silent mode)
- **`MoveMessages(account, mailbox, uids, destMailbox)`** - Moves messages identified by UIDs from one mailbox to another (IMAP MOVE)
- **`CopyMessages(account, mailbox, uids, destMailbox)`** - Copies messages identified by UIDs from one mailbox to another (IMAP COPY)
- **`ExpungeMessages(account, mailbox, uids)`** - Permanently removes messages identified by UIDs from a mailbox (IMAP UID EXPUNGE)
- **`CreateMailbox(account, name)`** - Creates a new mailbox on the server (IMAP CREATE)
- **`DeleteMailbox(account, name)`** - Deletes a mailbox from the server (IMAP DELETE)

Connections are thread-safe (protected by `sync.Mutex`). TLS vs insecure connections are selected based on the account's `tls` config field.

### Tool Interface (`internal/tools/tool.go`)

Every tool must implement:

```go
type Tool interface {
    Execute(ctx context.Context, args json.RawMessage) (string, error)
    Description() string
    InputSchema() map[string]interface{}
}
```

**Execute** - Runs the tool with parsed arguments, returns formatted string response
**Description** - Human-readable description for Claude
**InputSchema** - JSON Schema defining required/optional parameters

### Tool Registration (`internal/server/server.go`)

Tools are registered in `registerTools()`:

```go
s.tools["tool_name"] = tools.NewToolName(s.imap)
```

The server automatically discovers and exposes all registered tools via `tools/list`.

### Server Construction (`internal/server/server.go`)

The server accepts an IMAP connection manager:

```go
s := server.New(mgr)
```

The manager is available as `s.imap` for tools to access IMAP connections.

## Development Workflow

### Building

```bash
# Build executable
make build

# Output: dist/imap-mcp
```

### Testing

```bash
# Format code
make fmt

# Lint code
make lint

# Run vet
make vet

# Run tests
make test

# Run tests with coverage report
make coverage

# All checks (format, lint, vet, test)
make check
```

### Installing

```bash
# Install to $GOPATH/bin
make install
```

### Cleaning

```bash
# Remove built executables
make clean
```

## Adding a New Tool

### 1. Create the tool file in `internal/tools/`

Define a narrow interface for the Manager methods the tool needs, then depend on that interface (not the concrete Manager). This enables lightweight mock-based testing.

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
)

// narrow interface satisfied by *imapmanager.Manager
type myDoer interface {
    DoSomething(account string) (string, error)
}

type MyTool struct {
    doer myDoer
}

func NewMyTool(doer myDoer) *MyTool {
    return &MyTool{doer: doer}
}

func (t *MyTool) Description() string {
    return "Brief description of what this tool does"
}

func (t *MyTool) InputSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "account": map[string]interface{}{
                "type":        "string",
                "description": "IMAP account name",
            },
        },
        "required": []string{"account"},
    }
}

func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    var params struct {
        Account string `json:"account"`
    }
    if err := json.Unmarshal(args, &params); err != nil {
        return "", fmt.Errorf("failed to parse arguments: %w", err)
    }

    if params.Account == "" {
        return "", fmt.Errorf("account is required")
    }

    result, err := t.doer.DoSomething(params.Account)
    if err != nil {
        return "", fmt.Errorf("failed to do something: %w", err)
    }

    return result, nil
}
```

### 2. Register in `internal/server/server.go`

Add to `registerTools()`:
```go
s.tools["my_tool"] = tools.NewMyTool(s.imap)
```

### 3. Write tests for the tool

Create `internal/tools/my_tool_test.go` with table-driven tests covering:
- Valid input
- Missing/invalid parameters
- Error conditions

### 4. Rebuild and test

```bash
make check
make build
```

## Code Quality Standards

### Error Messages
Include context in error messages:
```go
if err != nil {
    return "", fmt.Errorf("descriptive context: %w", err)
}
```

### Testing Requirements
Every new tool should have:
- Input validation tests
- Description and schema tests
- Tests run in `make check`

### Code Organization
- One tool per file
- Tool interface defined in `tool.go`
- Shared formatting helpers (e.g., `formatFlags`, `flagLabels`) live in `format.go`
- Shared test helpers (e.g., `assertContains`) live in `helpers_test.go`

## Current Tools

- **`list_accounts`** - Lists all configured IMAP accounts with host, username, TLS status, and connection state. Takes no parameters. Does not initiate connections.
- **`list_mailboxes`** - Lists all mailboxes for a given IMAP account with special-use annotations (archive, drafts, sent, trash, junk, flagged, all mail, important). INBOX is always listed first, remaining mailboxes sorted alphabetically. Takes required `account` parameter.
- **`list_messages`** - Lists message envelopes in a mailbox with pagination (100 messages per page, newest first). Displays UID, date, sender, subject, and flag indicators (unread, flagged, replied, draft, deleted). Takes required `account` and `mailbox` parameters, optional `page` parameter (default: 1).
- **`get_message`** - Retrieves a full email message by UID, including headers (From, To, CC, Date, Subject), flags, plain text body, and attachment metadata (filename, size, media type). HTML-only bodies are noted but not yet rendered. Body text is truncated at 1 MB. Takes required `account`, `mailbox`, and `uid` parameters.
- **`search_messages`** - Searches messages in a mailbox using IMAP SEARCH criteria. Supports filtering by `from`, `to`, `subject`, `body` text, date range (`since`/`before` in YYYY-MM-DD format), and flags (`flagged`, `seen`). At least one search criterion is required beyond account and mailbox. Results are capped at 100 (newest first), with a note when more matches exist. Takes required `account` and `mailbox` parameters, plus optional search criteria.

## Configuration

The server requires a TOML config file passed via `--config`:

```bash
./dist/imap-mcp --config /path/to/config.toml
```

See `config.example.toml` for the format.

## Response Formatting Guidelines

Tools should return **human-readable formatted strings**, not raw JSON. Claude presents these directly to users.

## Error Handling

**Always wrap errors with context:**
```go
if err != nil {
    return "", fmt.Errorf("descriptive context: %w", err)
}
```

**Handle empty results gracefully:**
```go
if len(items) == 0 {
    return "No items found.", nil
}
```

## Important Patterns

### Context Propagation
- Always accept and pass `context.Context` through the call chain
- Enables timeout and cancellation support
- Tool execution has a 30-second timeout

### JSON Marshaling
- Use `json.RawMessage` for unknown/dynamic structures
- Type assert cautiously when parsing responses
- Provide defaults for missing fields

### Narrow Interfaces for Testability
Tools define narrow interfaces for the Manager methods they need (e.g., `mailboxLister` in `list_mailboxes.go`). The concrete `*imapmanager.Manager` satisfies these implicitly. Tests provide lightweight mock implementations of these interfaces instead of mocking the full Manager. This pattern should be followed when adding new tools.

### IMAP Connection Lifecycle
- Connections are established lazily on first `GetClient()` call
- Connections are pooled and reused across tool calls
- `Manager.Close()` is called via `defer` in `main.go`

## Dependencies

- `github.com/BurntSushi/toml` - TOML config parsing
- `github.com/emersion/go-imap/v2` - IMAP client
- `github.com/emersion/go-message` - RFC 2822/MIME message parsing (used by `get_message` for body extraction and attachment metadata)

## Version Information

- MCP Protocol Version: `2024-11-05`
- Server Version: `0.1.0`
- Go Version: 1.24+ required

## Resources

- MCP Specification: https://modelcontextprotocol.io/
- Go Documentation: https://golang.org/doc/
- go-imap/v2 Documentation: https://pkg.go.dev/github.com/emersion/go-imap/v2
