package yomi

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// writeFolder writes one Markdown file per page under root, mirroring the URL
// paths, and a SUMMARY.md table of contents linking them all.
func writeFolder(root string, _ *url.URL, pages []*Page, opts Options) error {
	for _, p := range pages {
		if err := writePageFile(root, p, opts); err != nil {
			return err
		}
	}
	return writeSummary(root, pages)
}

// writePageFile writes one page's Markdown file under root at its .md path,
// creating any parent directories. It is what the resumable crawl calls per page
// so an interrupt leaves every page read so far already on disk.
func writePageFile(root string, p *Page, opts Options) error {
	full := filepath.Join(root, filepath.FromSlash(p.Path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(Document(p, opts)), 0o644)
}

// writeSummary writes the SUMMARY.md table of contents for the pages under root.
func writeSummary(root string, pages []*Page) error {
	return os.WriteFile(filepath.Join(root, "SUMMARY.md"), []byte(buildSummary(pages)), 0o644)
}

// buildSummary renders the SUMMARY.md: an ordered, depth-nested list of every
// page, each linking its relative .md file by its readable title.
func buildSummary(pages []*Page) string {
	var b strings.Builder
	b.WriteString("# Summary\n\n")
	for _, p := range pages {
		indent := strings.Repeat("  ", strings.Count(p.Path, "/"))
		fmt.Fprintf(&b, "%s- [%s](%s)\n", indent, titleOr(p), p.Path)
	}
	return b.String()
}

// writeSingle assembles every page into one Markdown file at root: a
// document-level front-matter block, a table of contents, then each page as an
// anchored section.
func writeSingle(root string, sd *url.URL, pages []*Page, opts Options) error {
	var b strings.Builder
	if opts.FrontMatter {
		writeSiteFrontMatter(&b, sd, pages, opts)
		b.WriteString("\n")
	}
	b.WriteString("# ")
	b.WriteString(siteTitle(sd, pages))
	b.WriteString("\n\n## Contents\n\n")
	for _, p := range pages {
		indent := strings.Repeat("  ", strings.Count(p.Path, "/"))
		fmt.Fprintf(&b, "%s- [%s](#%s)\n", indent, titleOr(p), p.Anchor)
	}
	b.WriteString("\n")
	for _, p := range pages {
		writeSection(&b, p)
	}
	out := root
	if !strings.HasSuffix(out, ".md") {
		out += ".md"
	}
	if dir := filepath.Dir(out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

// writeSection writes one page as a section of the single-file document: an
// anchored heading, a small source line, and the body with its own headings
// demoted so the document keeps one outline.
func writeSection(b *strings.Builder, p *Page) {
	depth := strings.Count(p.Path, "/")
	level := 2 + depth
	if level > 6 {
		level = 6
	}
	fmt.Fprintf(b, "%s %s\n\n", strings.Repeat("#", level), titleOr(p))
	// An explicit anchor span so the table-of-contents links resolve in plain
	// Markdown renderers that do not auto-slug headings the same way.
	fmt.Fprintf(b, "<a id=%q></a>\n\n", p.Anchor)
	fmt.Fprintf(b, "*Source: <%s>", p.URL)
	if p.ReadingMin > 0 {
		fmt.Fprintf(b, " - %d min read", p.ReadingMin)
	}
	b.WriteString("*\n\n")
	b.WriteString(demoteHeadings(p.Markdown, level))
	b.WriteString("\n\n")
}

// demoteHeadings shifts every ATX heading in body down so the shallowest sits
// just beneath a section at the given level, keeping the assembled document's
// outline coherent. It never pushes a heading past level 6.
func demoteHeadings(body string, sectionLevel int) string {
	lines := strings.Split(body, "\n")
	inFence := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if h := countHash(ln); h > 0 {
			add := sectionLevel
			newLevel := h + add
			if newLevel > 6 {
				newLevel = 6
			}
			lines[i] = strings.Repeat("#", newLevel) + ln[h:]
		}
	}
	return strings.Join(lines, "\n")
}

// countHash returns the number of leading '#' of an ATX heading line, or 0 when
// the line is not a heading.
func countHash(ln string) int {
	n := 0
	for n < len(ln) && ln[n] == '#' {
		n++
	}
	if n == 0 || n > 6 || n >= len(ln) || ln[n] != ' ' {
		return 0
	}
	return n
}

func writeSiteFrontMatter(b *strings.Builder, sd *url.URL, pages []*Page, opts Options) {
	b.WriteString("---\n")
	fmt.Fprintf(b, "title: %q\n", siteTitle(sd, pages))
	fmt.Fprintf(b, "url: %q\n", sd.String())
	fmt.Fprintf(b, "pages: %d\n", len(pages))
	if opts.Fetched != "" {
		fmt.Fprintf(b, "fetched: %q\n", opts.Fetched)
	}
	total := 0
	for _, p := range pages {
		total += p.ReadingMin
	}
	fmt.Fprintf(b, "reading_time: %d\n", total)
	b.WriteString("---\n")
}

// siteTitle picks a title for the whole site: the root page's title when there
// is one, else the host.
func siteTitle(sd *url.URL, pages []*Page) string {
	for _, p := range pages {
		if p.Path == "index.md" && p.Title != "" {
			return p.Title
		}
	}
	return sd.Hostname()
}

func titleOr(p *Page) string {
	if p.Title != "" {
		return p.Title
	}
	return p.Path
}
