package yomi

import (
	"context"
	"path/filepath"
	"strings"
)

// Read fetches one page and returns it as a Page. It builds and tears down its
// own fetcher, so for many URLs prefer ReadAll, which shares one browser pool.
func Read(ctx context.Context, rawURL string, opts Options) (*Page, error) {
	r := newReader(opts)
	defer func() { _ = r.close() }()
	return r.read(ctx, rawURL, standaloneRewrite(opts))
}

// ReadHTML reads a page from an in-memory HTML body instead of fetching it,
// resolving relative links and images against baseURL. It is the entry point for
// reading HTML piped on stdin or held in a local file, where no network request,
// render decision, or robots check applies. baseURL may be empty, in which case
// relative links are left relative.
func ReadHTML(ctx context.Context, body []byte, baseURL string, opts Options) (*Page, error) {
	r := newReader(opts)
	defer func() { _ = r.close() }()
	return r.readHTML(ctx, body, baseURL, standaloneRewrite(opts))
}

// ReadAll reads several URLs through one shared fetcher and downloader, so the
// browser launches at most once. A URL that fails yields a nil entry and its
// error in the parallel errs slice; the others still return.
func ReadAll(ctx context.Context, urls []string, opts Options) (pages []*Page, errs []error) {
	r := newReader(opts)
	defer func() { _ = r.close() }()
	rw := standaloneRewrite(opts)
	for _, u := range urls {
		p, err := r.read(ctx, u, rw)
		pages = append(pages, p)
		errs = append(errs, err)
	}
	return pages, errs
}

// standaloneRewrite is the rewiring for a single-page read: links stay absolute,
// and downloaded images land in a sidecar next to the output file (when one is
// named).
func standaloneRewrite(opts Options) rewrite {
	rw := rewrite{}
	if opts.Images == ImageDownload && opts.Out != "" {
		base := strings.TrimSuffix(filepath.Base(opts.Out), filepath.Ext(opts.Out))
		sidecar := base + ".media"
		rw.imageDir = filepath.Join(filepath.Dir(opts.Out), sidecar)
		rw.imageRel = sidecar
	}
	return rw
}
