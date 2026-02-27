package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf(
			"result does not contain %q\ngot:\n%s",
			substr,
			s,
		)
	}
}

func assertNotContains(
	t *testing.T,
	s, substr string,
) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf(
			"result should not contain %q\ngot:\n%s",
			substr,
			s,
		)
	}
}

// assertSchema validates that a tool's InputSchema has the
// expected structure, properties, and required fields.
func assertSchema(
	t *testing.T,
	schema map[string]interface{},
	expectedProps, expectedRequired []string,
) {
	t.Helper()
	if schema["type"] != "object" {
		t.Errorf(
			"schema type = %v, want object",
			schema["type"],
		)
	}

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}

	for _, p := range expectedProps {
		if _, ok := props[p]; !ok {
			t.Errorf(
				"schema should have %q property",
				p,
			)
		}
	}

	if len(expectedRequired) == 0 {
		return
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required should be a []string")
	}

	requiredSet := map[string]bool{}
	for _, r := range required {
		requiredSet[r] = true
	}
	for _, r := range expectedRequired {
		if !requiredSet[r] {
			t.Errorf(
				"required should contain %q, "+
					"got %v",
				r,
				required,
			)
		}
	}
}

// assertInvalidJSONError verifies that a tool returns a
// parse error when given malformed JSON input.
func assertInvalidJSONError(t *testing.T, tool Tool) {
	t.Helper()
	_, err := tool.Execute(
		context.Background(),
		json.RawMessage(`{invalid`),
	)
	if err == nil {
		t.Fatal(
			"Execute() expected error for " +
				"invalid JSON",
		)
	}
	assertContains(t, err.Error(), "parse")
}
