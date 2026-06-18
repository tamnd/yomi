package yomi

import (
	"context"
	"net/url"
	"strings"

	"github.com/tamnd/kage/asset"
	"github.com/tamnd/yomi/extract"
	"github.com/tamnd/yomi/fetch"
	"github.com/tamnd/yomi/mdconv"
)

// reader holds the per-run machinery shared across one or many pages: the
// fetcher (which owns the browser pool) and the asset downloader (for the image
// policy). Single-page Read and whole-site Site both drive it.
type reader struct {
	f    *fetch.Fetcher
	dl   *asset.Downloader
	opts Options
}

func newReader(opts Options) *reader {
	return &reader{
		f: fetch.New(fetch.Options{
			Mode:       fetch.Mode(opts.Render),
			UserAgent:  opts.UserAgent,
			Timeout:    opts.Timeout,
			Scroll:     opts.Scroll,
			ChromeBin:  opts.ChromeBin,
			ControlURL: opts.ControlURL,
			Logf:       opts.Logf,
		}),
		dl:   asset.NewDownloader(opts.UserAgent, opts.Timeout, opts.MaxImageBytes),
		opts: opts,
	}
}

// close releases the browser pool.
func (r *reader) close() error { return r.f.Close() }

// rewrite governs how an article's internal links are rewired during a site
// build. The zero rewrite (single-page read) leaves links absolute.
type rewrite struct {
	// link maps an absolute article link to its Markdown target and reports
	// whether it was internal to the crawl.
	link func(abs string) (target string, internal bool)
	// imageDir is the filesystem directory downloaded images are written into;
	// empty disables on-disk image localisation (falling back to remote URLs).
	imageDir string
	// imageRel is the Markdown path prefix that reaches imageDir from the page
	// being written, e.g. "post.media" or "../media".
	imageRel string
}

// read fetches rawURL, extracts the article, converts it, and returns a Page.
// rw is the site rewiring strategy; pass the zero rewrite for a standalone read.
func (r *reader) read(ctx context.Context, rawURL string, rw rewrite) (*Page, error) {
	rawURL = ensureScheme(rawURL)
	resp, err := r.f.Fetch(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	p, err := r.readHTML(ctx, resp.Body, resp.URL, rw)
	if err != nil {
		return nil, err
	}
	p.Rendered = resp.Rendered
	if rawURL != resp.URL {
		p.RequestURL = rawURL
	}
	return p, nil
}

// readHTML is the post-fetch half of a read: it extracts the article from an
// already-obtained HTML body, builds the Page, converts the body to Markdown,
// and collects its links. baseURL is the absolute URL the body came from, used
// to resolve relative links and images. It is the path the live-fetch read, the
// stdin read, and the local-file read all share.
func (r *reader) readHTML(ctx context.Context, body []byte, baseURL string, rw rewrite) (*Page, error) {
	art, err := extract.FromHTML(body, baseURL)
	if err != nil {
		return nil, err
	}

	p := &Page{
		URL:       baseURL,
		Title:     art.Title,
		Byline:    art.Byline,
		SiteName:  art.SiteName,
		Excerpt:   art.Excerpt,
		Lang:      art.Lang,
		Published: art.Published,
		Fetched:   r.opts.Fetched,
	}

	base, _ := url.Parse(baseURL)
	imgSink := r.imageSink(ctx, base, rw, p)

	if art.Node != nil {
		md, err := mdconv.Convert(art.Node, mdconv.Options{
			Base:         base,
			RewriteLink:  linkSink(rw.link),
			RewriteImage: imgSink,
		})
		if err != nil {
			return nil, err
		}
		p.Markdown = md
	}

	p.Links = collectLinks(art.Links, rw.link)
	p.WordCount = countWords(p.Markdown)
	p.ReadingMin = readingMinutes(p.WordCount)
	return p, nil
}

// linkSink adapts a rewrite's link function to mdconv's RewriteLink signature.
func linkSink(fn func(abs string) (string, bool)) func(string) string {
	if fn == nil {
		return nil
	}
	return func(abs string) string {
		target, _ := fn(abs)
		return target
	}
}

// collectLinks turns the extracted links into Page links, marking the internal
// ones via the rewrite predicate when a site build supplies it.
func collectLinks(links []extract.Link, fn func(abs string) (string, bool)) []Link {
	out := make([]Link, 0, len(links))
	for _, l := range links {
		lk := Link{Text: l.Text, URL: l.URL}
		if fn != nil {
			if _, internal := fn(l.URL); internal {
				lk.Internal = true
			}
		}
		out = append(out, lk)
	}
	return out
}

// ensureScheme defaults a scheme-less URL to https, so a bare host like
// "example.com/post" reads the same way the site command already accepts it.
// A URL that names its own scheme, or a protocol-relative "//host" one, is left
// to resolve on its own.
func ensureScheme(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "", strings.Contains(raw, "://"):
		return raw
	case strings.HasPrefix(raw, "//"):
		return "https:" + raw
	default:
		return "https://" + raw
	}
}

func countWords(md string) int { return len(strings.Fields(md)) }

// readingMinutes estimates reading time at 200 words per minute, at least one.
func readingMinutes(words int) int {
	m := words / 200
	if m < 1 {
		return 1
	}
	return m
}
