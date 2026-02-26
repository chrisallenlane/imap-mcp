package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func FuzzServerRun(f *testing.F) {
	// Valid initialize request.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":1,` +
			`"method":"initialize",` +
			`"params":{}}` + "\n",
	))

	// Valid tool call.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":1,` +
			`"method":"tools/call",` +
			`"params":{"name":"mock_tool",` +
			`"arguments":{}}}` + "\n",
	))

	// Malformed JSON.
	f.Add([]byte("this is not json\n"))

	// Empty input.
	f.Add([]byte{})

	// Multiple requests (initialize + tools/list).
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":1,` +
			`"method":"initialize",` +
			`"params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,` +
			`"method":"tools/list"}` + "\n",
	))

	// Very long line (>64 KB).
	f.Add(
		[]byte(strings.Repeat("x", 65536) + "\n"),
	)

	// String ID.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":"abc",` +
			`"method":"initialize",` +
			`"params":{}}` + "\n",
	))

	// Null ID.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":null,` +
			`"method":"initialize",` +
			`"params":{}}` + "\n",
	))

	// Bool ID.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":true,` +
			`"method":"initialize",` +
			`"params":{}}` + "\n",
	))

	// Array ID.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":[1,2,3],` +
			`"method":"initialize",` +
			`"params":{}}` + "\n",
	))

	// Deeply nested params.
	f.Add([]byte(
		`{"jsonrpc":"2.0","id":1,` +
			`"method":"tools/call",` +
			`"params":{"name":"mock_tool",` +
			`"arguments":{"a":{"b":` +
			`{"c":"d"}}}}}` + "\n",
	))

	f.Fuzz(func(t *testing.T, input []byte) {
		s := newTestServer()
		s.tools["mock_tool"] = &mockTool{
			result: "ok",
		}

		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)
		defer cancel()

		reader := bytes.NewReader(input)
		var stdout bytes.Buffer

		// We only care about panics and output
		// validity, not errors.
		_ = s.Run(ctx, reader, &stdout)

		output := strings.TrimSpace(stdout.String())
		if output == "" {
			return
		}

		for _, line := range strings.Split(
			output, "\n",
		) {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var resp JSONRPCResponse
			if err := json.Unmarshal(
				[]byte(line), &resp,
			); err != nil {
				t.Errorf(
					"output is not valid "+
						"JSON-RPC: %q",
					line,
				)
			}

			if resp.JSONRPC != "2.0" {
				t.Errorf(
					"jsonrpc = %q, "+
						"want \"2.0\"",
					resp.JSONRPC,
				)
			}
		}
	})
}
