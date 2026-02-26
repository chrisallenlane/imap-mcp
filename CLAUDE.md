# CLAUDE.md

This file provides guidance to Claude Code when working with this MCP server template.

## Project Overview

**go-mcp-server** is a template for building Model Context Protocol (MCP) servers in Go. It provides a complete, production-ready foundation for creating MCP servers that integrate external services with Claude and other AI assistants.

**Tech Stack:**
- **Language**: Go 1.21+
- **Protocol**: MCP (Model Context Protocol) via JSON-RPC 2.0 over stdio
- **Dependencies**: Minimal - Go stdlib only for production code

## Project Structure

```
go-mcp-server/
├── cmd/
│   └── go-mcp-server/        # Main application
│       └── main.go          # Entry point, configuration, initialization
├── internal/                # Private application packages
│   ├── client/              # HTTP client (customize for your API)
│   │   ├── client.go        # HTTP client with request helpers
│   │   └── client_test.go   # Client tests
│   ├── models/              # Type-safe data structures
│   │   ├── models.go        # Placeholder models (replace with yours)
│   │   └── models_test.go   # JSON marshaling tests
│   ├── server/              # MCP server implementation
│   │   ├── server.go        # JSON-RPC server, request routing
│   │   ├── server_test.go   # Protocol tests
│   │   └── types.go         # JSON-RPC request/response types
│   └── tools/               # MCP tool implementations
│       ├── tool.go          # Tool interface definition
│       ├── helpers.go       # Shared utility functions
│       ├── helpers_test.go  # Helper function tests
│       ├── echo.go          # Example tool
│       └── echo_test.go     # Example tool tests
├── Makefile                 # Build automation
├── CLAUDE.md                # This file
├── README.md                # User-facing documentation
└── SETUP.md                 # Setup instructions
```

This follows the **standard Go project layout**:
- `cmd/` - Main application entry points
- `internal/` - Private packages that cannot be imported by external projects

## Architecture

### MCP Protocol Implementation

The server implements MCP via **JSON-RPC 2.0 over stdio**:

1. **Stdin** → JSON-RPC requests from Claude
2. **Process** → Route to handlers, execute tools
3. **Stdout** → JSON-RPC responses back to Claude

**Key Methods:**
- `initialize` - Handshake, declare capabilities
- `tools/list` - Return available tools and their schemas
- `tools/call` - Execute a specific tool

**Flow:**
```
Claude → stdin → Scanner → JSON unmarshal → handleRequest() → execute tool → JSON marshal → stdout → Claude
```

### HTTP Client (`internal/client/client.go`)

Generic HTTP client for making API requests. Customize for your use case:

**HTTP Methods:**
- `Get(ctx, path)` - GET requests
- `Post(ctx, path, body)` - POST requests with JSON body
- `Put(ctx, path, body)` - PUT requests with JSON body
- `Delete(ctx, path)` - DELETE requests

**Testing Support:**
- `HTTPDoer` interface allows mocking HTTP requests
- `NewWithHTTPClient(baseURL, httpClient)` - Test constructor
- Use `httptest.Server` for testing without real API calls

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
s.tools["tool_name"] = tools.NewToolName(s.client)
```

The server automatically discovers and exposes all registered tools via `tools/list`.

### Type-Safe Models (`internal/models/models.go`)

Replace the placeholder models with your domain-specific types:

```go
type MyResource struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description,omitempty"`
}
```

**Benefits:**
- Compile-time type checking (no `map[string]interface{}`)
- IDE autocomplete support
- Self-documenting code

### Helper Functions (`internal/tools/helpers.go`)

Shared utility functions to eliminate code duplication:

**`doAPIRequest(ctx, client, path)`** - Common HTTP request pattern
**`ParseJSONResponse(body, target)`** - Type-safe JSON parsing

## Development Workflow

### Building

```bash
# Build executable
make build

# Output: dist/go-mcp-server
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

Follow this pattern:

### 1. Create the tool file in `internal/tools/`

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/yourusername/go-mcp-server/internal/client"
)

type MyTool struct {
    client *client.Client
}

func NewMyTool(c *client.Client) *MyTool {
    return &MyTool{client: c}
}

func (t *MyTool) Description() string {
    return "Brief description of what this tool does"
}

func (t *MyTool) InputSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "paramName": map[string]interface{}{
                "type":        "string",
                "description": "Parameter description",
            },
        },
        "required": []string{"paramName"},
    }
}

func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    // 1. Parse arguments
    var params struct {
        ParamName string `json:"paramName"`
    }
    if err := json.Unmarshal(args, &params); err != nil {
        return "", fmt.Errorf("failed to parse arguments: %w", err)
    }

    // 2. Validate input
    if params.ParamName == "" {
        return "", fmt.Errorf("paramName is required")
    }

    // 3. Make API request (if needed)
    body, err := doAPIRequest(ctx, t.client, "/api/endpoint")
    if err != nil {
        return "", fmt.Errorf("API request failed: %w", err)
    }

    // 4. Parse response
    var result YourModel
    if err := ParseJSONResponse(body, &result); err != nil {
        return "", fmt.Errorf("failed to parse response: %w", err)
    }

    // 5. Format and return result
    return fmt.Sprintf("Result: %v", result), nil
}
```

### 2. Register in `internal/server/server.go`

Add to `registerTools()`:
```go
s.tools["my_tool"] = tools.NewMyTool(s.client)
```

### 3. Write tests for the tool

Create `internal/tools/my_tool_test.go`:

```go
package tools

