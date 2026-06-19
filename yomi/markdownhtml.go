package yomi

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// markdown is the shared GitHub-Flavored Markdown renderer used to turn a
// stored page back into HTML for a ZIM or EPUB. Unsafe is on so the inline HTML
// yomi already emits (data-URI images, the odd raw tag) survives the round trip;
// the input is content yomi extracted itself and the output is read offline.
// Auto heading IDs give every heading a stable anchor, so a same-page link to a
// section resolves in the packed book instead of dangling.
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// renderMarkdown converts a Markdown body to an HTML fragment.
func renderMarkdown(md string) (string, error) {
	var buf bytes.Buffer
	if err := markdown.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// zimStyle is a small readable stylesheet inlined into every packed page, so the
// archive looks like an article and not raw HTML without depending on any
// external file inside the ZIM.
const zimStyle = `<style>
:root { color-scheme: light dark; }
body { margin: 0; background: Canvas; color: CanvasText;
  font: 18px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; }
main { max-width: 42rem; margin: 0 auto; padding: 2.5rem 1.25rem 4rem; }
h1, h2, h3 { line-height: 1.25; }
a { color: #2563eb; }
img { max-width: 100%; height: auto; }
pre { overflow-x: auto; padding: 1rem; background: rgba(127,127,127,.12); border-radius: .5rem; }
code { font-size: .92em; }
pre code { font-size: inherit; }
table { border-collapse: collapse; }
th, td { border: 1px solid rgba(127,127,127,.4); padding: .4rem .6rem; }
hr { border: none; border-top: 1px solid rgba(127,127,127,.3); margin: 2.5rem 0; }
.yomi-meta { color: #6b7280; font-size: .9rem; margin-top: -.5rem; }
.yomi-nav { font-size: .9rem; }
ul.yomi-toc { list-style: none; padding: 0; }
ul.yomi-toc li { padding: .35rem 0; border-bottom: 1px solid rgba(127,127,127,.15); }
ul.yomi-toc .min { color: #6b7280; font-size: .85rem; }
</style>`

// pageHTML wraps a rendered Markdown body in a complete, self-contained HTML
// document: the title, a small source-and-byline line, the article, and a link
// back to the contents page. indexHref is the relative path to that contents
// page from this entry.
func pageHTML(p *Page, body, indexHref string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html")
	if p.Lang != "" {
		fmt.Fprintf(&b, " lang=%q", p.Lang)
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(titleOr(p)))
	b.WriteString(zimStyle)
	b.WriteString("\n</head>\n<body>\n<main>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(titleOr(p)))
	b.WriteString("<p class=\"yomi-meta\">")
	if p.Byline != "" {
		fmt.Fprintf(&b, "%s &middot; ", html.EscapeString(p.Byline))
	}
	fmt.Fprintf(&b, "<a href=%q>source</a>", html.EscapeString(p.URL))
	if p.ReadingMin > 0 {
		fmt.Fprintf(&b, " &middot; %d min read", p.ReadingMin)
	}
	b.WriteString("</p>\n")
	b.WriteString(body)
	// A pack entry links back to the archive's contents page; a standalone
	// document (read -f html) has none, so an empty indexHref drops the footer.
	if indexHref != "" {
		fmt.Fprintf(&b, "\n<hr>\n<p class=\"yomi-nav\"><a href=%q>Contents</a></p>\n", html.EscapeString(indexHref))
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

// StandaloneHTML renders a Page as a complete, self-contained HTML document: the
// readable stylesheet, the title and a small source line, then the article body.
// It is the html output of `read -f html`, the single-page sibling of the
// per-page HTML the pack formats render, without the offline contents-page link
// a ZIM entry carries.
func StandaloneHTML(p *Page) (string, error) {
	body, err := renderMarkdown(p.Markdown)
	if err != nil {
		return "", err
	}
	return pageHTML(p, body, ""), nil
}

// indexHTML builds the contents landing page that a reader opens first: the
// archive title and a list of every page linking to its entry, with the reading
// time alongside. hrefs[i] is the link target for pages[i].
func indexHTML(title, host string, pages []*Page, hrefs []string) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString(zimStyle)
	b.WriteString("\n</head>\n<body>\n<main>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<p class=\"yomi-meta\">%d pages packed from %s by yomi.</p>\n", len(pages), html.EscapeString(host))
	b.WriteString("<ul class=\"yomi-toc\">\n")
	for i, p := range pages {
		fmt.Fprintf(&b, "<li><a href=%q>%s</a>", html.EscapeString(hrefs[i]), html.EscapeString(titleOr(p)))
		if p.ReadingMin > 0 {
			fmt.Fprintf(&b, " <span class=\"min\">%d min</span>", p.ReadingMin)
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n</main>\n</body>\n</html>\n")
	return b.String()
}

// rewriteLinks rewires the <a href> targets in an HTML fragment through resolve.
// When resolve reports a hit, the href becomes the returned in-archive path;
// otherwise the link is left exactly as it was, so an external URL still points
// at the live web. A parse failure returns the fragment untouched.
func rewriteLinks(fragment string, resolve func(href string) (string, bool)) string {
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := xhtml.ParseFragment(strings.NewReader(fragment), ctx)
	if err != nil {
		return fragment
	}
	for _, n := range nodes {
		rewriteNode(n, resolve)
	}
	var buf bytes.Buffer
	for _, n := range nodes {
		if err := xhtml.Render(&buf, n); err != nil {
			return fragment
		}
	}
	return buf.String()
}

func rewriteNode(n *xhtml.Node, resolve func(string) (string, bool)) {
	if n.Type == xhtml.ElementNode && n.DataAtom == atom.A {
		for i, a := range n.Attr {
			if a.Key == "href" {
				if t, ok := resolve(a.Val); ok {
					n.Attr[i].Val = t
				}
				break
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		rewriteNode(c, resolve)
	}
}
