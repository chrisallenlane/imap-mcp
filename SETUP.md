# MCP Server Setup Guide

## Quick Start

1. **Build the server**:
   ```bash
   cd ~/path/to/go-mcp-server
   make build
   # Binary: dist/go-mcp-server
   ```

2. **Set environment variables**:
   ```bash
   export API_URL="https://api.example.com"
   # Add any other required environment variables
   ```

3. **Test the server** (optional):
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"initialize"}' | ./dist/go-mcp-server
   ```

## Configuration

### For Claude Code (CLI)

Use the `claude mcp add` command to configure the server:

```bash
claude mcp add my-server /path/to/dist/go-mcp-server \
  -s user \
  -e API_URL=https://api.example.com
```

**Scope options:**
- `-s user` - Available in all projects (recommended)
- `-s local` - Private to current project only
- `-s project` - Save to `.mcp.json` for team sharing

**Verify configuration:**
```bash
claude mcp list
# Should show "my-server" in the list

claude mcp get my-server
# Shows configuration details
```

**Within a Claude Code session:**
The MCP tools will be automatically available. Test by asking:
> "What MCP tools are available?"

### For Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
**Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "my-mcp-server": {
      "command": "/path/to/dist/go-mcp-server",
      "env": {
        "API_URL": "https://api.example.com"
      }
    }
  }
}
```

**Restart Claude Desktop** after updating the configuration.

## Development Workflow

### Local Development

1. Make changes to code
2. Run `make check` to format, lint, and test
3. Build with `make build`
4. Test with Claude

### Updating the Server

After making changes:

```bash
# Rebuild
make build

# For Claude Code: remove and re-add
claude mcp remove my-server
claude mcp add my-server /path/to/dist/go-mcp-server \
  -s user \
  -e API_URL=https://api.example.com

# For Claude Desktop: just restart the app
```

## Troubleshooting

### Server not appearing in Claude Code

```bash
# Check if server is registered
claude mcp list

# Check configuration details
claude mcp get my-server

# Try removing and re-adding
claude mcp remove my-server
claude mcp add my-server /path/to/dist/go-mcp-server -s user
```

### Tools not working

1. Check environment variables are set correctly
2. Verify the binary has execute permissions: `chmod +x dist/go-mcp-server`
3. Test the server directly with stdin/stdout
4. Check Claude logs for errors

### Binary not found

Make sure you're using the absolute path to the binary:

```bash
# Good
claude mcp add my-server /home/user/go-mcp-server/dist/go-mcp-server

# Bad (relative path may not work)
claude mcp add my-server ./dist/go-mcp-server
```

## Environment Variables

Common environment variables you might need:

```bash
# API Configuration
API_URL=https://api.example.com
API_KEY=your-api-key

# Authentication
USERNAME=your-username
PASSWORD=your-password

# Optional settings
DEBUG=true
TIMEOUT=30
```

## Security Notes

- Store sensitive credentials in environment variables, not in code
- Use `claude mcp add` with env flags rather than hardcoding secrets
- Consider using a credential manager for production use
- The MCP server runs locally and communicates via stdio (no network exposure)

## Testing Your Configuration

After setup, test your MCP server:

1. **Start a Claude session**
2. **Ask Claude**: "What MCP tools are available?"
3. **Try your tools**: "Use the echo tool to say hello"

## Next Steps

- Customize `internal/tools/` with your own tools
- Update `internal/models/` for your data structures
- Add authentication to `internal/client/` if needed
- See `CLAUDE.md` for development guidance