import (
    "context"
    "encoding/json"
    "testing"
    "github.com/yourusername/go-mcp-server/internal/client"
)

func TestMyTool_Execute(t *testing.T) {
    c := client.New("http://localhost")
    tool := NewMyTool(c)

    tests := []struct {
        name      string
        args      map[string]interface{}
        expectErr bool
    }{
        {
            name: "valid input",
            args: map[string]interface{}{
                "paramName": "test",
            },
            expectErr: false,
        },
        {
            name: "empty param",
            args: map[string]interface{}{
                "paramName": "",
            },
            expectErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            argsJSON, _ := json.Marshal(tt.args)
            _, err := tool.Execute(context.Background(), argsJSON)

            if tt.expectErr && err == nil {
                t.Error("Expected error but got nil")
            }
            if !tt.expectErr && err != nil {
                t.Errorf("Unexpected error: %v", err)
            }
        })
    }
}
```

### 4. Rebuild and test

```bash
# Run tests
go test ./internal/tools -v -run TestMyTool

# Check coverage
make coverage

# Run all checks
make check

# Build binary
make build
```

## Code Quality Standards

### Input Validation
Always validate input before making API calls:
```go
if params.ID <= 0 {
    return "", fmt.Errorf("invalid id: must be positive, got %d", params.ID)
}
```

### Use Helper Functions
Prefer helpers over duplicating code:
```go
// Good: Use doAPIRequest helper
body, err := doAPIRequest(ctx, t.client, path)

// Bad: Duplicate HTTP request logic
resp, err := t.client.Get(ctx, path)
// ... 10 lines of boilerplate
```

### Type Safety
Use models package instead of map[string]interface{}:
```go
// Good: Type-safe
var items []models.Item
ParseJSONResponse(body, &items)

// Bad: Unsafe type assertions
var data []map[string]interface{}
json.Unmarshal(body, &data)
```

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
- Keep it simple - prefer standard library over dependencies
- One tool per file
- Shared logic in helpers.go
- Type definitions in models.go

## Extending the Server

### Current Tools

The template includes one example tool:
- `echo` - Simple example demonstrating tool structure

### Adding Your Tools

1. **Create tool file**: `internal/tools/my_tool.go`
2. **Write tests**: `internal/tools/my_tool_test.go`
3. **Register tool**: Update `registerTools()` in `server.go`
4. **Test**: Run `make check` and test with Claude

## Configuration

**Environment Variables** (customize for your needs):
- `API_URL` - Base URL of your API
- Add authentication credentials as needed

## Response Formatting Guidelines

Tools should return **human-readable formatted strings**, not raw JSON. Claude presents these directly to users.

**Good:**
```go
return "Found 3 items:\n1. Item One\n2. Item Two\n3. Item Three", nil
```

**Avoid:**
```go
return `{"items":[{"name":"Item One"},...]}`, nil
```

## Error Handling

**Always wrap errors with context:**
```go
if err != nil {
    return "", fmt.Errorf("descriptive context: %w", err)
}
```

**Check HTTP status codes:**
```go
if resp.StatusCode != 200 {
    return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
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
- Use `context.Background()` at entry points

### JSON Marshaling
- Use `json.RawMessage` for unknown/dynamic structures
- Type assert cautiously when parsing API responses
- Provide defaults for missing fields

### Resource Cleanup
- Always `defer resp.Body.Close()` after HTTP requests
- Read the body even on errors to allow connection reuse

## Dependencies

Currently zero external dependencies - uses only Go standard library:
- `encoding/json` - JSON marshaling
- `net/http` - HTTP client
- `context` - Context propagation
- `bufio` - Stdio scanning
- `io` - I/O utilities

Keep it this way for simplicity and fast builds.

## Version Information

- MCP Protocol Version: `2024-11-05`
- Server Version: `0.1.0`
- Go Version: 1.21+ required

## Resources

- MCP Specification: https://modelcontextprotocol.io/
- Go Documentation: https://golang.org/doc/

## Customization Checklist

When using this template:

- [ ] Update `go.mod` with your module name
- [ ] Replace `internal/models/` with your domain models
- [ ] Customize `internal/client/` for your API
- [ ] Add authentication if needed
- [ ] Create your tools in `internal/tools/`
- [ ] Update `main.go` environment variables
- [ ] Update `README.md` with your project details
- [ ] Update `SETUP.md` with your configuration
- [ ] Test with `make check`
- [ ] Build with `make build`
- [ ] Configure with Claude (see SETUP.md)
