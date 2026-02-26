# go-mcp-server

A template for building [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers in Go.

This template provides a complete, production-ready foundation for creating MCP servers that integrate external services with Claude and other AI assistants.

## Features

- **Complete MCP Implementation**: Full JSON-RPC 2.0 over stdio
- **Type-Safe Models**: Structured Go types with JSON marshaling
- **HTTP Client Example**: Generic HTTP client with customizable authentication
- **Tool System**: Clean interface for adding new capabilities
- **Testing Infrastructure**: Comprehensive test examples
- **Code Quality Tools**: Formatting, linting, and vetting built-in

## Project Structure

```
go-mcp-server/
├── cmd/
│   └── go-mcp-server/    # Main application entry point
├── internal/
│   ├── client/           # HTTP client (customize for your API)
│   ├── models/           # Data structures (replace with your models)
│   ├── server/           # MCP JSON-RPC server (keep as-is)
│   └── tools/            # Tool implementations (add yours here)
├── Makefile              # Build automation
└── README.md             # This file
```

## Getting Started

### Prerequisites

- Go 1.21 or later
- Make (optional, but recommended)

### Installation

```bash
# Clone this template
git clone https://github.com/yourusername/go-mcp-server.git
cd go-mcp-server

# Install dependencies and build
make install
```

### Configuration

Set environment variables for your API:

```bash
export API_URL="https://api.example.com"
# Add any other environment variables your server needs
```

### Running

The MCP server communicates via stdin/stdout:

```bash
# Direct execution
./dist/go-mcp-server

# Or via Claude Desktop (see SETUP.md)
```

## Customizing for Your Use Case

### 1. Update Models (`internal/models/models.go`)

Replace the placeholder models with your domain-specific types:

```go
type MyResource struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    // Add your fields
}
```

### 2. Customize HTTP Client (`internal/client/client.go`)

Add authentication, custom headers, etc.:

```go
// Add your auth fields
type Client struct {
    BaseURL string
    HTTPClient HTTPDoer
    APIKey string  // Add your auth
}

// Update doRequest to include auth
req.Header.Set("Authorization", "Bearer "+c.APIKey)
```

### 3. Create Tools (`internal/tools/`)

Each tool needs:
- Implementation file (e.g., `my_tool.go`)
- Test file (e.g., `my_tool_test.go`)

Example tool structure:

```go
type MyTool struct {
    client *client.Client
}

func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
    // Your implementation
}

func (t *MyTool) Description() string {
    return "What your tool does"
}

func (t *MyTool) InputSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param": map[string]interface{}{
                "type": "string",
                "description": "Parameter description",
            },
        },
        "required": []string{"param"},
    }
}
```

### 4. Register Tools (`internal/server/server.go`)

Add your tools to the `registerTools()` function:

```go
func (s *Server) registerTools() {
    s.tools["echo"] = tools.NewEcho(s.client)
    s.tools["my_tool"] = tools.NewMyTool(s.client)  // Add yours
}
```

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
- **Dependencies**: Minimal - only Go stdlib for production code
- **Error Handling**: Always wrap errors with context
- **Type Safety**: Use structs, not `map[string]interface{}`

## Architecture

### MCP Protocol Flow

```
Claude → stdin → JSON-RPC Request → Tool Execution → JSON-RPC Response → stdout → Claude
```

### Key Components

- **Server** (`internal/server`): Handles JSON-RPC protocol
- **Client** (`internal/client`): Makes HTTP requests to your API
- **Tools** (`internal/tools`): Implements MCP tools
- **Models** (`internal/models`): Type-safe data structures

## License

MIT License - see LICENSE file for details

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check` to ensure code quality
5. Submit a pull request

## Resources

- [MCP Specification](https://modelcontextprotocol.io/)
- [Go Documentation](https://golang.org/doc/)
- [Standard Project Layout](https://github.com/golang-standards/project-layout)

## Next Steps

1. Customize `internal/models/` for your data structures
2. Update `internal/client/` for your API authentication
3. Create tools in `internal/tools/` for your use case
4. Update `cmd/go-mcp-server/main.go` for configuration
5. Test with Claude Desktop (see SETUP.md)

For detailed development guidance, see `CLAUDE.md`.
