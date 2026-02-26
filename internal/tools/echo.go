// Package tools provides MCP tool implementations.
package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chrisallenlane/go-mcp-server/internal/client"
)

// Echo is a simple example tool that echoes back a message.
// This demonstrates the basic structure of an MCP tool.
type Echo struct {
	client *client.Client
}

// NewEcho creates a new Echo tool instance
func NewEcho(c *client.Client) *Echo {
	return &Echo{client: c}
}

// Description returns a description of what this tool does
func (t *Echo) Description() string {
	return "Echoes back the provided message. A simple example tool."
}

// InputSchema returns the JSON schema for the tool's input parameters
func (t *Echo) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "The message to echo back",
			},
		},
		"required": []string{"message"},
	}
}

// Execute runs the tool with the provided arguments
func (t *Echo) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	// Parse input arguments
	var params struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Validate input
	if params.Message == "" {
		return "", fmt.Errorf("message cannot be empty")
	}

	// Return the echoed message
	return fmt.Sprintf("Echo: %s", params.Message), nil
}
