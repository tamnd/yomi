// Package cli wires yomi's command surface: the cobra tree, the global flags,
// and the fang-rendered help and errors. The actual reading, extraction, and
// Markdown conversion live in the yomi, fetch, extract, mdconv, and site
// packages; this layer only parses flags and prints progress.
package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Execute builds the root command and runs it through fang. main passes the
// signal-aware context so Ctrl-C cancels the in-flight read or crawl. It returns
// the process exit code.
func Execute(ctx context.Context) int {
	root := newRoot()
	opts := []fang.Option{
		fang.WithVersion(Version),
	}
	if err := fang.Execute(ctx, root, opts...); err != nil {
		return 1
	}
	return 0
}

// newRoot assembles the command tree.
func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "yomi",
		Short: "Read any web page, or a whole website, into clean Markdown",
		Long: "yomi (読み, \"reading\") fetches a page, rendering JavaScript when the page\n" +
			"needs it, strips the navigation and the clutter, and converts the main\n" +
			"content to Markdown. Point it at one URL for one document, or crawl a whole\n" +
			"site into a folder of Markdown or a single combined file.",
		Version:       fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newReadCmd())
	root.AddCommand(newSiteCmd())
	root.AddCommand(newMetaCmd())
	root.AddCommand(newLinksCmd())
	root.AddCommand(newServeCmd())
	return root
}
