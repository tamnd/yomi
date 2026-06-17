package yomi

import (
	"fmt"
	"strings"
)

// Document renders a Page to the Markdown a reader sees: an optional YAML
// front-matter block, an optional title heading, then the body. It is what the
// CLI writes to a file or stdout for a single page.
func Document(p *Page, opts Options) string {
	var b strings.Builder
	if opts.FrontMatter {
		b.WriteString(FrontMatter(p))
		b.WriteString("\n")
	}
	if opts.TitleHeading && p.Title != "" {
		b.WriteString("# ")
		b.WriteString(p.Title)
		b.WriteString("\n\n")
	}
	b.WriteString(p.Markdown)
	return b.String()
}

// FrontMatter renders a Page's metadata as a YAML front-matter block. Only
// non-empty fields are written, in a fixed order, so output is deterministic.
func FrontMatter(p *Page) string {
	var b strings.Builder
	b.WriteString("---\n")
	yamlField(&b, "title", p.Title)
	yamlField(&b, "url", p.URL)
	yamlField(&b, "site", p.SiteName)
	yamlField(&b, "byline", p.Byline)
	yamlField(&b, "published", p.Published)
	yamlField(&b, "fetched", p.Fetched)
	yamlField(&b, "lang", p.Lang)
	if p.WordCount > 0 {
		fmt.Fprintf(&b, "word_count: %d\n", p.WordCount)
	}
	if p.ReadingMin > 0 {
		fmt.Fprintf(&b, "reading_time: %d\n", p.ReadingMin)
	}
	b.WriteString("---\n")
	return b.String()
}

// yamlField writes one quoted YAML string field when the value is non-empty.
func yamlField(b *strings.Builder, key, val string) {
	if val == "" {
		return
	}
	fmt.Fprintf(b, "%s: %q\n", key, val)
}
