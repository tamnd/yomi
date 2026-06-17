package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tamnd/kage/urlx"
	"github.com/tamnd/yomi/yomi"
)

// packFlags adds the packaging and crawl controls on top of the shared read
// flags.
type packFlags struct {
	readFlags
	out         string
	format      string
	state       string
	refresh     bool
	maxAge      time.Duration
	noCompress  bool
	title       string
	description string
	language    string
	date        string
	maxPages    int
	maxDepth    int
	workers     int
	subdomains  bool
	scopePrefix string
	exclude     []string
	noRobots    bool
}

func newPackCmd() *cobra.Command {
	f := &packFlags{}
	cmd := &cobra.Command{
		Use:   "pack <url>",
		Short: "Crawl a site into a single SQLite database or ZIM archive",
		Long: "pack crawls a whole site and packages it into one file: a SQLite database\n" +
			"of pages, links, and images (the default), or a ZIM offline archive you can\n" +
			"open in Kiwix. The crawl is backed by the database, so it resumes where it\n" +
			"left off and a later run only fetches what is new. Re-fetch everything with\n" +
			"--refresh, or just the stale pages with --max-age.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// An explicit --format wins; otherwise a .zim or .db output name
			// picks the format, so -o site.zim just does the obvious thing.
			if !cmd.Flags().Changed("format") {
				if inferred, ok := formatFromExt(f.out); ok {
					f.format = inferred
				}
			}
			return runPack(cmd.Context(), args[0], f)
		},
	}
	f.register(cmd)
	fs := cmd.Flags()
	fs.StringVar(&f.format, "format", "sqlite", "output format: sqlite or zim")
	fs.StringVarP(&f.out, "out", "o", "", "output file (default: the host with .db or .zim)")
	fs.StringVar(&f.state, "state", "", "SQLite store path for a zim build (default: the output with .db)")
	fs.BoolVar(&f.refresh, "refresh", false, "re-fetch every page, ignoring what is already stored")
	fs.DurationVar(&f.maxAge, "max-age", 0, "re-fetch a stored page older than this (e.g. 24h; 0 = never)")
	fs.BoolVar(&f.noCompress, "no-compress", false, "zim: store every entry raw, with no compression")
	fs.StringVar(&f.title, "title", "", "zim: archive title (default: the home page title)")
	fs.StringVar(&f.description, "description", "", "zim: archive description")
	fs.StringVar(&f.language, "language", "eng", "zim: archive language as an ISO 639-3 code")
	fs.StringVar(&f.date, "date", time.Now().UTC().Format("2006-01-02"), "zim: archive date (YYYY-MM-DD)")
	fs.IntVarP(&f.maxPages, "max-pages", "p", 0, "stop after N pages (0 = unlimited)")
	fs.IntVarP(&f.maxDepth, "max-depth", "d", 0, "link-follow depth cap (0 = unlimited)")
	fs.IntVar(&f.workers, "workers", 4, "concurrent page workers")
	fs.BoolVar(&f.subdomains, "subdomains", false, "treat subdomains of the seed host as in scope")
	fs.StringVar(&f.scopePrefix, "scope-prefix", "", "only crawl pages whose path starts with this prefix")
	fs.StringSliceVar(&f.exclude, "exclude", nil, "path prefixes to skip (repeatable)")
	fs.BoolVar(&f.noRobots, "no-robots", false, "ignore robots.txt (be careful and polite)")
	return cmd
}

func runPack(ctx context.Context, rawURL string, f *packFlags) error {
	format, err := packFormat(f.format)
	if err != nil {
		return err
	}
	opts, err := f.options()
	if err != nil {
		return err
	}
	opts.MaxPages = f.maxPages
	opts.MaxDepth = f.maxDepth
	opts.Workers = f.workers
	opts.Robots = !f.noRobots
	opts.Scope = urlx.ScopeConfig{
		IncludeSubdomains: f.subdomains,
		ScopePrefix:       f.scopePrefix,
		ExcludePaths:      f.exclude,
	}

	seed, err := urlx.ParseSeed(rawURL)
	if err != nil {
		return err
	}
	out, state := packPaths(format, seed.Hostname(), f.out, f.state)

	if opts.Images == yomi.ImageDownload {
		fmt.Fprintln(os.Stderr, styleWarn.Render("note")+styleDim.Render(
			" --images download has no folder to write into when packing; use --images inline for a self-contained archive"))
	}

	popts := yomi.PackOptions{
		Options:     opts,
		Format:      format,
		Out:         out,
		State:       state,
		Refresh:     f.refresh,
		MaxAge:      f.maxAge,
		NoCompress:  f.noCompress,
		Title:       f.title,
		Description: f.description,
		Language:    f.language,
		Date:        f.date,
		Version:     Version,
	}

	fmt.Fprintln(os.Stderr, styleTitle.Render("yomi")+" packing "+styleAccent.Render(rawURL)+
		styleDim.Render(" → "+string(format)))

	res, err := yomi.Pack(ctx, rawURL, popts)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if res != nil {
		if info, statErr := os.Stat(res.OutPath); statErr == nil {
			res.Bytes = info.Size()
		}
		printPackSummary(res)
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, styleWarn.Render("interrupted")+styleDim.Render(" (re-run to resume)"))
	}
	return nil
}

// formatFromExt maps a known output extension to a format, so a user who writes
// -o site.zim or -o site.db gets that format without also passing --format.
func formatFromExt(out string) (string, bool) {
	switch strings.ToLower(filepath.Ext(out)) {
	case ".zim":
		return "zim", true
	case ".db", ".sqlite", ".sqlite3":
		return "sqlite", true
	default:
		return "", false
	}
}

// packFormat validates the --format value.
func packFormat(s string) (yomi.PackFormat, error) {
	switch s {
	case "sqlite", "zim":
		return yomi.PackFormat(s), nil
	default:
		return "", fmt.Errorf("invalid --format %q: want sqlite or zim", s)
	}
}

// packPaths resolves the artifact path and the backing store path from the
// format, the host, and any explicit --out/--state. For sqlite the store is the
// output; for zim the store is a sidecar database next to the archive.
func packPaths(format yomi.PackFormat, host, out, state string) (artifact, store string) {
	if format == yomi.PackSQLite {
		if out == "" {
			out = host + ".db"
		}
		return out, out
	}
	if out == "" {
		out = host + ".zim"
	}
	if state == "" {
		state = strings.TrimSuffix(out, filepath.Ext(out)) + ".db"
	}
	return out, state
}

func printPackSummary(res *yomi.PackResult) {
	fmt.Fprintln(os.Stderr, styleOK.Render("done")+" "+styleTitle.Render(res.OutPath)+" "+
		styleDim.Render("("+string(res.Format)+", "+humanBytes(res.Bytes)+")"))
	fmt.Fprintf(os.Stderr, "  %s %d   %s %d   %s %d   %s %d\n",
		styleAccent.Render("pages"), res.Pages,
		styleAccent.Render("new"), res.Added,
		styleAccent.Render("kept"), res.Skipped,
		styleAccent.Render("words"), res.Words)
	if res.Format == yomi.PackZIM {
		fmt.Fprintln(os.Stderr, styleDim.Render("  store "+res.StorePath+" kept for the next incremental run"))
	}
}

// humanBytes renders a byte count as a short human-readable size.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
