package yomi

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/tamnd/kage/urlx"
	"github.com/tamnd/yomi/site"
)

// SiteResult reports the outcome of reading a whole site.
type SiteResult struct {
	Seed    string
	Pages   []*Page
	OutPath string // the folder or single file written
	Single  bool
}

// Site crawls the website at seedArg within scope, reads every in-scope page,
// and writes the result in the requested shape: a folder of Markdown (default),
// one combined Markdown file (opts.Single), or one JSON/JSONL dataset
// (opts.Format). A Markdown crawl is resumable (opts.Resume), backed by an
// incremental sidecar. It returns the pages and where they were written; on an
// interrupt it returns the partial result with the cancellation error, so the
// caller can report what was saved.
func Site(ctx context.Context, seedArg string, opts Options) (*SiteResult, error) {
	seed, err := urlx.ParseSeed(seedArg)
	if err != nil {
		return nil, err
	}
	outRoot := opts.Out
	if outRoot == "" {
		outRoot = seed.Hostname()
	}

	r := newReader(opts)
	defer func() { _ = r.close() }()

	if opts.Format == SiteJSON || opts.Format == SiteJSONL {
		return r.siteDataset(ctx, seed, outRoot, opts)
	}
	return r.siteMarkdown(ctx, seed, outRoot, opts)
}

// siteMarkdown is the folder/single-file path, resume-aware. Each new page is
// written as it is read (its .md in folder mode) and appended to the sidecar, so
// an interrupt loses at most the page in flight; --resume replays the sidecar to
// skip pages already done. The final table of contents (SUMMARY.md) or the
// combined file is assembled from the full set at the end.
func (r *reader) siteMarkdown(ctx context.Context, seed *url.URL, outRoot string, opts Options) (*SiteResult, error) {
	statePath := resumeStatePath(outRoot, opts.Single)

	done := map[string]*Page{}
	if opts.Resume {
		loaded, lerr := loadResumeState(statePath)
		if lerr != nil {
			return nil, lerr
		}
		done = loaded
	}
	if !opts.Single {
		if err := os.MkdirAll(outRoot, 0o755); err != nil {
			return nil, err
		}
	}
	rw, err := openResumeWriter(statePath, opts.Resume)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rw.close() }()

	var (
		mu    sync.Mutex
		pages []*Page
		seen  = map[string]bool{}
	)
	// Replay the resumed pages into the accumulator and the seen set, so the
	// final assembly includes them and the crawl skips re-fetching them.
	for _, p := range done {
		pages = append(pages, p)
		seen[canonURL(p.URL)] = true
	}

	visit := func(ctx context.Context, rawURL string, depth int) ([]string, error) {
		mu.Lock()
		if p := done[canonURL(ensureScheme(rawURL))]; p != nil {
			mu.Unlock()
			return internalLinks(p), nil // resumed: skip the fetch, keep walking
		}
		mu.Unlock()

		u, _ := url.Parse(rawURL)
		pr := r.siteRewrite(seed, opts, outRoot, u)
		p, err := r.read(ctx, rawURL, pr)
		if err != nil {
			return nil, err
		}
		final, _ := url.Parse(p.URL)
		p.Path = mdPath(final)
		p.Anchor = anchorFor(final)

		key := canonURL(p.URL)
		mu.Lock()
		if seen[key] { // two request URLs converged on one page after a redirect
			mu.Unlock()
			return internalLinks(p), nil
		}
		seen[key] = true
		pages = append(pages, p)
		done[key] = p
		if !opts.Single {
			if werr := writePageFile(outRoot, p, opts); werr != nil {
				mu.Unlock()
				return nil, werr
			}
		}
		_ = rw.append(p)
		mu.Unlock()
		opts.log("read %s (%d words)", p.URL, p.WordCount)
		return internalLinks(p), nil
	}

	crawlErr := site.Crawl(ctx, siteConfig(seed, opts), visit)

	sortPages(pages)
	res := &SiteResult{Seed: seed.String(), Pages: pages, OutPath: outRoot, Single: opts.Single}
	if opts.Single {
		if err := writeSingle(outRoot, seed, pages, opts); err != nil {
			return res, err
		}
	} else if err := writeSummary(outRoot, pages); err != nil {
		return res, err
	}
	return res, crawlErr
}

