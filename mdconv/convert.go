// Package mdconv converts an extracted article node to GitHub-Flavored Markdown.
// Before conversion it rewrites the node's links and images through caller
// callbacks, so a site build can point internal links at local files or in-file
// anchors and the image policy can localise pictures. After conversion it tidies
// the Markdown: no blank-line runs, a single trailing newline.
package mdconv

import (
	"net/url"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

// Options configure one conversion.
type Options struct {
	// Base resolves relative href and src values to absolute URLs before the
	// rewrite callbacks see them.
	Base *url.URL
	// RewriteLink maps an absolute link URL to the target written into the
	// Markdown. A nil callback, or an empty return, leaves the link as the
	// absolute URL.
	RewriteLink func(abs string) string
	// RewriteImage maps an absolute image URL (with its alt text) to the target
	// written into the Markdown. A nil callback, or an empty return, leaves the
	// image as the absolute URL.
	RewriteImage func(abs, alt string) string
}

// Convert rewrites node's references and renders it to Markdown. The node is
// mutated in place by the rewrite, which is fine because it is a per-page copy.
func Convert(node *html.Node, opts Options) (string, error) {
	rewriteRefs(node, opts)
	out, err := htmltomarkdown.ConvertNode(node)
	if err != nil {
		return "", err
	}
	return Tidy(string(out)), nil
}

// rewriteRefs walks node rewriting every <a href> and <img src> through the
// callbacks, resolving relative URLs against the base first.
func rewriteRefs(n *html.Node, opts Options) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "a":
			if abs, ok := absolute(opts.Base, getAttr(n, "href")); ok && opts.RewriteLink != nil {
				if t := opts.RewriteLink(abs); t != "" {
					setAttr(n, "href", t)
				} else {
					setAttr(n, "href", abs)
				}
			} else if ok {
				setAttr(n, "href", abs)
			}
		case "img":
			if abs, ok := absolute(opts.Base, getAttr(n, "src")); ok {
				alt := getAttr(n, "alt")
				if opts.RewriteImage != nil {
					if t := opts.RewriteImage(abs, alt); t != "" {
						setAttr(n, "src", t)
					} else {
						setAttr(n, "src", abs)
					}
				} else {
					setAttr(n, "src", abs)
				}
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		rewriteRefs(c, opts)
	}
}

// absolute resolves ref against base. It reports false for empty, fragment-only,
// or non-http references, which are left untouched.
func absolute(base *url.URL, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "data:") {
		return "", false
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	return u.String(), true
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// Tidy normalises converted Markdown: it trims trailing spaces, collapses runs of
// blank lines to one, and ends the document with exactly one newline.
func Tidy(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, ln := range lines {
		ln = strings.TrimRight(ln, " \t")
		if ln == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n")) + "\n"
}
