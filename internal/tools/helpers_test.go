package tools

import (
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
