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
- **HTML Parsing**: `golang.org/x/net/html`

## Project Structure

```
imap-mcp/
├── cmd/
│   └── imap-mcp/            # Main application
│       └── main.go              # Entry point, flag parsing, initialization
├── internal/                    # Private application packages
│   ├── config/                  # TOML configuration parsing
│   │   ├── config.go            # Config types and loader
│   │   ├── config_test.go       # Config parsing and validation tests
│   │   └── config_fuzz_test.go  # Fuzz tests for config validation
│   ├── imap/                    # IMAP connection manager
│   │   ├── manager.go           # ConnectionManager struct, connection lifecycle (GetClient, Close, connect, selectMailbox)
│   │   ├── manager_test.go      # Connection lifecycle tests + shared test helpers
│   │   ├── retry.go             # Auto-reconnect retry logic (withRetryResult, withRetry)
│   │   ├── retry_test.go        # Retry logic tests
│   │   ├── message.go           # Message IMAP operations (Fetch, Search, Store, Move, Copy, Expunge)
│   │   ├── message_test.go      # Message operation tests
│   │   ├── mailbox.go           # Mailbox IMAP operations (List, Examine, Status, Create, Delete)
│   │   └── mailbox_test.go      # Mailbox operation tests
│   ├── server/                  # MCP server implementation
│   │   ├── server.go            # JSON-RPC server, request routing
│   │   ├── server_test.go       # Protocol tests
│   │   ├── server_fuzz_test.go  # Fuzz tests for JSON-RPC handling
│   │   └── types.go             # JSON-RPC request/response types
│   └── tools/                   # MCP tool implementations
│       ├── tool.go              # Tool interface definition
│       ├── format.go            # Shared formatting helpers (formatFlags, formatMessage, formatUIDs, toIMAPUIDs, etc.)
│       ├── format_test.go       # Format helper tests
│       ├── format_fuzz_test.go  # Fuzz tests for format helpers
│       ├── html.go              # HTML-to-text conversion (HTMLToText)
│       ├── html_test.go         # HTML-to-text tests
│       ├── html_fuzz_test.go    # Fuzz tests for HTML-to-text
│       ├── helpers_test.go      # Shared test helpers (assertContains, assertNotContains)
│       ├── list_accounts.go     # list_accounts tool
│       ├── list_accounts_test.go
│       ├── list_mailboxes.go     # list_mailboxes tool
│       ├── list_mailboxes_test.go
│       ├── list_messages.go      # list_messages tool
│       ├── list_messages_test.go
│       ├── list_messages_fuzz_test.go
│       ├── get_message.go        # get_message tool (presentation/formatting)
│       ├── get_message_test.go
│       ├── get_message_fuzz_test.go
│       ├── parse.go              # MIME body parsing (parseBody, readBodyPart, attachment)
│       ├── search_messages.go    # search_messages tool
│       ├── search_messages_test.go
│       ├── search_messages_fuzz_test.go
│       ├── mark_messages.go      # mark_messages tool
│       ├── mark_messages_test.go
│       ├── transfer_messages.go  # Shared transferTool implementation + NewMoveMessages/NewCopyMessages constructors
│       ├── transfer_messages_test.go
│       ├── delete_messages.go    # delete_messages tool
│       ├── delete_messages_test.go
│       ├── create_mailbox.go     # create_mailbox tool
│       ├── create_mailbox_test.go
│       ├── delete_mailbox.go     # delete_mailbox tool
│       └── delete_mailbox_test.go
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

Manages persistent IMAP connections per account with lazy initialization and transparent auto-reconnect:

**Connection & Config:**
- **`NewConnectionManager(cfg)`** - Creates a connection manager from config
- **`GetClient(accountName)`** - Returns an IMAP client, connecting on first use. Detects dead cached connections (via `imapclient.Client.Closed()` channel) and reconnects automatically.
- **`IsConnected(accountName)`** - Checks if an account has a live connection (returns false for dead cached connections)
- **`Config()`** - Returns the manager's config
- **`Close()`** - Closes all open connections (returns `error`)

**Auto-Reconnect:**
All read and write operations are wrapped in `withRetryResult` (generic, returns `(T, error)`) which:
1. Get or reconnect the client (dead connections are evicted proactively)
2. Execute the operation
3. If the operation fails and the connection is dead, evict the dead client, reconnect once, and retry
4. If the retry also fails, return the error to the caller

`withRetryResult` is a package-level generic function. `withRetry` is a method on `*ConnectionManager` that wraps `withRetryResult` for operations that return only an error.

Reconnections are logged to stderr via `log.Printf`. The identity check `m.conns[account] == client` prevents race conditions when multiple goroutines detect the same dead connection.

**Read Operations (read-only mailbox selection):**
- **`ListMailboxes(accountName)`** - Returns all mailboxes for an account (connects lazily if needed, issues IMAP LIST command)
- **`ExamineMailbox(account, mailbox)`** - Selects a mailbox in read-only mode (IMAP EXAMINE) and returns metadata including message count
- **`FetchMessages(account, seqSet, options)`** - Fetches message data (envelopes, flags, UIDs, etc.) for a given sequence set
- **`SearchMessages(account, mailbox, criteria)`** - Selects a mailbox in read-only mode and runs an IMAP UID SEARCH with the given criteria, returning matching UIDs
- **`FetchMessagesByUID(account, mailbox, uids, options)`** - Fetches message data for multiple UIDs in a mailbox (selects mailbox in read-only mode, then fetches via UID set)
- **`MailboxStatus(account, mailbox)`** - Issues an IMAP STATUS command for a mailbox, returning message count and unseen count
- **`FindTrashMailbox(account)`** - Scans mailboxes for the `\Trash` special-use attribute and returns its name

**Write Operations (read-write mailbox selection):**
- **`StoreFlags(account, mailbox, uids, op, flags)`** - Sets or clears flags on messages identified by UIDs (selects mailbox read-write, then issues STORE with silent mode)
- **`MoveMessages(account, mailbox, uids, destMailbox)`** - Moves messages identified by UIDs from one mailbox to another (IMAP MOVE). Delegates to `transferMessages`.
- **`CopyMessages(account, mailbox, uids, destMailbox)`** - Copies messages identified by UIDs from one mailbox to another (IMAP COPY). Delegates to `transferMessages`.
- **`ExpungeMessages(account, mailbox, uids)`** - Permanently removes messages identified by UIDs from a mailbox (IMAP UID EXPUNGE)
- **`CreateMailbox(account, name)`** - Creates a new mailbox on the server (IMAP CREATE)
- **`DeleteMailbox(account, name)`** - Deletes a mailbox from the server (IMAP DELETE)

**Internal Helpers (in `manager.go`):**
- **`selectMailbox(client, mailbox, readOnly)`** - Selects a mailbox in read-only or read-write mode and returns metadata. Used by both read and write operations.

**Internal Helpers (in `message.go`):**
- **`transferMessages(account, mailbox, uids, destMailbox, verb, op)`** - Shared helper for move/copy operations. Validates UIDs, selects the mailbox read-write, and executes the transfer operation with retry.

Connections are thread-safe (protected by `sync.Mutex`). TLS vs insecure connections are selected based on the account's `tls` config field. Dead connections are detected and replaced transparently — callers never need to handle reconnection.

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

# Run fuzz tests (FUZZTIME=30s by default)
make fuzz

# All checks (format, lint, vet, test)
make check
```

