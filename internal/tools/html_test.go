package tools

import (
	"testing"
)

func TestHTMLToText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"plain text passthrough",
			"Hello, world!",
			"Hello, world!",
		},
		{
			"simple paragraph",
			"<p>Hello</p>",
			"Hello",
		},
		{
			"br converts to newline",
			"Line one<br>Line two",
			"Line one\nLine two",
		},
		{
			"br self-closing",
			"Line one<br/>Line two",
			"Line one\nLine two",
		},
		{
			"paragraph block elements",
			"<p>First</p><p>Second</p>",
			"First\nSecond",
		},
		{
			"div block elements",
			"<div>One</div><div>Two</div>",
			"One\nTwo",
		},
		{
			"list items",
			"<ul><li>A</li><li>B</li><li>C</li></ul>",
			"A\nB\nC",
		},
		{
			"table rows",
			"<table><tr><td>R1</td></tr>" +
				"<tr><td>R2</td></tr></table>",
			"R1\nR2",
		},
		{
			"table cells separated by tab",
			"<table><tr>" +
				"<td>A</td><td>B</td><td>C</td>" +
				"</tr></table>",
			"A\tB\tC",
		},
		{
			"table multi-row multi-col",
			"<table>" +
				"<tr><td>A1</td><td>A2</td></tr>" +
				"<tr><td>B1</td><td>B2</td></tr>" +
				"</table>",
			"A1\tA2\nB1\tB2",
		},
		{
			"script removal",
			"Before<script>alert('xss');</script>After",
			"BeforeAfter",
		},
		{
			"style removal",
			"Before<style>body{color:red}</style>After",
			"BeforeAfter",
		},
		{
			"link preservation",
			`<a href="https://example.com">Click here</a>`,
			"Click here (https://example.com)",
		},
		{
			"link text matches href",
			`<a href="https://example.com">` +
				`https://example.com</a>`,
			"https://example.com",
		},
		{
			"link with no href",
			"<a>just text</a>",
			"just text",
		},
		{
			"link with no text",
			`<a href="https://example.com"></a>`,
			"https://example.com",
		},
		{
			"link with nested elements",
			`<a href="https://example.com">` +
				`<b>bold</b> link</a>`,
			"bold link (https://example.com)",
		},
		{
			"entity decoding",
			"&amp; &lt; &gt; &quot; &apos;",
			"& < > \" '",
		},
		{
			"nbsp entity",
			"Hello&nbsp;world",
			"Hello\u00a0world",
		},
		{
			"numeric entity",
			"&#169; 2025",
			"\u00a9 2025",
		},
		{
			"strips remaining tags",
			"<b>bold</b> and <em>italic</em>",
			"bold and italic",
		},
		{
			"blank line collapsing",
			"<p>A</p><p></p><p></p><p></p><p>B</p>",
			"A\n\nB",
		},
		{
			"heading elements",
			"<h1>Title</h1><p>Body</p>",
			"Title\nBody",
		},
		{
			"empty input",
			"",
			"",
		},
		{
			"whitespace only",
			"   \n  \t  ",
			"",
		},
		{
			"nested elements",
			"<div><p>Nested <b>bold</b> text</p></div>",
			"Nested bold text",
		},
		{
			"complex email-like HTML",
			"<html><body>" +
				"<h1>Newsletter</h1>" +
				"<p>Hello,</p>" +
				"<p>Check out " +
				`<a href="https://example.com">` +
				"our site</a>.</p>" +
				"<div>Thanks!</div>" +
				"</body></html>",
			"Newsletter\nHello,\nCheck out " +
				"our site (https://example.com)." +
				"\nThanks!",
		},
		{
			"malformed HTML",
			"<p>Unclosed <b>tags",
			"Unclosed tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTMLToText(tt.input)
			if got != tt.want {
				t.Errorf(
					"HTMLToText()\n"+
						"got:  %q\n"+
						"want: %q",
					got,
					tt.want,
				)
			}
		})
	}
}
