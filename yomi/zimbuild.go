package yomi

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"sort"
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
	// Creator is the origin of the content, the site itself; Publisher and
	// Scraper name the tool that built the archive.
	w.AddMetadata("Creator", host)
	w.AddMetadata("Publisher", "yomi")
	w.AddMetadata("Scraper", strings.TrimSpace("yomi "+popts.Version))
	// The archive carries no full-text (Xapian) index, only the title index, so
	// tell Kiwix not to advertise search-in-book it cannot serve.
	w.AddMetadata("Tags", "_ftindex:no")
	if popts.Date != "" {
		w.AddMetadata("Date", popts.Date)
	}

	// The library icon Kiwix shows for the archive. A caller-supplied PNG wins;
	// otherwise a built-in reading icon, so the tile is never a blank placeholder.
	if icon := illustration(popts.Icon); icon != nil {
		w.AddMetadataBytes("Illustration_48x48@1", "image/png", icon)
	}

	mimeCounts := map[string]int{}
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
		mimeCounts["text/html"]++
	}

	w.AddContent(zim.NamespaceContent, contentsURL, title, "text/html",
		[]byte(indexHTML(title, host, pages, zurls)))
	mimeCounts["text/html"]++
	w.SetMainPage(zim.NamespaceContent, contentsURL)

	// Counter lets Kiwix report the archive's content breakdown by MIME type.
	w.AddMetadata("Counter", encodeCounter(mimeCounts))

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

// illustration returns the PNG bytes for the archive's library icon. When path
// is set it reads that file; a read failure falls back to the built-in icon so a
// bad path never aborts the pack. With no path it returns the built-in icon.
func illustration(path string) []byte {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return data
		}
	}
	return defaultIllustration()
}

// encodeCounter renders a MIME-type histogram as the Kiwix Counter value, the
// "mime=count" pairs joined by semicolons, in a stable order.
func encodeCounter(counts map[string]int) string {
	mimes := make([]string, 0, len(counts))
	for m := range counts {
		mimes = append(mimes, m)
	}
	sort.Strings(mimes)
	parts := make([]string, 0, len(mimes))
	for _, m := range mimes {
		parts = append(parts, fmt.Sprintf("%s=%d", m, counts[m]))
	}
	return strings.Join(parts, ";")
}
