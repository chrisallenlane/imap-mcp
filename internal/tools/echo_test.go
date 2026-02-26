package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chrisallenlane/go-mcp-server/internal/client"
)

func TestEcho_Execute(t *testing.T) {
	c := client.New("https://api.example.com")
	tool := NewEcho(c)

	tests := []struct {
		name      string
		message   string
		expectErr bool
		expected  string
	}{
		{
			name:      "valid message",
			message:   "Hello, World!",
			expectErr: false,
			expected:  "Echo: Hello, World!",
		},
		{
			name:      "empty message",
			message:   "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{
				"message": tt.message,
			})

			result, err := tool.Execute(context.Background(), args)

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestEcho_Description(t *testing.T) {
	c := client.New("https://api.example.com")
	tool := NewEcho(c)

	desc := tool.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
}

func TestEcho_InputSchema(t *testing.T) {
	c := client.New("https://api.example.com")
	tool := NewEcho(c)

	schema := tool.InputSchema()
	if schema == nil {
		t.Fatal("InputSchema should not be nil")
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Schema should have properties")
	}

	if _, ok := properties["message"]; !ok {
		t.Error("Schema should have message property")
	}
}
