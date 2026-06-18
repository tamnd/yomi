// Package site walks a website within a scope and hands each in-scope page to a
// visit callback. It owns only the frontier: scope, dedup, depth and page caps,
// and robots. Rendering and conversion belong to the caller, which returns the
// page's outbound links for the crawl to follow. URL scoping and normalisation
// reuse kage's urlx so a yomi crawl and a kage clone agree on what is in scope.
package site

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/tamnd/kage/robots"
	"github.com/tamnd/kage/urlx"
)

// Config controls one crawl.
type Config struct {
	Seed      *url.URL
	Scope     urlx.ScopeConfig
	MaxPages  int
	MaxDepth  int
	Workers   int
	Robots    bool
	Sitemap   bool // seed the frontier from the site's sitemap before walking links
	UserAgent string
	Timeout   time.Duration
	Logf      func(format string, args ...any)
}

// Visit is called once per in-scope page. It returns the outbound links found on
// that page (absolute or relative URLs as text); the crawler resolves, scopes,
// and enqueues the page-like ones. An error is logged and the page is skipped.
type Visit func(ctx context.Context, rawURL string, depth int) (links []string, err error)

// Crawl walks from cfg.Seed, calling visit for each in-scope page until the
// frontier drains, MaxPages is hit, or ctx is cancelled.
func Crawl(ctx context.Context, cfg Config, visit Visit) error {
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	c := &crawler{
		cfg:     cfg,
		visit:   visit,
		matcher: robots.AllowAll(),
		visited: map[string]bool{},
		jobs:    make(chan job),
	}
	if cfg.Robots {
		c.loadRobots(ctx)
	}

	var workers sync.WaitGroup
	for range max1(cfg.Workers) {
		workers.Go(func() {
			for j := range c.jobs {
				c.process(ctx, j)
				c.wg.Done()
			}
		})
	}

	c.enqueue(ctx, cfg.Seed, 0)
	if cfg.Sitemap {
		c.loadSitemap(ctx)
	}

	go func() {
		c.wg.Wait()
		close(c.jobs)
	}()
	workers.Wait()
	return ctx.Err()
}

type job struct {
	u     *url.URL
	depth int
}

type crawler struct {
	cfg     Config
	visit   Visit
	matcher *robots.Matcher

	mu       sync.Mutex
	visited  map[string]bool
	enqueued int
	sitemaps []string // Sitemap: URLs declared in robots.txt

	wg   sync.WaitGroup
	jobs chan job
}

func (c *crawler) process(ctx context.Context, j job) {
	if ctx.Err() != nil {
		return
	}
	if c.cfg.Robots && !c.matcher.Allowed(j.u.Path) {
		c.cfg.Logf("robots: skipping %s", j.u.String())
		return
	}
	links, err := c.visit(ctx, j.u.String(), j.depth)
	if err != nil {
		c.cfg.Logf("page error: %v", err)
		return
	}
	for _, raw := range links {
		u, err := urlx.Normalize(j.u, raw)
		if err != nil {
			continue
		}
		if urlx.InScope(c.cfg.Seed, u, c.cfg.Scope) && urlx.LikelyPage(u) {
			c.enqueue(ctx, u, j.depth+1)
		}
	}
}

// enqueue offers a URL to the frontier, honouring the visited set, the depth cap,
// and the page budget.
func (c *crawler) enqueue(ctx context.Context, u *url.URL, depth int) {
	if c.cfg.MaxDepth > 0 && depth > c.cfg.MaxDepth {
		return
	}
	key := urlx.Key(u)
	c.mu.Lock()
	if c.visited[key] {
		c.mu.Unlock()
		return
	}
	if c.cfg.MaxPages > 0 && c.enqueued >= c.cfg.MaxPages {
		c.mu.Unlock()
		return
	}
	c.visited[key] = true
	c.enqueued++
	c.mu.Unlock()

	c.wg.Add(1)
	go func() {
		select {
		case c.jobs <- job{u: u, depth: depth}:
		case <-ctx.Done():
			c.wg.Done()
		}
	}()
}

// loadRobots fetches and parses the site's robots.txt.
func (c *crawler) loadRobots(ctx context.Context) {
	robotsURL := c.cfg.Seed.Scheme + "://" + c.cfg.Seed.Host + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	client := &http.Client{Timeout: c.cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return
	}
	c.matcher = robots.Parse(string(data), "yomi")
	c.sitemaps = sitemapLines(string(data))
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
