package site

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"

	"github.com/tamnd/kage/urlx"
)

// maxSitemaps caps how many sitemap documents one crawl fetches, so a pathological
// nest of sitemap indexes cannot fan out without bound.
const maxSitemaps = 50

// loadSitemap seeds the frontier from the site's sitemap before the link walk
// begins: it fetches the conventional /sitemap.xml plus any Sitemap: URLs the
// robots.txt declared, parses the urlset and sitemapindex forms (following an
// index one level), and enqueues every in-scope, page-like URL at depth zero.
// Any failure is silent: a site without a sitemap simply falls back to the link
// walk, so the option is always safe to pass.
func (c *crawler) loadSitemap(ctx context.Context) {
	seeds := append([]string{
		c.cfg.Seed.Scheme + "://" + c.cfg.Seed.Host + "/sitemap.xml",
	}, c.sitemaps...)

	client := &http.Client{Timeout: c.cfg.Timeout}
	seen := map[string]bool{}
	fetched := 0

	// queue is the list of sitemap URLs still to read; an index expands into it.
	var queue []string
	queue = append(queue, seeds...)
	for len(queue) > 0 && fetched < maxSitemaps {
		smURL := queue[0]
		queue = queue[1:]
		if smURL == "" || seen[smURL] {
			continue
		}
		seen[smURL] = true
		fetched++

		urls, indexes := c.fetchSitemap(ctx, client, smURL)
		queue = append(queue, indexes...)
		for _, raw := range urls {
			u, err := urlx.Normalize(c.cfg.Seed, raw)
			if err != nil {
				continue
			}
			if urlx.InScope(c.cfg.Seed, u, c.cfg.Scope) && urlx.LikelyPage(u) {
				c.enqueue(ctx, u, 0)
			}
		}
	}
}

// fetchSitemap reads one sitemap document and returns the page URLs from a
// urlset and the nested sitemap URLs from a sitemapindex. A .gz document, or one
// whose bytes are gzip-framed, is transparently decompressed.
func (c *crawler) fetchSitemap(ctx context.Context, client *http.Client, smURL string) (urls, indexes []string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, smURL, nil)
	if err != nil {
		return nil, nil
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, nil
	}
	if strings.HasSuffix(smURL, ".gz") || isGzip(data) {
		if gz, derr := gunzip(data); derr == nil {
			data = gz
		}
	}

	var doc sitemapDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, nil
	}
	c.cfg.Logf("sitemap %s: %d urls, %d sub-sitemaps", smURL, len(doc.URLs), len(doc.Maps))
	for _, l := range doc.URLs {
		if loc := strings.TrimSpace(l.Loc); loc != "" {
			urls = append(urls, loc)
		}
	}
	for _, l := range doc.Maps {
		if loc := strings.TrimSpace(l.Loc); loc != "" {
			indexes = append(indexes, loc)
		}
	}
	return urls, indexes
}

// sitemapDoc matches both sitemap forms at once: encoding/xml ignores the root
// element name, so a urlset's <url><loc> and a sitemapindex's <sitemap><loc>
// both bind from one struct.
type sitemapDoc struct {
	URLs []sitemapLoc `xml:"url"`
	Maps []sitemapLoc `xml:"sitemap"`
}

type sitemapLoc struct {
	Loc string `xml:"loc"`
}

// sitemapLines pulls the Sitemap: URLs a robots.txt declares, which a site uses
// to point at a sitemap that does not sit at the conventional /sitemap.xml.
func sitemapLines(robotsTxt string) []string {
	var out []string
	for _, line := range strings.Split(robotsTxt, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 8 || !strings.EqualFold(line[:8], "sitemap:") {
			continue
		}
		if v := strings.TrimSpace(line[8:]); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func isGzip(b []byte) bool { return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b }

func gunzip(b []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	return io.ReadAll(io.LimitReader(zr, 64<<20))
}
