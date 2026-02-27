# MCP Server Setup Guide

## Quick Start

1. Build: `make build`
2. Configure: `cp config.example.toml config.toml` and edit
3. Register: `make setup`
4. Verify: `claude mcp list`

## Configuration File

The server requires a TOML config file specifying one or more IMAP accounts:

```toml
[accounts.gmail]
host     = "imap.gmail.com"
port     = 993
username = "user@gmail.com"
password = "app-password"
tls      = true
```

All fields (host, port, username, password) are required per account. Set `tls = true` for TLS connections (port 993) or `tls = false` for plaintext (e.g., local IMAP bridges).

### SMTP (Sending)

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

When `smtp_enabled = true`, `smtp_host` and `smtp_port` are required. `smtp_tls` defaults to `"starttls"` if omitted; valid values are `"starttls"`, `"implicit"`, or `"none"`. The three sending tools (`send_message`, `save_draft`, `reply_message`) are only registered when at least one account has `smtp_enabled = true`.

`config.toml` is gitignored because it contains credentials. See `config.example.toml` for a full example.

## Integration

### Claude Code (CLI)

The easiest way to register imap-mcp with Claude Code is via the Makefile:

```bash
make setup
```

This builds the binary, checks for `config.toml`, and runs `claude mcp add` with the correct paths and flags.

**Manual alternative** (if you need custom paths or scope):

```bash
claude mcp add imap-mcp \
  -s user \
  -- /path/to/dist/imap-mcp --config /path/to/config.toml
```

**Scope options:**
- `-s user` - Available in all projects (recommended)
- `-s local` - Private to current project only
- `-s project` - Save to `.mcp.json` for team sharing

**Verify configuration:**
```bash
claude mcp list
# Should show "imap-mcp" in the list

claude mcp get imap-mcp
# Shows configuration details
```

**Within a Claude Code session:**
The MCP tools will be automatically available. Test by asking:
> "What MCP tools are available?"

### Claude Desktop

Add to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
**Linux**: `~/.config/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "imap-mcp": {
      "command": "/path/to/dist/imap-mcp",
      "args": ["--config", "/path/to/config.toml"]
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
# Rebuild and re-register
make setup

# For Claude Desktop: just restart the app
```

## Troubleshooting

### Server not appearing in Claude Code

```bash
# Check if server is registered
claude mcp list

# Check configuration details
claude mcp get imap-mcp

# Try removing and re-adding
claude mcp remove -s user imap-mcp
make setup
```

### Tools not working

1. Verify the config file exists and is valid TOML
2. Check that IMAP credentials are correct
3. Verify the binary has execute permissions: `chmod +x dist/imap-mcp`
4. Test the server directly with stdin/stdout
5. Check Claude logs for errors

### SMTP tools not available

The `send_message`, `save_draft`, and `reply_message` tools only appear when at least one account has `smtp_enabled = true` in the config. Verify:

1. `smtp_enabled = true` is set for the account
2. `smtp_host` and `smtp_port` are provided
3. `smtp_tls` is omitted (defaults to `"starttls"`) or one of `"starttls"`, `"implicit"`, `"none"`
4. Restart Claude Code or Claude Desktop after changing the config

### Binary not found

Make sure you're using the absolute path to the binary. `make setup` handles this automatically. If registering manually:

```bash
# Good
claude mcp add imap-mcp /home/user/imap-mcp/dist/imap-mcp

# Bad (relative path may not work)
claude mcp add imap-mcp ./dist/imap-mcp
```

## Security Notes

- Store IMAP and SMTP credentials in `config.toml`, which is gitignored
- Do not commit `config.toml` to version control
- Consider file permissions on `config.toml` (e.g., `chmod 600 config.toml`)
- The MCP server runs locally and communicates via stdio (no network exposure beyond IMAP/SMTP connections)
