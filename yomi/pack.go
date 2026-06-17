package yomi

import (
	"context"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/tamnd/kage/urlx"
	"github.com/tamnd/yomi/site"
)

// PackFormat is the artifact a pack run produces.
type PackFormat string

const (
	// PackSQLite writes the page store itself as the deliverable: a SQLite
	// database of pages, links, and images.
	PackSQLite PackFormat = "sqlite"
	// PackZIM compiles the page store into a ZIM offline archive, browsable in
	// Kiwix, and keeps the store alongside it for the next incremental run.
	PackZIM PackFormat = "zim"
)

// PackOptions govern a pack run. It reuses the read and crawl Options and adds
// the packaging choices on top.
type PackOptions struct {
	Options

	Format PackFormat // sqlite (default) or zim
	Out    string     // the artifact path (the .db for sqlite, the .zim for zim)
	State  string     // the SQLite store path; equals Out for the sqlite format

	Refresh bool          // re-fetch pages already in the store
	MaxAge  time.Duration // re-fetch a stored page older than this (0 = never)

	NoCompress  bool   // zim: store every cluster raw, no compression
	Title       string // zim metadata: archive title
	Description string // zim metadata: archive description
	Language    string // zim metadata: ISO 639-3 language code
	Date        string // zim metadata: archive date (YYYY-MM-DD)
	Version     string // tool version recorded as the archive's creator
}

// PackResult reports the outcome of a pack run.
type PackResult struct {
	Seed      string
	Format    PackFormat
	OutPath   string // the artifact a user opens
	StorePath string // the backing SQLite store
	Pages     int    // total pages in the store after this run
	Added     int    // pages fetched and written this run
	Skipped   int    // pages already present and kept without re-fetching
	Words     int    // total words across the store
	Bytes     int64  // size of the artifact on disk
}

// Pack crawls the site at seedArg into a resumable SQLite store and produces the
// requested artifact. A re-run reuses the store: pages already present are kept
// without re-fetching (resume), unless --refresh forces them or --max-age finds
// them stale (incremental update). For the ZIM format the store is compiled into
// the archive at the end and kept as a sidecar for next time.
func Pack(ctx context.Context, seedArg string, popts PackOptions) (*PackResult, error) {
	seed, err := urlx.ParseSeed(seedArg)
	if err != nil {
		return nil, err
	}
	host := seed.Hostname()

	st, err := openStore(popts.State)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.close() }()

	_ = st.setMeta("seed", seed.String())
	_ = st.setMeta("host", host)
	if created, _ := st.getMeta("created"); created == "" {
		_ = st.setMeta("created", popts.Fetched)
	}
	_ = st.setMeta("yomi", firstNonEmpty(popts.Version, "yomi"))

	now := parseStamp(popts.Fetched)
	r := newReader(popts.Options)
	defer func() { _ = r.close() }()
	rw := r.packRewrite(seed, popts.Options)

	var (
		mu             sync.Mutex
		added, skipped int
		seen           = map[string]bool{}
	)
	visit := func(ctx context.Context, rawURL string, depth int) ([]string, error) {
		if !popts.Refresh {
			if links, fresh, lerr := st.lookup(rawURL, popts.MaxAge, now); lerr == nil && fresh {
				mu.Lock()
				skipped++
				mu.Unlock()
				return links, nil
			}
		}
		p, err := r.read(ctx, rawURL, rw)
		if err != nil {
			return nil, err
		}
		final, _ := url.Parse(p.URL)
		p.Path = mdPath(final)
		p.Anchor = anchorFor(final)

		// Two request URLs can converge on the same page after a redirect; the
		// crawler dedups on the request URL, so it only learns they are the same
		// here, once the final URL is known. Write the page once and let the
		// duplicate ride on the stored links.
		key := canonURL(p.URL)
		mu.Lock()
		dup := seen[key]
		seen[key] = true
		mu.Unlock()
		if dup {
			return internalLinks(p), nil
		}
		// The first lookup keyed on the request URL; a redirect alias only matches
		// the store once we know where it landed. Re-check on the final URL so a
		// resume keeps the page instead of rewriting it.
		if !popts.Refresh {
			if links, fresh, lerr := st.lookup(p.URL, popts.MaxAge, now); lerr == nil && fresh {
				mu.Lock()
				skipped++
				mu.Unlock()
				return links, nil
			}
		}

		if err := st.put(ctx, p, depth, popts.Fetched); err != nil {
			return nil, err
		}
		mu.Lock()
		added++
		mu.Unlock()
		popts.log("packed %s (%d words)", p.URL, p.WordCount)
		return internalLinks(p), nil
	}

	cfg := site.Config{
		Seed:      seed,
		Scope:     popts.Scope,
		MaxPages:  popts.MaxPages,
		MaxDepth:  popts.MaxDepth,
		Workers:   popts.Workers,
		Robots:    popts.Robots,
		UserAgent: popts.UserAgent,
		Timeout:   popts.Timeout,
		Logf:      popts.Logf,
	}
	crawlErr := site.Crawl(ctx, cfg, visit)

	_ = st.setMeta("updated", popts.Fetched)
	pages, words, _ := st.counts()
	_ = st.setMeta("pages", strconv.Itoa(pages))
	_ = st.setMeta("format", string(popts.Format))

	res := &PackResult{
		Seed:      seed.String(),
		Format:    popts.Format,
		StorePath: popts.State,
		Pages:     pages,
		Added:     added,
		Skipped:   skipped,
		Words:     words,
	}

	if popts.Format == PackZIM {
		n, zerr := buildZIM(st, popts, host, popts.Out)
		if zerr != nil {
			if crawlErr != nil {
				return res, crawlErr
			}
			return res, zerr
		}
		res.OutPath = popts.Out
		res.Bytes = n
	} else {
		res.OutPath = popts.State
	}
	return res, crawlErr
}

// packRewrite is the link strategy for a pack crawl: every link target is left
// absolute in the stored Markdown, but in-scope page links are flagged internal
// so the crawl follows them and a ZIM build can later rewire them to sibling
// entries.
func (r *reader) packRewrite(seed *url.URL, opts Options) rewrite {
	return rewrite{
		link: func(abs string) (string, bool) {
			u, err := url.Parse(abs)
			if err != nil {
				return "", false
			}
			internal := urlx.InScope(seed, u, opts.Scope) && urlx.LikelyPage(u)
			return "", internal
		},
	}
}

// parseStamp parses an RFC 3339 timestamp, returning the zero time when it is
// empty or malformed. The zero time only matters when --max-age is set, where it
// makes a stored page look old and earns a refresh.
func parseStamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
