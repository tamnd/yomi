package yomi

import (
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// xhtmlVoid is the set of HTML void elements, the ones an XHTML serialiser must
// self-close (<br/>, <img .../>, <hr/>) rather than leave open as HTML5 allows.
var xhtmlVoid = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// toXHTML parses an HTML fragment and re-serialises it as well-formed XHTML, so
// the Markdown renderer's HTML5 output (with unclosed void elements and the odd
// raw tag the Unsafe renderer passes through) becomes the XML an EPUB content
// document requires. A parse failure returns the fragment unchanged, which keeps
// a pathological body from aborting the whole book.
func toXHTML(fragment string) string {
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := xhtml.ParseFragment(strings.NewReader(fragment), ctx)
	if err != nil {
		return fragment
	}
	var b strings.Builder
	for _, n := range nodes {
		writeXHTML(&b, n)
	}
	return b.String()
}

// writeXHTML serialises one node tree as XHTML.
func writeXHTML(b *strings.Builder, n *xhtml.Node) {
	switch n.Type {
	case xhtml.TextNode:
		b.WriteString(xhtmlEscapeText(n.Data))
		return
	case xhtml.CommentNode:
		b.WriteString("<!--")
		b.WriteString(n.Data)
		b.WriteString("-->")
		return
	case xhtml.DoctypeNode:
		return // the chapter template supplies its own doctype
	case xhtml.ElementNode:
		// fall through
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeXHTML(b, c)
		}
		return
	}

	tag := n.Data
	b.WriteByte('<')
	b.WriteString(tag)
	for _, a := range n.Attr {
		b.WriteByte(' ')
		if a.Namespace != "" {
			b.WriteString(a.Namespace)
			b.WriteByte(':')
		}
		b.WriteString(a.Key)
		b.WriteString(`="`)
		b.WriteString(xhtmlEscapeAttr(a.Val))
		b.WriteByte('"')
	}
	if xhtmlVoid[tag] {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeXHTML(b, c)
	}
	b.WriteString("</")
	b.WriteString(tag)
	b.WriteByte('>')
}

// xhtmlEscapeText escapes a text node for XML: the three characters that would
// otherwise be read as markup.
func xhtmlEscapeText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// xhtmlEscapeAttr escapes an attribute value for an XML double-quoted attribute.
func xhtmlEscapeAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
