package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tamnd/yomi/yomi"
)

func newMetaCmd() *cobra.Command {
	f := &readFlags{}
	cmd := &cobra.Command{
		Use:   "meta <url>",
		Short: "Print a page's metadata as JSON",
		Long: "meta reads a page and prints its record as JSON without the Markdown body:\n" +
			"title, byline, site, language, word count, reading time, and the links and\n" +
			"images found. Useful for scripting and for inspecting what yomi extracted.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMeta(cmd.Context(), args[0], f)
		},
	}
	f.register(cmd)
	return cmd
}

func runMeta(ctx context.Context, url string, f *readFlags) error {
	opts, err := f.options()
	if err != nil {
		return err
	}
	p, err := yomi.Read(ctx, url, opts)
	if err != nil {
		return fmt.Errorf("read %s: %w", url, err)
	}
	// Drop the body so meta stays a metadata view.
	p.Markdown = ""
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}
