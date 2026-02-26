package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chrisallenlane/imap-mcp/internal/config"
	imapmanager "github.com/chrisallenlane/imap-mcp/internal/imap"
)

// mockTool implements tools.Tool for testing purposes.
type mockTool struct {
	result string
	err    error
}

func (m *mockTool) Execute(
	_ context.Context,
	_ json.RawMessage,
) (string, error) {
	return m.result, m.err
}

func (m *mockTool) Description() string {
	return "A mock tool for testing"
}

func (m *mockTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func newTestServer() *Server {
	cfg := &config.Config{
		Accounts: map[string]config.Account{
			"test": {
				Host:     "localhost",
				Port:     993,
				Username: "user",
				Password: "pass",
				TLS:      true,
			},
		},
	}
	mgr := imapmanager.NewManager(cfg)
	return New(mgr)
}

func TestHandleInitialize(t *testing.T) {
	s := newTestServer()

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
		t.Errorf(
			"Server name = %s, want %s",
			serverInfo["name"],
			ServerName,
		)
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
	s := newTestServer()

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

	// list_accounts, list_mailboxes, list_messages, and
	// get_message are registered by default
	if len(tools) != 4 {
		t.Errorf("Expected 4 tools, got %d", len(tools))
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	s := newTestServer()

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
	s := newTestServer()

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

	if !strings.Contains(resp.Error.Message, "tool not found") {
		t.Errorf(
			"Error message should mention 'tool not found', got: %s",
			resp.Error.Message,
		)
	}
}

func TestHandleCallTool_MalformedParams(t *testing.T) {
	s := newTestServer()

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

	if !strings.Contains(
		resp.Error.Message,
		"failed to parse tool call params",
	) {
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
		t.Errorf(
			"error code = %v, want -32600",
			errorObj["code"],
		)
	}

	if errorObj["message"] != "Invalid Request" {
		t.Errorf(
			"error message = %v, want Invalid Request",
			errorObj["message"],
		)
	}
}

func TestRun_Initialize(t *testing.T) {
	s := newTestServer()

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	err := s.Run(context.Background(), stdin, &stdout)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf(
			"failed to unmarshal response: %v",
			err,
		)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %s, want 2.0", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}
}

func TestRun_MultipleRequests(t *testing.T) {
	s := newTestServer()

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` +
		"\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` +
		"\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	err := s.Run(context.Background(), stdin, &stdout)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	// Each response is a newline-delimited JSON object
	lines := strings.Split(
		strings.TrimSpace(stdout.String()),
		"\n",
	)
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d", len(lines))
	}

	var resp1 JSONRPCResponse
	if err := json.Unmarshal(
		[]byte(lines[0]),
		&resp1,
	); err != nil {
		t.Fatalf("failed to unmarshal response 1: %v", err)
	}

	var resp2 JSONRPCResponse
	if err := json.Unmarshal(
		[]byte(lines[1]),
		&resp2,
	); err != nil {
		t.Fatalf("failed to unmarshal response 2: %v", err)
	}

	if resp1.Error != nil {
		t.Errorf("response 1 unexpected error: %+v", resp1.Error)
	}
	if resp2.Error != nil {
		t.Errorf("response 2 unexpected error: %+v", resp2.Error)
	}
}

func TestRun_MalformedJSON(t *testing.T) {
	s := newTestServer()

	input := "this is not json\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	err := s.Run(context.Background(), stdin, &stdout)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(
		stdout.Bytes(),
		&resp,
	); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response for malformed JSON")
	}
	if resp.Error.Code != -32700 {
		t.Errorf(
			"error code = %d, want -32700",
			resp.Error.Code,
		)
	}
}

func TestRun_EmptyInput(t *testing.T) {
	s := newTestServer()

	stdin := strings.NewReader("")
	var stdout bytes.Buffer

	err := s.Run(context.Background(), stdin, &stdout)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	if stdout.Len() != 0 {
		t.Errorf(
			"expected no output for empty input, got: %s",
			stdout.String(),
		)
	}
}

func TestHandleCallTool_Success(t *testing.T) {
	s := newTestServer()
	s.tools["mock_tool"] = &mockTool{
		result: "mock result",
	}

	params := map[string]interface{}{
		"name":      "mock_tool",
		"arguments": json.RawMessage(`{}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      6,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("Result should not be nil")
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	content, ok := result["content"].([]map[string]interface{})
	if !ok {
		t.Fatal("content should be a slice of maps")
	}

	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}

	if content[0]["type"] != "text" {
		t.Errorf(
			"content type = %v, want text",
			content[0]["type"],
		)
	}

	if content[0]["text"] != "mock result" {
		t.Errorf(
			"content text = %v, want mock result",
			content[0]["text"],
		)
	}
}

func TestHandleCallTool_ToolError(t *testing.T) {
	s := newTestServer()
	s.tools["failing_tool"] = &mockTool{
		err: context.DeadlineExceeded,
	}

	params := map[string]interface{}{
		"name":      "failing_tool",
		"arguments": json.RawMessage(`{}`),
	}
	paramsJSON, _ := json.Marshal(params)

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for tool that returns error")
	}

	if !strings.Contains(
		resp.Error.Message,
		"tool execution failed",
	) {
		t.Errorf(
			"error should mention tool execution failed, got: %s",
			resp.Error.Message,
		)
	}
}

func TestHandleListTools_WithRegisteredTools(t *testing.T) {
	s := newTestServer()
	s.tools["mock_tool"] = &mockTool{
		result: "test",
	}

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      8,
		Method:  "tools/list",
	}

	resp := s.handleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	toolsList, ok := result["tools"].([]map[string]interface{})
	if !ok {
		t.Fatal("tools should be a slice")
	}

	// list_accounts + list_mailboxes + list_messages +
	// get_message auto-registered, plus mock_tool
	if len(toolsList) != 5 {
		t.Fatalf(
			"expected 5 tools, got %d",
			len(toolsList),
		)
	}

	// Verify mock_tool is present
	found := false
	for _, tool := range toolsList {
		if tool["name"] == "mock_tool" {
			found = true
			if tool["description"] != "A mock tool for testing" {
				t.Errorf(
					"tool description = %v, "+
						"want A mock tool for testing",
					tool["description"],
				)
			}
			if tool["inputSchema"] == nil {
				t.Error(
					"tool inputSchema should not be nil",
				)
			}
		}
	}
	if !found {
		t.Error("mock_tool not found in tools list")
	}
}

func TestRun_ToolCallViaPipe(t *testing.T) {
	s := newTestServer()
	s.tools["mock_tool"] = &mockTool{
		result: "piped result",
	}

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"mock_tool","arguments":{}}}` +
		"\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	err := s.Run(context.Background(), stdin, &stdout)
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(
		stdout.Bytes(),
		&resp,
	); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	// Verify the response contains our mock result
	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	if !strings.Contains(string(data), "piped result") {
		t.Errorf(
			"response should contain 'piped result', got: %s",
			string(data),
		)
	}
}
