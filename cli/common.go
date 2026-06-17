package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tamnd/yomi/yomi"
)

// readFlags holds the flags shared by every command that reads a page: how to
// fetch it, what to do with images and links, and the front-matter shape. The
// site command embeds it and adds the crawl flags.
type readFlags struct {
	render     string
	images     string
	links      string
	noFront    bool
	titleH1    bool
	wrap       int
	timeout    time.Duration
	scroll     bool
	userAgent  string
	chromeBin  string
	controlURL string
	maxImageMB int64
	quiet      bool
}

// register binds the shared read flags onto a command.
func (f *readFlags) register(cmd *cobra.Command) {
	fs := cmd.Flags()
	fs.StringVar(&f.render, "render", "auto", "JavaScript rendering: auto, on, or off")
	fs.StringVar(&f.images, "images", "remote", "image policy: remote, download, or inline")
	fs.StringVar(&f.links, "links", "inline", "link style: inline or reference")
	fs.BoolVar(&f.noFront, "no-front-matter", false, "omit the YAML front-matter header")
	fs.BoolVar(&f.titleH1, "title-heading", false, "keep the title as an h1 at the top of the body")
	fs.IntVar(&f.wrap, "wrap", 0, "hard-wrap prose at this column (0 = no wrap)")
	fs.DurationVar(&f.timeout, "timeout", 30*time.Second, "per-request timeout")
	fs.BoolVar(&f.scroll, "scroll", false, "auto-scroll each page to trigger lazy loading (render mode)")
	fs.StringVar(&f.userAgent, "user-agent", yomi.DefaultUserAgent, "User-Agent for page and asset fetches")
	fs.StringVar(&f.chromeBin, "chrome", "", "path to the Chrome/Chromium binary")
	fs.StringVar(&f.controlURL, "control-url", "", "attach to an existing Chrome DevTools endpoint")
	fs.Int64Var(&f.maxImageMB, "max-image-mb", 16, "skip images larger than N MB (download/inline)")
	fs.BoolVarP(&f.quiet, "quiet", "q", false, "suppress progress lines")
}

// options folds the shared flags into a yomi.Options, validating the enum-like
// strings. The caller fills in the per-command fields (Out, Single, crawl caps)
// afterwards.
func (f *readFlags) options() (yomi.Options, error) {
	o := yomi.Defaults()

	switch f.render {
	case "auto", "on", "off":
		o.Render = yomi.RenderMode(f.render)
	default:
		return o, fmt.Errorf("invalid --render %q: want auto, on, or off", f.render)
	}
	switch f.images {
	case "remote", "download", "inline":
		o.Images = yomi.ImagePolicy(f.images)
	default:
		return o, fmt.Errorf("invalid --images %q: want remote, download, or inline", f.images)
	}
	switch f.links {
	case "inline", "reference":
		o.Links = yomi.LinkStyle(f.links)
	default:
		return o, fmt.Errorf("invalid --links %q: want inline or reference", f.links)
	}

	o.FrontMatter = !f.noFront
	o.TitleHeading = f.titleH1
	o.Wrap = f.wrap
	o.Timeout = f.timeout
	o.Scroll = f.scroll
	o.UserAgent = f.userAgent
	o.ChromeBin = f.chromeBin
	o.ControlURL = f.controlURL
	o.MaxImageBytes = f.maxImageMB << 20
	o.Fetched = nowStamp()

	if !f.quiet {
		o.Logf = func(format string, args ...any) {
			fmt.Fprintln(os.Stderr, styleDim.Render(fmt.Sprintf(format, args...)))
		}
	}
	return o, nil
}

// nowStamp returns the current time as an RFC 3339 string for the fetched field,
// or the empty string when the clock is unavailable.
func nowStamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
