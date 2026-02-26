package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chrisallenlane/go-mcp-server/internal/client"
)

func TestHandleInitialize(t *testing.T) {
	c := client.New("http://localhost")
	s := New(c)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.JSONRPC != "2.0" {
		t.Errorf("Response JSONRPC = %s, want 2.0", resp.JSONRPC)
	}

	if resp.ID != 1 {
		t.Errorf("Response ID = %v, want 1", resp.ID)
	}

	if resp.Error != nil {
		t.Errorf("Unexpected error: %+v", resp.Error)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	// Verify result structure
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	if result["protocolVersion"] != MCPProtocolVersion {
		t.Errorf(
			"Protocol version = %v, want %s",
			result["protocolVersion"],
			MCPProtocolVersion,
		)
	}

	serverInfo, ok := result["serverInfo"].(map[string]string)
	if !ok {
		t.Fatal("serverInfo should be a map")
	}

	if serverInfo["name"] != ServerName {
		t.Errorf("Server name = %s, want %s", serverInfo["name"], ServerName)
	}

	if serverInfo["version"] != ServerVersion {
		t.Errorf(
			"Server version = %s, want %s",
			serverInfo["version"],
			ServerVersion,
		)
	}
}

func TestHandleListTools(t *testing.T) {
	c := client.New("http://localhost")
	s := New(c)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Errorf("Unexpected error: %+v", resp.Error)
	}

	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	tools, ok := result["tools"].([]map[string]interface{})
	if !ok {
		t.Fatal("tools should be a slice")
	}

	// Verify we have tools registered
	if len(tools) == 0 {
		t.Error("Expected at least one tool to be registered")
	}

	// Verify tool structure
	for _, tool := range tools {
		if _, ok := tool["name"]; !ok {
			t.Error("Tool should have a name")
		}
		if _, ok := tool["description"]; !ok {
			t.Error("Tool should have a description")
		}
		if _, ok := tool["inputSchema"]; !ok {
			t.Error("Tool should have an inputSchema")
		}
	}

	// Verify the echo tool exists
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool["name"].(string)
	}

	expectedTool := "echo"
	found := false
	for _, name := range toolNames {
		if name == expectedTool {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected tool %s not found", expectedTool)
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	c := client.New("http://localhost")
	s := New(c)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "unknown/method",
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("Expected error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("Error code = %d, want -32601", resp.Error.Code)
	}

	if resp.Error.Message != "Method not found: unknown/method" {
		t.Errorf("Error message = %s", resp.Error.Message)
	}

	if resp.Result != nil {
		t.Error("Result should be nil for error response")
	}
}

func TestHandleCallTool_InvalidTool(t *testing.T) {
	c := client.New("http://localhost")
	s := New(c)

	params := map[string]interface{}{
		"name":      "nonexistent_tool",
		"arguments": json.RawMessage(`{}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("Expected error for nonexistent tool")
	}

	if !containsString(resp.Error.Message, "tool not found") {
		t.Errorf(
			"Error message should mention 'tool not found', got: %s",
			resp.Error.Message,
		)
	}
}

func TestHandleCallTool_MalformedParams(t *testing.T) {
	c := client.New("http://localhost")
	s := New(c)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  json.RawMessage(`{invalid json}`),
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("Expected error for malformed params")
	}

	if !containsString(resp.Error.Message, "failed to parse tool call params") {
		t.Errorf(
			"Error message should mention parsing failure, got: %s",
			resp.Error.Message,
		)
	}
}

func TestJSONRPCRequest_Unmarshal(t *testing.T) {
	jsonData := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`

	var req JSONRPCRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", req.JSONRPC)
	}

	if req.Method != "initialize" {
		t.Errorf("Method = %s, want initialize", req.Method)
	}
}

func TestJSONRPCResponse_Marshal(t *testing.T) {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]string{"status": "ok"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", decoded["jsonrpc"])
	}
}

func TestJSONRPCError_Marshal(t *testing.T) {
	resp := &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Error: &JSONRPCError{
			Code:    -32600,
			Message: "Invalid Request",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	errorObj, ok := decoded["error"].(map[string]interface{})
	if !ok {
		t.Fatal("error should be an object")
	}

	if errorObj["code"].(float64) != -32600 {
		t.Errorf("error code = %v, want -32600", errorObj["code"])
	}

	if errorObj["message"] != "Invalid Request" {
		t.Errorf(
			"error message = %v, want Invalid Request",
			errorObj["message"],
		)
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
