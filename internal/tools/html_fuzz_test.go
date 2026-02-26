package tools

import (
	"strings"
	"testing"
)

func FuzzHTMLToText(f *testing.F) {
	// Seed corpus from existing test cases.
	f.Add("Hello, world!")
	f.Add("<p>Hello</p>")
	f.Add("Line one<br>Line two")
	f.Add("<p>First</p><p>Second</p>")
	f.Add(
		"<ul><li>A</li><li>B</li><li>C</li></ul>",
	)
	f.Add(
		"Before<script>alert('xss');</script>After",
	)
	f.Add(
		`<a href="https://example.com">Click</a>`,
	)
	f.Add("&amp; &lt; &gt; &quot; &apos;")
	f.Add("<b>bold</b> and <em>italic</em>")
	f.Add(
		"<html><body>" +
			"<h1>Newsletter</h1>" +
			"<p>Hello,</p>" +
			"<p>Check out " +
			`<a href="https://example.com">` +
			"our site</a>.</p>" +
			"<div>Thanks!</div>" +
			"</body></html>",
	)

	// Empty input.
	f.Add("")

	// Whitespace only.
	f.Add("   \n  \t  ")

	// Malformed HTML.
	f.Add("<p>Unclosed <b>tags")
	f.Add("<<<>>>")
	f.Add("<div><div><div>")

	// Deeply nested tags (100 levels).
	f.Add(
		strings.Repeat("<div>", 100) +
			"deep" +
			strings.Repeat("</div>", 100),
	)

	// Table with cells.
	f.Add(
		"<table><tr><td>A</td><td>B</td></tr>" +
			"<tr><td>C</td><td>D</td></tr></table>",
	)

	f.Fuzz(func(t *testing.T, input string) {
		// HTMLToText must never panic.
		result := HTMLToText(input)

		// Output must not contain excessive blank
		// lines (3+ consecutive newlines).
		if strings.Contains(result, "\n\n\n") {
			t.Errorf(
				"output contains 3+ consecutive "+
					"newlines:\n%q",
				result,
			)
		}
	})
}
