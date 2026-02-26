// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"encoding/json"
)

// Tool represents an MCP tool
type Tool interface {
	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args json.RawMessage) (string, error)

	// Description returns a description of what the tool does
	Description() string

	// InputSchema returns the JSON schema for the tool's input
	InputSchema() map[string]interface{}
}
