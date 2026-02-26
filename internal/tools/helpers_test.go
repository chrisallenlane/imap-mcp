package tools

import (
	"testing"

	"github.com/chrisallenlane/go-mcp-server/internal/models"
)

func TestParseJSONResponse(t *testing.T) {
	tests := []struct {
		name      string
		jsonData  string
		expectErr bool
	}{
		{
			name:      "valid JSON",
			jsonData:  `{"id": 1, "name": "Test Item"}`,
			expectErr: false,
		},
		{
			name:      "invalid JSON",
			jsonData:  `{invalid json}`,
			expectErr: true,
		},
		{
			name:      "empty JSON",
			jsonData:  ``,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result models.Item
			err := ParseJSONResponse([]byte(tt.jsonData), &result)

			if tt.expectErr && err == nil {
				t.Error("ParseJSONResponse() expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("ParseJSONResponse() unexpected error: %v", err)
			}

			// For valid JSON, verify it was parsed correctly
			if !tt.expectErr && err == nil {
				if result.ID != 1 || result.Name != "Test Item" {
					t.Errorf(
						"ParseJSONResponse() parsed incorrectly: %+v",
						result,
					)
				}
			}
		})
	}
}
