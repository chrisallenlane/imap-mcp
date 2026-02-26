// Package client provides a generic HTTP client example.
// This is a template - customize it for your specific API needs.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPDoer interface allows mocking HTTP requests for testing
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client represents a generic HTTP API client.
// Customize this for your specific API needs (e.g., add authentication,
// custom headers, etc.)
type Client struct {
	BaseURL    string
	HTTPClient HTTPDoer
	// Add authentication fields as needed:
	// APIKey     string
	// Token      string
	// etc.
}

// New creates a new HTTP client with default settings
func New(baseURL string) *Client {
	return NewWithHTTPClient(baseURL, &http.Client{
		Timeout: 30 * time.Second,
	})
}

// NewWithHTTPClient creates a new client with a custom HTTP client
// (useful for testing).
func NewWithHTTPClient(baseURL string, httpClient HTTPDoer) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	}
}

// doRequest performs an HTTP request
func (c *Client) doRequest(
	ctx context.Context,
	method, path string,
	body []byte,
) (*http.Response, error) {
	url := c.BaseURL + path

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authentication headers here if needed:
	// req.Header.Set("Authorization", "Bearer "+c.Token)
	// req.Header.Set("X-API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, "GET", path, nil)
}

// Post performs a POST request
func (c *Client) Post(
	ctx context.Context,
	path string,
	body interface{},
) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return c.doRequest(ctx, "POST", path, data)
}

// Put performs a PUT request
func (c *Client) Put(
	ctx context.Context,
	path string,
	body interface{},
) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	return c.doRequest(ctx, "PUT", path, data)
}

// Delete performs a DELETE request
func (c *Client) Delete(
	ctx context.Context,
	path string,
) (*http.Response, error) {
	return c.doRequest(ctx, "DELETE", path, nil)
}
