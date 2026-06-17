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
	single      bool
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
		Short: "Crawl a whole site into Markdown",
		Long: "site fetches the seed URL, follows in-scope links, and reads every page to\n" +
			"Markdown. The default output is a folder of files mirroring the URL paths,\n" +
			"with a SUMMARY.md table of contents; --single assembles one combined file\n" +
			"with a table of contents and per-page sections instead.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSite(cmd.Context(), args[0], f)
		},
	}
	f.register(cmd)
	fs := cmd.Flags()
	fs.StringVarP(&f.out, "out", "o", "", "output folder, or file with --single (default: the host)")
	fs.BoolVarP(&f.single, "single", "s", false, "assemble one Markdown file instead of a folder")
	fs.IntVarP(&f.maxPages, "max-pages", "p", 0, "stop after N pages (0 = unlimited)")
	fs.IntVarP(&f.maxDepth, "max-depth", "d", 0, "link-follow depth cap (0 = unlimited)")
	fs.IntVar(&f.workers, "workers", 4, "concurrent page workers")
	fs.BoolVar(&f.subdomains, "subdomains", false, "treat subdomains of the seed host as in scope")
	fs.StringVar(&f.scopePrefix, "scope-prefix", "", "only crawl pages whose path starts with this prefix")
	fs.StringSliceVar(&f.exclude, "exclude", nil, "path prefixes to skip (repeatable)")
	fs.BoolVar(&f.noRobots, "no-robots", false, "ignore robots.txt (be careful and polite)")
	return cmd
}

func runSite(ctx context.Context, url string, f *siteFlags) error {
	opts, err := f.options()
	if err != nil {
		return err
	}
	opts.Out = f.out
	opts.Single = f.single
	opts.MaxPages = f.maxPages
	opts.MaxDepth = f.maxDepth
	opts.Workers = f.workers
	opts.Robots = !f.noRobots
	opts.Scope = urlx.ScopeConfig{
		IncludeSubdomains: f.subdomains,
		ScopePrefix:       f.scopePrefix,
		ExcludePaths:      f.exclude,
	}

	fmt.Fprintln(os.Stderr, styleTitle.Render("yomi")+" reading "+styleAccent.Render(url))

	res, err := yomi.Site(ctx, url, opts)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if res != nil {
		printSiteSummary(res)
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, styleWarn.Render("interrupted"))
	}
	return nil
}

func printSiteSummary(res *yomi.SiteResult) {
	words := 0
	for _, p := range res.Pages {
		words += p.WordCount
	}
	kind := "folder"
	if res.Single {
		kind = "file"
	}
	fmt.Fprintln(os.Stderr, styleOK.Render("done")+" "+styleTitle.Render(res.OutPath)+" "+styleDim.Render("("+kind+")"))
	fmt.Fprintf(os.Stderr, "  %s %d   %s %d\n",
		styleAccent.Render("pages"), len(res.Pages),
		styleAccent.Render("words"), words)
}