// siteDataset is the json/jsonl path: it crawls the site and writes one
// structured file of every page. It is not resumable; the dataset is rewritten
// whole, including the partial set when a crawl is interrupted.
func (r *reader) siteDataset(ctx context.Context, seed *url.URL, outFile string, opts Options) (*SiteResult, error) {
	var (
		mu    sync.Mutex
		pages []*Page
		seen  = map[string]bool{}
	)
	visit := func(ctx context.Context, rawURL string, depth int) ([]string, error) {
		u, _ := url.Parse(rawURL)
		pr := r.siteRewrite(seed, opts, outFile, u)
		p, err := r.read(ctx, rawURL, pr)
		if err != nil {
			return nil, err
		}
		final, _ := url.Parse(p.URL)
		p.Path = mdPath(final)
		p.Anchor = anchorFor(final)
		key := canonURL(p.URL)
		mu.Lock()
		if seen[key] {
			mu.Unlock()
			return internalLinks(p), nil
		}
		seen[key] = true
		pages = append(pages, p)
		mu.Unlock()
		opts.log("read %s (%d words)", p.URL, p.WordCount)
		return internalLinks(p), nil
	}

	crawlErr := site.Crawl(ctx, siteConfig(seed, opts), visit)

	sortPages(pages)
	if err := writeDataset(outFile, pages, opts.Format); err != nil {
		return &SiteResult{Seed: seed.String(), Pages: pages, OutPath: outFile}, err
	}
	return &SiteResult{Seed: seed.String(), Pages: pages, OutPath: outFile}, crawlErr
}

// siteConfig builds the crawl frontier config from the read options, including
// the optional sitemap seeding.
func siteConfig(seed *url.URL, opts Options) site.Config {
	return site.Config{
		Seed:      seed,
		Scope:     opts.Scope,
		MaxPages:  opts.MaxPages,
		MaxDepth:  opts.MaxDepth,
		Workers:   opts.Workers,
		Robots:    opts.Robots,
		Sitemap:   opts.Sitemap,
		UserAgent: opts.UserAgent,
		Timeout:   opts.Timeout,
		Logf:      opts.Logf,
	}
}

// writeDataset writes the pages as one JSON array (SiteJSON) or one record per
// line (SiteJSONL). The pages carry their Markdown body, so the dataset is the
// full reading, not just the metadata.
func writeDataset(outFile string, pages []*Page, format SiteFormat) error {
	if dir := path.Dir(outFile); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if format == SiteJSONL {
		enc := json.NewEncoder(f)
		for _, p := range pages {
			if err := enc.Encode(p); err != nil {
				return err
			}
		}
		return nil
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(pages)
}

// siteRewrite builds the per-page rewiring: internal links become local .md
// paths (folder) or in-file anchors (single), and downloaded images land in a
// shared media folder at the site root.
func (r *reader) siteRewrite(seed *url.URL, opts Options, outRoot string, pageURL *url.URL) rewrite {
	thisPath := mdPath(pageURL)
	toRoot := relToRoot(thisPath)

	rw := rewrite{}
	rw.link = func(abs string) (string, bool) {
		u, err := url.Parse(abs)
		if err != nil {
			return "", false
		}
		if !urlx.InScope(seed, u, opts.Scope) || !urlx.LikelyPage(u) {
			return "", false // external: leave absolute
		}
		if opts.Single {
			return "#" + anchorFor(u), true
		}
		return toRoot + mdPath(u), true
	}
	if opts.Images == ImageDownload {
		rw.imageDir = path.Join(outRoot, "media")
		rw.imageRel = toRoot + "media"
	}
	return rw
}

// internalLinks returns the URLs of a page's in-scope links, for the crawl to
// follow.
func internalLinks(p *Page) []string {
	var out []string
	for _, l := range p.Links {
		if l.Internal {
			out = append(out, l.URL)
		}
	}
	return out
}

// sortPages orders pages deterministically: shallower URL paths first, ties
// broken by URL, so the folder index and the single-file table of contents read
// top-down and a re-run is byte-identical.
func sortPages(pages []*Page) {
	sort.Slice(pages, func(i, j int) bool {
		di, dj := strings.Count(pages[i].Path, "/"), strings.Count(pages[j].Path, "/")
		if di != dj {
			return di < dj
		}
		return pages[i].Path < pages[j].Path
	})
}

// mdPath maps a page URL to its slash-separated .md path under the site root.
// A directory URL (trailing slash, or the root) becomes index.md in that
// directory; a leaf becomes <name>.md.
func mdPath(u *url.URL) string {
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return "index.md"
	}
	if strings.HasSuffix(u.Path, "/") {
		return p + "/index.md"
	}
	// A path whose last segment already names a file keeps its stem.
	if ext := path.Ext(p); ext != "" {
		p = strings.TrimSuffix(p, ext)
	}
	return p + ".md"
}

// anchorFor derives a stable in-file anchor for a page from its URL path, so a
// single-file build can link between pages deterministically.
func anchorFor(u *url.URL) string {
	p := strings.Trim(u.Path, "/")
	if p == "" {
		return "index"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(p) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// relToRoot returns the "../" prefix that reaches the site root from a page at
// the given .md path: "" for a root page, "../" one level down, and so on.
func relToRoot(mdPath string) string {
	return strings.Repeat("../", strings.Count(mdPath, "/"))
}
