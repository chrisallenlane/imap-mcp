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
	result       string
	err          error
	receivedArgs json.RawMessage
}

func (m *mockTool) Execute(
	_ context.Context,
	args json.RawMessage,
) (string, error) {
	m.receivedArgs = args
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

	if len(tools) < 1 {
		t.Error("Expected at least 1 registered tool")
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

	if !strings.Contains(
		resp.Error.Message,
		"unknown/method",
	) {
		t.Errorf(
			"error message should mention method name, got: %s",
			resp.Error.Message,
		)
	}

	if resp.Result != nil {
		t.Error("Result should be nil for error response")
	}
}

// TestHandleNotification documents the current behavior for MCP
// notifications. The MCP spec defines notifications like
// "notifications/initialized" that clients send after the
// handshake. The server currently treats these as unknown
// methods and returns an error. This test serves as a
// regression test so that any future change to notification
// handling is intentional.
func TestHandleNotification(t *testing.T) {
	s := newTestServer()

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      nil,
		Method:  "notifications/initialized",
	}

	resp := s.handleRequest(context.Background(), req)

	// Current behavior: notifications are treated as unknown
	// methods. If notification support is added later, this
	// test should be updated to reflect the new behavior.
	if resp.Error == nil {
		t.Fatal(
			"Expected error for notification " +
				"(current behavior)",
		)
	}

	if resp.Error.Code != -32601 {
		t.Errorf(
			"Error code = %d, want -32601",
			resp.Error.Code,
		)
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

	if resp.Error.Code != -32603 {
		t.Errorf(
			"Error code = %d, want -32603",
			resp.Error.Code,
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

	if resp.Error.Code != -32603 {
		t.Errorf(
			"Error code = %d, want -32603",
			resp.Error.Code,
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
	mock := &mockTool{
		result: "mock result",
	}
	s.tools["mock_tool"] = mock

	params := map[string]interface{}{
		"name": "mock_tool",
		"arguments": json.RawMessage(
			`{"account":"test","mailbox":"INBOX"}`,
		),
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

	// Verify arguments were forwarded to the tool
	if mock.receivedArgs == nil {
		t.Fatal("mock tool did not receive arguments")
	}

	var received map[string]string
	if err := json.Unmarshal(
		mock.receivedArgs,
		&received,
	); err != nil {
		t.Fatalf(
			"failed to unmarshal received args: %v",
			err,
		)
	}

	if received["account"] != "test" {
		t.Errorf(
			"received account = %q, want %q",
			received["account"],
			"test",
		)
	}

	if received["mailbox"] != "INBOX" {
		t.Errorf(
			"received mailbox = %q, want %q",
			received["mailbox"],
			"INBOX",
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

	if resp.Error.Code != -32603 {
		t.Errorf(
			"Error code = %d, want -32603",
			resp.Error.Code,
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
