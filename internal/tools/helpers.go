package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/chrisallenlane/go-mcp-server/internal/client"
)

// doAPIRequest performs an API request and returns the response body.
// It handles common patterns: making the request, checking status, reading body.
// Includes response body in error messages when status is not OK.
func doAPIRequest(
	ctx context.Context,
	c *client.Client,
	path string,
) ([]byte, error) {
	resp, err := c.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"unexpected status code %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return body, nil
}

// ParseJSONResponse unmarshals a JSON response body into the provided interface.
func ParseJSONResponse(body []byte, v interface{}) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}
