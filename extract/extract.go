// Package extract turns a full HTML page into its article: the main-content
// node with the chrome removed, plus the page metadata (title, byline, site
// name, excerpt, language, publish date) and the outbound links.
//
// It runs go-readability for the content node and harvests metadata from the
// document's own tags first, falling back to what readability recovers. The
// content node is sanitised with kage's CleanTree so no script or handler
// survives into the Markdown.
package extract

import (
	"bytes"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/tamnd/kage/sanitize"
	"golang.org/x/net/html"
)

// Link is one outbound hyperlink discovered on a page, resolved to an absolute
// URL.
type Link struct {
	Text string
	URL  string
}

// Article is the extracted form of one HTML page.
type Article struct {
	Title     string
	Byline    string
	SiteName  string
	Excerpt   string
	Lang      string
	Published string
	// Node is the main-content subtree, sanitised and ready for conversion. It is
	// nil when readability found no article.
	Node *html.Node
	// Links are every outbound hyperlink in the whole document.
	Links []Link
	// LowConfidence is true when readability could not isolate a clear article and
	// yomi fell back to a coarse selection.
	LowConfidence bool
}

// FromHTML parses an HTML body and extracts its Article. pageURL is the absolute
// URL of the page, used to resolve relative links and to guide readability.
func FromHTML(body []byte, pageURL string) (*Article, error) {
	base, _ := url.Parse(pageURL)

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	art := &Article{}
	walk(doc, base, art)

	// Keep class attributes through readability so syntax-highlighter language
	// hints (class="language-go", data-lang, and friends) survive to the code
	// normalisation pass below; readability strips all classes by default.
	parser := readability.NewParser()
	parser.KeepClasses = true
	rd, err := parser.Parse(bytes.NewReader(body), base)
	if err == nil && rd.Node != nil {
		art.Node = rd.Node
		sanitize.CleanTree(art.Node, sanitize.Options{})
		normalizeCode(art.Node)
		fillFromReadability(art, rd)
	} else {
		art.LowConfidence = true
	}
	return art, nil
}

// fillFromReadability backfills fields the page's own tags did not provide.
func fillFromReadability(a *Article, rd readability.Article) {
	if a.Title == "" {
		a.Title = strings.TrimSpace(rd.Title())
	}
	if a.Lang == "" {
		a.Lang = rd.Language()
	}
	if a.Byline == "" {
		a.Byline = strings.TrimSpace(rd.Byline())
	}
	if a.SiteName == "" {
		a.SiteName = strings.TrimSpace(rd.SiteName())
	}
	if a.Excerpt == "" {
		a.Excerpt = strings.TrimSpace(rd.Excerpt())
	}
	if a.Published == "" {
		if t, err := rd.PublishedTime(); err == nil && !t.IsZero() {
			a.Published = t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
}

// walk traverses the document collecting metadata and links.
func walk(n *html.Node, base *url.URL, a *Article) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "html":
			if v := attr(n, "lang"); v != "" && a.Lang == "" {
				a.Lang = v
			}
		case "title":
			if a.Title == "" {
				a.Title = collapse(text(n))
			}
		case "meta":
			meta(n, a)
		case "a":
			if u := resolve(base, attr(n, "href")); u != "" {
				a.Links = append(a.Links, Link{URL: u, Text: collapse(text(n))})
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, base, a)
	}
}

// meta reads one <meta> tag: standard names, OpenGraph properties, and the
// article publish timestamp.
func meta(n *html.Node, a *Article) {
	content := attr(n, "content")
	if content == "" {
		return
	}
	switch strings.ToLower(attr(n, "name")) {
	case "description":
		if a.Excerpt == "" {
			a.Excerpt = content
		}
	case "author":
		if a.Byline == "" {
			a.Byline = content
		}
	}
	switch strings.ToLower(attr(n, "property")) {
	case "og:title":
		if a.Title == "" {
			a.Title = content
		}
	case "og:description":
		if a.Excerpt == "" {
			a.Excerpt = content
		}
	case "og:site_name":
		if a.SiteName == "" {
			a.SiteName = content
		}
	case "article:published_time":
		if a.Published == "" {
			a.Published = content
		}
	}
}

func attr(n *html.Node, key string) string {
	for _, at := range n.Attr {
		if strings.EqualFold(at.Key, key) {
			return at.Val
		}
	}
	return ""
}

// text returns the concatenated text of a node and its descendants.
func text(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}

// collapse trims and replaces every run of whitespace with a single space.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// resolve turns a possibly relative href into an absolute http(s) URL, returning
// "" for empty, fragment-only, or non-http links.
func resolve(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}
	switch {
	case strings.HasPrefix(href, "javascript:"),
		strings.HasPrefix(href, "mailto:"),
		strings.HasPrefix(href, "tel:"),
		strings.HasPrefix(href, "data:"):
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	return u.String()
}