### Installing

```bash
# Install to $GOPATH/bin
make install
```

### Claude Code Setup

```bash
# Register imap-mcp with Claude Code (requires config.toml)
make setup
```

This builds the binary, checks for `config.toml`, and runs `claude mcp add` with absolute paths. It gives a clear error with guidance if `config.toml` is missing.

### Cleaning

```bash
# Remove built executables
make clean
```

## Adding a New Tool

### 1. Create the tool file in `internal/tools/`

Define a narrow interface for the ConnectionManager methods the tool needs, then depend on that interface (not the concrete ConnectionManager). This enables lightweight mock-based testing.

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
)

// narrow interface satisfied by *imapmanager.ConnectionManager
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
- One tool per file, except `move_messages` and `copy_messages` which share a `transferTool` implementation (and their constructors) in `transfer_messages.go`
- Tool interface defined in `tool.go`
- MIME body parsing separated into `parse.go` (parsing concern) while `get_message.go` handles presentation
- Shared formatting helpers (e.g., `formatFlags`, `formatMessage`, `formatUIDs`, `toIMAPUIDs`, `formatFlagNames`, `envelopeDate`) live in `format.go`; `formatAddresses` lives in `get_message.go`
- Shared test helpers (e.g., `assertContains`) live in `helpers_test.go`
- The `internal/imap/` package is organized by domain noun: `manager.go` (connection lifecycle + shared helpers), `message.go` (message operations), `mailbox.go` (mailbox operations), `retry.go` (reconnect logic)

## Current Tools

