// Package fetch gets the bytes of a page. It has two backends: a plain HTTP
// client (the fast default) and a headless-browser renderer that reuses kage's
// Chrome pool. The auto policy fetches statically first and escalates to the
// browser only when the static DOM looks empty or JavaScript-gated.
package fetch

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/kage/browser"
	"golang.org/x/net/html"
)

// Response is the outcome of fetching one URL.
type Response struct {
	URL      string // final URL after redirects
	Body     []byte // decoded to UTF-8
	MIME     string // sniffed or declared content type
	Rendered bool   // produced by the headless browser
}

// Mode mirrors yomi's render mode without importing the yomi package, keeping
// fetch a leaf.
type Mode string

const (
	Auto Mode = "auto"
	On   Mode = "on"
	Off  Mode = "off"
)

// Options configure a Fetcher.
type Options struct {
	Mode       Mode
	UserAgent  string
	Timeout    time.Duration
	Scroll     bool
	ChromeBin  string
	ControlURL string
	// Logf, when set, receives a line each time auto escalates to the browser.
	Logf func(format string, args ...any)
}

// Fetcher fetches pages with a static client, a renderer, or static-first auto
// escalation. It is safe for concurrent use; the browser pool it owns launches
// Chrome once and is shared across every Fetch.
type Fetcher struct {
	opts   Options
	static *staticFetcher

	mu   sync.Mutex
	pool *browser.Pool // created lazily on first render
}

// New builds a Fetcher. Chrome is not launched until a render is actually
// needed.
func New(opts Options) *Fetcher {
	if opts.Logf == nil {
		opts.Logf = func(string, ...any) {}
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "yomi"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	return &Fetcher{opts: opts, static: newStatic(opts.UserAgent, opts.Timeout)}
}

// Fetch returns the bytes of url under the configured mode.
func (f *Fetcher) Fetch(ctx context.Context, url string) (*Response, error) {
	switch f.opts.Mode {
	case Off:
		return f.static.fetch(ctx, url)
	case On:
		return f.render(ctx, url)
	default: // Auto
		res, err := f.static.fetch(ctx, url)
		if err != nil {
			return nil, err
		}
		if reason, gated := looksJSGated(res.Body); gated {
			f.opts.Logf("render: %s, escalating to browser: %s", reason, url)
			if r, rerr := f.render(ctx, url); rerr == nil {
				return r, nil
			}
			// The browser failed; the static body is the best we have.
		}
		return res, nil
	}
}

// render fetches url through the shared headless-browser pool, creating it on
// first use.
func (f *Fetcher) render(ctx context.Context, url string) (*Response, error) {
	f.mu.Lock()
	if f.pool == nil {
		f.pool = browser.New(browser.Options{
			Headless:      true,
			Workers:       1,
			Settle:        time.Second,
			RenderTimeout: f.opts.Timeout,
			Scroll:        f.opts.Scroll,
			ChromeBin:     f.opts.ChromeBin,
			ControlURL:    f.opts.ControlURL,
		})
	}
	pool := f.pool
	f.mu.Unlock()

	r, err := pool.Render(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Response{URL: r.FinalURL, Body: []byte(r.HTML), MIME: "text/html", Rendered: true}, nil
}

// Close shuts down the browser pool if one was started.
func (f *Fetcher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pool != nil {
		err := f.pool.Close()
		f.pool = nil
		return err
	}
	return nil
}

// jsGateWordThreshold is the body word count below which auto mode suspects a
// page is not really server-rendered.
const jsGateWordThreshold = 25

// looksJSGated reports whether a statically fetched body looks like it needs a
// browser: it carries an empty single-page-app mount point, a <noscript> that
// says JavaScript is required, or almost no visible text. It returns a short
// reason for the log when it does.
func looksJSGated(body []byte) (string, bool) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return "", false
	}
	var words, mount int
	var noscriptNeedsJS bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style":
				return // do not count script or style text as content
			case "noscript":
				if strings.Contains(strings.ToLower(textOf(n)), "javascript") {
					noscriptNeedsJS = true
				}
				return
			case "div", "app-root", "main":
				if id := attr(n, "id"); id == "root" || id == "__next" || id == "app" {
					if textLen(n) < 20 {
						mount++
					}
				}
			}
		}
		if n.Type == html.TextNode {
			words += len(strings.Fields(n.Data))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	switch {
	case mount > 0 && words < jsGateWordThreshold:
		return "empty single-page-app mount point", true
	case noscriptNeedsJS && words < jsGateWordThreshold:
		return "page says JavaScript is required", true
	case words < jsGateWordThreshold:
		return "static body had almost no text", true
	}
	return "", false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textLen(n *html.Node) int { return len(strings.TrimSpace(textOf(n))) }

func textOf(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}
