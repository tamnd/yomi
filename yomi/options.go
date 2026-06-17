package yomi

import (
	"time"

	"github.com/tamnd/kage/urlx"
)

// RenderMode decides whether yomi fetches a page with a plain HTTP client or a
// headless browser.
type RenderMode string

const (
	// RenderAuto fetches statically first and escalates to the browser only when
	// the static DOM looks empty or JavaScript-gated. It is the default.
	RenderAuto RenderMode = "auto"
	// RenderOn renders every page with the headless browser.
	RenderOn RenderMode = "on"
	// RenderOff never launches a browser; a JavaScript-only page comes back thin.
	RenderOff RenderMode = "off"
)

// ImagePolicy decides what happens to the images inside an article.
type ImagePolicy string

const (
	// ImageRemote leaves image targets as absolute URLs (default).
	ImageRemote ImagePolicy = "remote"
	// ImageDownload fetches each image next to the output and rewrites the target
	// to a relative path.
	ImageDownload ImagePolicy = "download"
	// ImageInline embeds each image as a data: URI so the Markdown is
	// self-contained.
	ImageInline ImagePolicy = "inline"
)

// LinkStyle decides how Markdown links are rendered.
type LinkStyle string

const (
	// LinkInline renders links inline: [text](url) (default).
	LinkInline LinkStyle = "inline"
	// LinkReference renders links as reference definitions collected at the end.
	LinkReference LinkStyle = "reference"
)

// Options govern one Read or Site run. The zero value is not valid; use
// Defaults and adjust.
type Options struct {
	Render       RenderMode  // auto (default), on, off
	Images       ImagePolicy // remote (default), download, inline
	Links        LinkStyle   // inline (default), reference
	FrontMatter  bool        // emit the YAML front-matter header
	TitleHeading bool        // keep the title as an in-body h1
	Wrap         int         // hard-wrap prose at this column; 0 means no wrap

	// Fetched is the timestamp written into the front-matter and the Page. It is
	// passed in rather than read from the clock so a run is reproducible.
	Fetched string

	UserAgent     string
	Timeout       time.Duration
	Scroll        bool   // drive lazy-load scrolling in render mode
	ChromeBin     string // explicit Chrome binary for the render backend
	ControlURL    string // attach to a running Chrome
	MaxImageBytes int64  // per-image cap for download/inline

	// Site-only crawl controls. They are reused from kage's scope model, so a
	// yomi crawl and a kage clone scope a site identically.
	Out      string           // output directory (site) or file (read)
	Single   bool             // assemble one Markdown file instead of a folder
	Scope    urlx.ScopeConfig // include-subdomains, scope-prefix, exclude-paths
	MaxPages int              // hard cap on pages crawled; 0 means unbounded
	MaxDepth int              // link depth from the seed; 0 means unbounded
	Workers  int              // crawl concurrency
	Resume   bool             // continue from a saved frontier
	Robots   bool             // respect robots.txt
	Sitemap  bool             // seed from the site's sitemap

	// Logf, when set, receives human-readable progress lines.
	Logf func(format string, args ...any)
}

// Defaults returns Options with the documented default for every field.
func Defaults() Options {
	return Options{
		Render:        RenderAuto,
		Images:        ImageRemote,
		Links:         LinkInline,
		FrontMatter:   true,
		Wrap:          0,
		UserAgent:     DefaultUserAgent,
		Timeout:       30 * time.Second,
		MaxImageBytes: 16 << 20,
		Workers:       4,
		MaxDepth:      0,
		Robots:        true,
		Logf:          func(string, ...any) {},
	}
}

// DefaultUserAgent is the User-Agent yomi presents to a server, a real browser
// string so a page returns the same HTML a person would see.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36 yomi"

// log calls the configured logger, tolerating a nil one.
func (o Options) log(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
	}
}
