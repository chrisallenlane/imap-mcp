package tools

import (
	"regexp"
	"strings"

	"golang.org/x/net/html/atom"

	nethtml "golang.org/x/net/html"
)

// blockElements are HTML elements that produce a newline
// before their content in plain-text conversion.
var blockElements = map[atom.Atom]bool{
	atom.P:   true,
	atom.Div: true,
	atom.Tr:  true,
	atom.Li:  true,
	atom.H1:  true,
	atom.H2:  true,
	atom.H3:  true,
	atom.H4:  true,
	atom.H5:  true,
	atom.H6:  true,
}

// skipElements are HTML elements whose entire content
// (including children) should be discarded.
var skipElements = map[atom.Atom]bool{
	atom.Script: true,
	atom.Style:  true,
}

// blankLineRe matches runs of three or more newlines,
// used to collapse excessive blank lines.
var blankLineRe = regexp.MustCompile(`\n{3,}`)

// HTMLToText converts an HTML string to plain text. It
// removes script and style blocks, converts block-level
// elements and <br> tags to newlines, preserves link URLs,
// decodes HTML entities, and collapses excessive blank lines.
func HTMLToText(htmlContent string) string {
	doc, err := nethtml.Parse(
		strings.NewReader(htmlContent),
	)
	if err != nil {
		return "(HTML body could not be parsed)"
	}

	var b strings.Builder
	walkNode(&b, doc)

	out := b.String()
	out = blankLineRe.ReplaceAllString(out, "\n\n")
	out = strings.TrimSpace(out)
	return out
}

// walkNode iteratively traverses the HTML node tree and
// writes plain-text output to the builder. It uses an
// explicit stack to avoid stack overflow on deeply nested
// HTML.
func walkNode(b *strings.Builder, root *nethtml.Node) {
	stack := []*nethtml.Node{root}

	for len(stack) > 0 {
		// Pop the top node.
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch n.Type {
		case nethtml.TextNode:
			b.WriteString(n.Data)

		case nethtml.ElementNode:
			if skipElements[n.DataAtom] {
				continue
			}

			if n.DataAtom == atom.Br {
				b.WriteString("\n")
			}

			if blockElements[n.DataAtom] {
				b.WriteString("\n")
			}

			// Separate table cells with a tab so
			// columns don't run together.
			if n.DataAtom == atom.Td ||
				n.DataAtom == atom.Th {
				if n.PrevSibling != nil {
					b.WriteString("\t")
				}
			}

			// Handle <a href="url">text</a> ->
			// text (url). walkAnchor only recurses
			// into a single anchor's children
			// (shallow), so it stays recursive.
			if n.DataAtom == atom.A {
				walkAnchor(b, n)
				continue
			}

			// Push children in reverse order so the
			// leftmost child is processed first.
			pushChildrenReverse(&stack, n)

		default:
			pushChildrenReverse(&stack, n)
		}
	}
}

// pushChildrenReverse appends n's children to the stack in
// reverse (right-to-left) order so the leftmost child is
// popped first.
func pushChildrenReverse(
	stack *[]*nethtml.Node,
	n *nethtml.Node,
) {
	var children []*nethtml.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		children = append(children, c)
	}
	for i := len(children) - 1; i >= 0; i-- {
		*stack = append(*stack, children[i])
	}
}

// walkAnchor handles <a> elements, rendering them as
// "text (url)" to preserve link information.
func walkAnchor(
	b *strings.Builder,
	n *nethtml.Node,
) {
	href := ""
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			href = attr.Val
			break
		}
	}

	// Collect the anchor's inner text.
	var inner strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNode(&inner, c)
	}
	text := strings.TrimSpace(inner.String())

	if text == "" && href == "" {
		return
	}
	if href == "" || href == text {
		b.WriteString(text)
		return
	}
	if text == "" {
		b.WriteString(href)
		return
	}

	b.WriteString(text)
	b.WriteString(" (")
	b.WriteString(href)
	b.WriteString(")")
}
