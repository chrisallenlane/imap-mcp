// Package models defines the data structures for your MCP server.
// These are placeholder examples - customize for your specific API.
package models

// Item represents a generic item in your system.
// Replace this with your actual domain model.
type Item struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

// User represents a generic user.
// Replace this with your actual user model.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
}

// Response is an example generic API response wrapper.
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}
