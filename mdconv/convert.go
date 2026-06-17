// Package mdconv converts an extracted article node to GitHub-Flavored Markdown.
// Before conversion it rewrites the node's links and images through caller
// callbacks, so a site build can point internal links at local files or in-file
// anchors and the image policy can localise pictures. After conversion it tidies
// the Markdown: no blank-line runs, a single trailing newline.
package mdconv

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
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
	unwrapSelfLinkedImages(node)
	rewriteRefs(node, opts)
	out, err := newConverter().ConvertNode(node)
	if err != nil {
		return "", err
	}
	md := cleanHeadings(string(out))
	md = dropDuplicateCaptions(md)
	md = dropWidgetLinks(md)
	md = dropPreviewCounters(md)
	return Tidy(md), nil
}

var numberOnly = regexp.MustCompile(`^\d{1,3}$`)

// dropPreviewCounters removes runs of standalone short-number lines that sit
// outside code fences. Component documentation often renders a live preview
// labelled 01, 02, 03 next to the code, and that gutter survives extraction as a
// column of bare numbers with nothing to anchor them. Only a run of two or more
// such lines is dropped, so a lone number in prose is never mistaken for a
// gutter, and numbers inside code (a REPL result, for instance) are untouched.
func dropPreviewCounters(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			inFence = !inFence
			out = append(out, ln)
			continue
		}
		if inFence || !numberOnly.MatchString(strings.TrimSpace(ln)) {
			out = append(out, ln)
			continue
		}
		// Gather a run of numeric lines separated only by blanks.
		j, count := i, 0
		for j < len(lines) {
			t := strings.TrimSpace(lines[j])
			if t == "" {
				j++
				continue
			}
			if !numberOnly.MatchString(t) {
				break
			}
			count++
			j++
		}
		if count >= 2 {
			i = j - 1 // skip the whole gutter run
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

var (
	headingLine = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)
	mdLink      = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
)

var imageOnlyLine = regexp.MustCompile(`^!\[([^\]]*)\]\([^)]*\)\s*$`)

// dropDuplicateCaptions removes a caption paragraph that only repeats the image
// it follows. Article platforms commonly emit a figure whose caption text is the
// same string as the image alt text, which extraction turns into an image line
// followed by a paragraph saying exactly the same thing. The repeat carries no
// information, so it is dropped; a caption that differs from the alt text is a
// real caption and is kept. Lines inside code fences are never touched.
func dropDuplicateCaptions(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	pendingAlt := ""
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			pendingAlt = ""
			out = append(out, ln)
			continue
		}
		if inFence {
			out = append(out, ln)
			continue
		}
		if pendingAlt != "" && t != "" {
			// First non-blank line after an image that carried alt text.
			if t == pendingAlt {
				pendingAlt = ""
				continue
			}
			pendingAlt = ""
		}
		if m := imageOnlyLine.FindStringSubmatch(ln); m != nil {
			pendingAlt = strings.TrimSpace(m[1])
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

var widgetLink = regexp.MustCompile(`(?i)^\[(share|subscribe|leave a comment|comment|give a gift subscription|pledge your support|refer a friend)\]\([^)]*\)\s*$`)

// dropWidgetLinks removes a standalone line that is only a link whose visible
// text is one of the share-and-subscribe call-to-action buttons publishing
// platforms inject around an article. Real prose is never a single such word
// linked on its own line, so the match is precise. Lines inside code fences are
// left alone.
func dropWidgetLinks(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			inFence = !inFence
			out = append(out, ln)
			continue
		}
		if !inFence && widgetLink.MatchString(strings.TrimSpace(ln)) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// unwrapSelfLinkedImages replaces a link that wraps only an image, and points at
// an image itself, with the bare image. Many article platforms link a picture to
// its own full-size file so a reader can open it in a lightbox, which is noise in
// a Markdown document where the link cannot do anything useful. A link that wraps
// an image but points at an article or any non-image target is left intact.
func unwrapSelfLinkedImages(n *html.Node) {
	var anchors []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "a" && soleImage(c) != nil && isImageURL(getAttr(c, "href")) {
				anchors = append(anchors, c)
			}
			walk(c)
		}
	}
	walk(n)
	for _, a := range anchors {
		img := soleImage(a)
		img.Parent.RemoveChild(img)
		a.Parent.InsertBefore(img, a)
		a.Parent.RemoveChild(a)
	}
}

// soleImage returns the one <img> in n's subtree when that image is the only
// content n carries. Whitespace text and line breaks are ignored, and wrapper
// elements that hold nothing but the image are descended into, since platforms
// nest a linked picture under layout divs. It returns nil when the subtree holds
// visible text, more than one image, or any other content-bearing element, which
// keeps a real caption or a second link from being unwrapped.
func soleImage(n *html.Node) *html.Node {
	var img *html.Node
	var walk func(*html.Node) bool
	walk = func(node *html.Node) bool {
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				if strings.TrimSpace(c.Data) != "" {
					return false
				}
			case html.ElementNode:
				switch c.Data {
				case "img":
					if img != nil {
						return false
					}
					img = c
				case "br", "source":
					// Layout-only, no content of their own.
				case "div", "span", "figure", "picture", "p":
					if !walk(c) {
						return false
					}
				default:
					return false
				}
			}
		}
		return true
	}
	if !walk(n) || img == nil {
		return nil
	}
	return img
}

var imageExt = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif", ".bmp"}

// isImageURL reports whether href points at an image, either by file extension or
// through a CDN transform path that serves an image.
func isImageURL(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" {
		return false
	}
	u, err := url.Parse(href)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	for _, ext := range imageExt {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return strings.Contains(p, "/image/fetch/")
}

// cleanHeadings strips the permalink decorations documentation generators hang
// off their headings: a trailing pilcrow or hash link, an empty link, or a
// heading whose whole text is a self-anchor. It unwraps fragment-only links to
// their visible text and drops links whose text is just a permalink glyph, while
// leaving real cross-reference links (those with an absolute or relative URL)
// untouched. Lines inside code fences are never rewritten.
func cleanHeadings(md string) string {
	lines := strings.Split(md, "\n")
	inFence := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := headingLine.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		cleaned := mdLink.ReplaceAllStringFunc(m[2], func(s string) string {
			sub := mdLink.FindStringSubmatch(s)
			text, target := sub[1], strings.TrimSpace(sub[2])
			// A real cross-reference (non-empty, non-fragment URL) stays as written.
			if target != "" && !strings.HasPrefix(target, "#") {
				return s
			}
			switch strings.TrimSpace(text) {
			case "", "¶", "#", "§", "🔗":
				return ""
			}
			return text
		})
		cleaned = strings.TrimRight(cleaned, " \t")
		if strings.TrimSpace(cleaned) == "" {
			continue
		}
		lines[i] = m[1] + " " + cleaned
	}
	return strings.Join(lines, "\n")
}

// newConverter builds the html-to-markdown converter yomi uses. Beyond the
// default base and commonmark plugins it adds GitHub-Flavored tables and
// strikethrough, so documentation tables become Markdown tables rather than a
// flattened run of cells. A fresh converter per call keeps conversions
// independent across the concurrent site crawler.
func newConverter() *converter.Converter {
	return converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
		strikethrough.NewStrikethroughPlugin(),
	))
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