- **`list_accounts`** - Lists all configured IMAP accounts with host, username, TLS status, and connection state. Takes no parameters. Does not initiate connections.
- **`list_mailboxes`** - Lists all mailboxes for a given IMAP account with special-use annotations (archive, drafts, sent, trash, junk, flagged, all mail, important) and message counts (total and unread). INBOX is always listed first, remaining mailboxes sorted alphabetically. Mailboxes with the `\Noselect` attribute are listed without counts. Takes required `account` parameter.
- **`list_messages`** - Lists message envelopes in a mailbox with pagination (100 messages per page, newest first). Displays UID, date, sender, subject, and flag indicators (unread, flagged, replied, draft, deleted). Takes required `account` and `mailbox` parameters, optional `page` parameter (default: 1).
- **`get_message`** - Retrieves a full email message by UID, including headers (From, To, CC, Date, Subject), flags, body text, and attachment metadata (filename, size, media type). Prefers `text/plain` body parts; falls back to HTML-to-text conversion via `HTMLToText()` (in `html.go`) when only `text/html` is available. HTML conversion strips tags, removes script/style blocks, preserves links as `text (url)`, decodes entities, and collapses blank lines. Output indicates when HTML conversion was used ("Body (converted from HTML):"). Body text is truncated at 1 MB. Takes required `account`, `mailbox`, and `uid` parameters.
- **`search_messages`** - Searches messages in a mailbox using IMAP SEARCH criteria. Supports filtering by `from`, `to`, `subject`, `body` text, date range (`since`/`before` in YYYY-MM-DD format), and flags (`flagged`, `seen`). At least one search criterion is required beyond account and mailbox. Results are capped at 100 (newest first), with a note when more matches exist. Takes required `account` and `mailbox` parameters, plus optional search criteria.
- **`mark_messages`** - Sets or clears flags on messages. Supports `read` (boolean, maps to `\Seen`) and `flagged` (boolean, maps to `\Flagged`). At least one flag parameter is required. Uses pointer booleans to distinguish "not provided" from "false". Batches add/remove into minimal `StoreFlags` calls. Takes required `account`, `mailbox`, and `uids` parameters, plus optional `read` and `flagged` booleans.
- **`move_messages`** - Moves messages from one mailbox to another via IMAP MOVE (RFC 6851). Destination must differ from source. After a move, source UIDs are invalidated (expected IMAP behavior). Takes required `account`, `mailbox`, `uids`, and `destination` parameters.
- **`copy_messages`** - Copies messages from one mailbox to another via IMAP COPY. Original messages remain in the source mailbox. Destination must differ from source. Copied messages get new UIDs in the destination. Takes required `account`, `mailbox`, `uids`, and `destination` parameters.
- **`delete_messages`** - Deletes messages by moving to Trash (default) or permanently expunging (`permanent: true`). Trash folder detected via SPECIAL-USE `\Trash` attribute. Returns error with guidance if no Trash folder found or messages are already in Trash. Permanent delete sets `\Deleted` flag then issues UID EXPUNGE. Takes required `account`, `mailbox`, and `uids` parameters, plus optional `permanent` boolean.
- **`create_mailbox`** - Creates a new mailbox (folder) via IMAP CREATE. Intermediate hierarchy levels are created automatically by most servers. Takes required `account` and `name` parameters.
- **`delete_mailbox`** - Deletes a mailbox (folder) via IMAP DELETE. Refuses to delete INBOX (case-insensitive) or any mailbox with SPECIAL-USE attributes (`\Sent`, `\Trash`, `\Drafts`, `\Junk`, `\Archive`, `\Flagged`). Takes required `account` and `name` parameters.

## Configuration

The server requires a TOML config file passed via `--config`:

```bash
./dist/imap-mcp --config /path/to/config.toml
```

Use `--version` to print the version and exit (works without a config file).

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
Tools define narrow interfaces for the ConnectionManager methods they need (e.g., `mailboxLister` in `list_mailboxes.go`). The concrete `*imapmanager.ConnectionManager` satisfies these implicitly. Tests provide lightweight mock implementations of these interfaces instead of mocking the full ConnectionManager. This pattern should be followed when adding new tools.

### IMAP Connection Lifecycle
- Connections are established lazily on first `GetClient()` call
- Connections are pooled and reused across tool calls
- Dead connections are detected (via `Closed()` channel) and replaced transparently
- All operations retry once on connection death before returning an error
- `ConnectionManager.Close()` is called via `defer` in `main.go`

## Dependencies

- `github.com/BurntSushi/toml` - TOML config parsing
- `github.com/emersion/go-imap/v2` - IMAP client
- `github.com/emersion/go-message` - RFC 2822/MIME message parsing (used by `get_message` for body extraction and attachment metadata)
- `golang.org/x/net/html` - HTML tokenizer/parser (used by `HTMLToText` for HTML-to-text conversion of email bodies)

## Version Information

- MCP Protocol Version: `2024-11-05`
- Server Version: `0.1.0`
- Go Version: 1.24+ required

## Resources

- MCP Specification: https://modelcontextprotocol.io/
- Go Documentation: https://golang.org/doc/
- go-imap/v2 Documentation: https://pkg.go.dev/github.com/emersion/go-imap/v2
