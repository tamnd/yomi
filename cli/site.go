package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tamnd/kage/urlx"
	"github.com/tamnd/yomi/yomi"
)

// siteFlags adds the crawl controls on top of the shared read flags.
type siteFlags struct {
	readFlags
	out         string
	format      string
	single      bool
	sitemap     bool
	resume      bool
	maxPages    int
	maxDepth    int
	workers     int
	subdomains  bool
	scopePrefix string
	exclude     []string
	noRobots    bool
}

func newSiteCmd() *cobra.Command {
	f := &siteFlags{}
	cmd := &cobra.Command{
		Use:   "site <url>",
		Short: "Crawl a whole site into Markdown, JSON, or JSONL",
		Long: "site fetches the seed URL, follows in-scope links, and reads every page.\n" +
			"The default output is a folder of Markdown files mirroring the URL paths,\n" +
			"with a SUMMARY.md table of contents; --single assembles one combined file,\n" +
			"and --format json|jsonl writes one structured dataset of every page instead.\n" +
			"A Markdown crawl is resumable: --resume continues an interrupted run, and\n" +
			"--sitemap seeds the crawl from the site's sitemap.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSite(cmd.Context(), args[0], f)
		},
	}
	f.register(cmd)
	fs := cmd.Flags()
	fs.StringVarP(&f.out, "out", "o", "", "output folder, or file with --single/--format (default: the host)")
	fs.StringVarP(&f.format, "format", "f", "md", "output format: md, json, or jsonl")
	fs.BoolVarP(&f.single, "single", "s", false, "assemble one Markdown file instead of a folder")
	fs.BoolVar(&f.sitemap, "sitemap", false, "seed the crawl from the site's sitemap")
	fs.BoolVar(&f.resume, "resume", false, "continue an interrupted crawl from its saved state")
	fs.IntVarP(&f.maxPages, "max-pages", "p", 0, "stop after N pages (0 = unlimited)")
	fs.IntVarP(&f.maxDepth, "max-depth", "d", 0, "link-follow depth cap (0 = unlimited)")
	fs.IntVar(&f.workers, "workers", 4, "concurrent page workers")
	fs.BoolVar(&f.subdomains, "subdomains", false, "treat subdomains of the seed host as in scope")
	fs.StringVar(&f.scopePrefix, "scope-prefix", "", "only crawl pages whose path starts with this prefix")
	fs.StringSliceVar(&f.exclude, "exclude", nil, "path prefixes to skip (repeatable)")
	fs.BoolVar(&f.noRobots, "no-robots", false, "ignore robots.txt (be careful and polite)")
	return cmd
}

func runSite(ctx context.Context, rawURL string, f *siteFlags) error {
	format, err := siteFormat(f.format)
	if err != nil {
		return err
	}
	opts, err := f.options()
	if err != nil {
		return err
	}
	opts.Format = format
	opts.Single = f.single
	opts.Sitemap = f.sitemap
	opts.Resume = f.resume
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
	opts.Out = siteOut(format, seed.Hostname(), f.out)

	fmt.Fprintln(os.Stderr, styleTitle.Render("yomi")+" reading "+styleAccent.Render(rawURL))

	res, err := yomi.Site(ctx, rawURL, opts)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if res != nil {
		printSiteSummary(res, format)
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, styleWarn.Render("interrupted")+styleDim.Render(" (re-run with --resume to continue)"))
	}
	return nil
}

// siteFormat validates the --format value for site.
func siteFormat(s string) (yomi.SiteFormat, error) {
	switch s {
	case "md", "json", "jsonl":
		return yomi.SiteFormat(s), nil
	default:
		return "", fmt.Errorf("invalid --format %q: want md, json, or jsonl", s)
	}
}

// siteOut resolves the output path. A Markdown crawl defaults to a folder (or a
// .md file with --single) named for the host; a dataset crawl defaults to one
// <host>.json/.jsonl file. An explicit --out always wins.
func siteOut(format yomi.SiteFormat, host, out string) string {
	if out != "" {
		return out
	}
	switch format {
	case yomi.SiteJSON:
		return host + ".json"
	case yomi.SiteJSONL:
		return host + ".jsonl"
	default:
		return host
	}
}

func printSiteSummary(res *yomi.SiteResult, format yomi.SiteFormat) {
	words := 0
	for _, p := range res.Pages {
		words += p.WordCount
	}
	kind := "folder"
	switch {
	case format == yomi.SiteJSON || format == yomi.SiteJSONL:
		kind = string(format)
	case res.Single:
		kind = "file"
	}
	fmt.Fprintln(os.Stderr, styleOK.Render("done")+" "+styleTitle.Render(res.OutPath)+" "+styleDim.Render("("+kind+")"))
	fmt.Fprintf(os.Stderr, "  %s %d   %s %d\n",
		styleAccent.Render("pages"), len(res.Pages),
		styleAccent.Render("words"), words)
}
