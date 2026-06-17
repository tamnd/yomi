package yomi

import (
	"context"
	"net/url"
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

// Site crawls the website at seedArg within scope, reads every in-scope page to
// Markdown, and assembles the result into a folder (default) or a single file
// (opts.Single). It returns the pages and where they were written.
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

	var (
		mu    sync.Mutex
		pages []*Page
	)
	visit := func(ctx context.Context, rawURL string, depth int) ([]string, error) {
		u, _ := url.Parse(rawURL)
		rw := r.siteRewrite(seed, opts, outRoot, u)
		p, err := r.read(ctx, rawURL, rw)
		if err != nil {
			return nil, err
		}
		final, _ := url.Parse(p.URL)
		p.Path = mdPath(final)
		p.Anchor = anchorFor(final)
		mu.Lock()
		pages = append(pages, p)
		mu.Unlock()
		opts.log("read %s (%d words)", p.URL, p.WordCount)
		return internalLinks(p), nil
	}

	cfg := site.Config{
		Seed:      seed,
		Scope:     opts.Scope,
		MaxPages:  opts.MaxPages,
		MaxDepth:  opts.MaxDepth,
		Workers:   opts.Workers,
		Robots:    opts.Robots,
		UserAgent: opts.UserAgent,
		Timeout:   opts.Timeout,
		Logf:      opts.Logf,
	}
	if err := site.Crawl(ctx, cfg, visit); err != nil {
		return nil, err
	}

	sortPages(pages)
	res := &SiteResult{Seed: seed.String(), Pages: pages, Single: opts.Single}
	if opts.Single {
		if err := writeSingle(outRoot, seed, pages, opts); err != nil {
			return nil, err
		}
		res.OutPath = outRoot
	} else {
		if err := writeFolder(outRoot, seed, pages, opts); err != nil {
			return nil, err
		}
		res.OutPath = outRoot
	}
	return res, nil
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
