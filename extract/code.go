package extract

import (
	"strings"

	"golang.org/x/net/html"
)

// normalizeCode rewrites every code block so its language hint sits on the
// <code> element as class="language-X", the form html-to-markdown reads when it
// picks a fenced-code info string. The hint is recovered from the conventions
// syntax highlighters use: a language- or lang- class, a GitHub
// highlight-source- class on a wrapper, or a data-language attribute, looking at
// the <pre>, its <code>, and a few ancestors. Once the language is captured the
// pass strips every class from the subtree so no styling noise reaches the
// Markdown, then reapplies the clean language class. It also restores the line
// breaks inside code whose highlighter wrapped each line in an element without a
// literal newline.
func normalizeCode(root *html.Node) {
	lang := map[*html.Node]string{}
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "pre" {
			if l := detectLang(n); l != "" {
				target := firstCode(n)
				if target == nil {
					target = n
				}
				lang[target] = l
			}
			restoreCodeLines(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)

	stripClasses(root)
	for n, l := range lang {
		setClass(n, "language-"+l)
	}
}

// detectLang looks for a language hint on the pre, its code child, and up to
// three ancestors, returning the first it finds.
func detectLang(pre *html.Node) string {
	nodes := []*html.Node{pre}
	if c := firstCode(pre); c != nil {
		nodes = append(nodes, c)
	}
	for a, i := pre.Parent, 0; a != nil && i < 3; a, i = a.Parent, i+1 {
		nodes = append(nodes, a)
	}
	for _, n := range nodes {
		if l := langFromAttrs(n); l != "" {
			return l
		}
	}
	return ""
}

// langFromAttrs pulls a language token from a node's data-language attributes or
// its class, recognising the language-, lang-, and highlight-source- prefixes.
func langFromAttrs(n *html.Node) string {
	for _, key := range []string{"data-language", "data-lang", "data-code-language"} {
		if v := cleanLang(attr(n, key)); v != "" {
			return v
		}
	}
	// highlight-source- (GitHub) is checked before highlight- (Sphinx/Pygments)
	// so the longer, more specific prefix wins.
	toks := strings.Fields(attr(n, "class"))
	for i, tok := range toks {
		// MDN's Prism marks code with two tokens: "brush:" then the language.
		if tok == "brush:" && i+1 < len(toks) {
			if v := cleanLang(toks[i+1]); v != "" {
				return v
			}
		}
		for _, p := range []string{"language-", "lang-", "highlight-source-", "highlight-"} {
			if strings.HasPrefix(tok, p) {
				if v := cleanLang(strings.TrimPrefix(tok, p)); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

// cleanLang normalises a candidate language token, rejecting empty, oversized,
// or non-identifier values so a stray class never becomes a bogus info string.
// "default", "none", and "auto" are highlighter placeholders, not languages, so
// they collapse to a plain fence.
func cleanLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "", "none", "default", "auto", "plaintext", "text":
		return ""
	}
	if len(s) > 20 {
		return ""
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9',
			r == '+', r == '#', r == '-', r == '.', r == '_':
		default:
			return ""
		}
	}
	if alias, ok := langAlias[s]; ok {
		return alias
	}
	return s
}

// langAlias maps highlighter-specific lexer names to the conventional Markdown
// info string for the same language.
var langAlias = map[string]string{
	"python3":    "python",
	"py":         "python",
	"pycon":      "python",
	"golang":     "go",
	"js":         "javascript",
	"ts":         "typescript",
	"sh":         "bash",
	"shell":      "bash",
	"console":    "bash",
	"jsonc":      "json",
	"yml":        "yaml",
	"html+jinja": "html",
}

// firstCode returns the first <code> descendant of pre, or nil.
func firstCode(pre *html.Node) *html.Node {
	var found *html.Node
	var rec func(*html.Node)
	rec = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "code" {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	for c := pre.FirstChild; c != nil; c = c.NextSibling {
		rec(c)
	}
	return found
}

// restoreCodeLines inserts a newline between consecutive line wrappers when a
// highlighter laid each source line out as its own element and relied on CSS,
// not a literal newline, to break them. It runs only when the block has no
// newline of its own, so well-formed code is never touched.
func restoreCodeLines(pre *html.Node) {
	if strings.Contains(text(pre), "\n") {
		return
	}
	var lines []*html.Node
	var rec func(*html.Node)
	rec = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "span" || n.Data == "div") && hasClassToken(n, "line") {
			lines = append(lines, n)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(pre)
	if len(lines) < 2 {
		return
	}
	for _, ln := range lines[:len(lines)-1] {
		if ln.Parent == nil {
			continue
		}
		ln.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: "\n"}, ln.NextSibling)
	}
}

// hasClassToken reports whether n's class attribute contains the whitespace
// delimited token tok.
func hasClassToken(n *html.Node, tok string) bool {
	for _, t := range strings.Fields(attr(n, "class")) {
		if t == tok {
			return true
		}
	}
	return false
}

// stripClasses removes the class attribute from every element in the subtree.
func stripClasses(root *html.Node) {
	if root.Type == html.ElementNode {
		kept := root.Attr[:0]
		for _, a := range root.Attr {
			if a.Key != "class" {
				kept = append(kept, a)
			}
		}
		root.Attr = kept
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		stripClasses(c)
	}
}

// setClass sets n's class attribute, replacing any existing value.
func setClass(n *html.Node, val string) {
	for i, a := range n.Attr {
		if a.Key == "class" {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: val})
}
