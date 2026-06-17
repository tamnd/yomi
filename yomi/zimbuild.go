package yomi

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/tamnd/kage/zim"
)

// buildZIM compiles the pages already in the store into a ZIM archive at
// outPath. Each page becomes a self-contained HTML entry under the content
// namespace, a generated contents page is the archive's main page, and internal
// links are rewired to point at sibling entries so the archive browses offline.
// It returns the number of bytes written.
func buildZIM(st *store, popts PackOptions, host, outPath string) (int64, error) {
	pages, err := st.allPages()
	if err != nil {
		return 0, err
	}
	if len(pages) == 0 {
		return 0, fmt.Errorf("nothing to pack: the store has no pages")
	}

	// Assign a stable, collision-free archive URL to every page, and remember
	// the canonical-URL to archive-URL mapping so links between pages resolve.
	used := map[string]bool{}
	zurls := make([]string, len(pages))
	canon2zim := map[string]string{}
	for i, p := range pages {
		u := uniqueName(used, htmlName(p.Path))
		used[u] = true
		zurls[i] = u
		canon2zim[canonURL(p.URL)] = u
	}
	contentsURL := uniqueName(used, "index.html", "contents.html", "_yomi_contents.html")
	used[contentsURL] = true

	title := firstNonEmpty(popts.Title, siteTitleFromPages(host, pages))

	w := zim.NewWriter()
	if popts.NoCompress {
		w.SetNoCompress(true)
	}
	w.AddMetadata("Title", title)
	w.AddMetadata("Name", host)
	w.AddMetadata("Language", firstNonEmpty(popts.Language, "eng"))
	w.AddMetadata("Description", firstNonEmpty(popts.Description, "Offline reading of "+host+", packed by yomi."))
	w.AddMetadata("Creator", firstNonEmpty(popts.Version, "yomi"))
	w.AddMetadata("Publisher", "yomi")
	w.AddMetadata("Scraper", firstNonEmpty(popts.Version, "yomi"))
	if popts.Date != "" {
		w.AddMetadata("Date", popts.Date)
	}

	for i, p := range pages {
		body, err := renderMarkdown(p.Markdown)
		if err != nil {
			return 0, fmt.Errorf("render %s: %w", p.URL, err)
		}
		prefix := relToRoot(zurls[i])
		body = rewriteLinks(body, func(href string) (string, bool) {
			if t, ok := canon2zim[canonURL(href)]; ok {
				return prefix + t, true
			}
			return "", false
		})
		doc := pageHTML(p, body, prefix+contentsURL)
		w.AddContent(zim.NamespaceContent, zurls[i], titleOr(p), "text/html", []byte(doc))
	}

	w.AddContent(zim.NamespaceContent, contentsURL, title, "text/html",
		[]byte(indexHTML(title, host, pages, zurls)))
	w.SetMainPage(zim.NamespaceContent, contentsURL)

	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriter(f)
	n, werr := w.WriteTo(bw)
	if ferr := bw.Flush(); werr == nil {
		werr = ferr
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return n, werr
}

// htmlName maps a page's .md path to its archive HTML URL: the same path with an
// .html suffix, so /greatwork.html stays greatwork.html and a directory index
// stays index.html in its folder.
func htmlName(mdPath string) string {
	return strings.TrimSuffix(mdPath, ".md") + ".html"
}

// uniqueName returns the first candidate not already taken, or, when every
// candidate collides, the last candidate with a numeric suffix. It keeps archive
// URLs distinct even when two source URLs map to the same path.
func uniqueName(used map[string]bool, candidates ...string) string {
	for _, c := range candidates {
		if !used[c] {
			return c
		}
	}
	base := candidates[len(candidates)-1]
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if !used[c] {
			return c
		}
	}
}

// siteTitleFromPages picks an archive title: the root page's title when there is
// one, otherwise the host.
func siteTitleFromPages(host string, pages []*Page) string {
	for _, p := range pages {
		if p.Path == "index.md" && p.Title != "" {
			return p.Title
		}
	}
	return host
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
