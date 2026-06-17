package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tamnd/yomi/yomi"
)

func newLinksCmd() *cobra.Command {
	f := &readFlags{}
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "links <url>",
		Short: "List the links in a page's article body",
		Long: "links reads a page and prints the outbound links found in its main content,\n" +
			"one URL per line, or as JSON with --json. Navigation, footers, and other\n" +
			"chrome are already stripped, so this is the article's real link list.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinks(cmd.Context(), args[0], asJSON, f)
		},
	}
	f.register(cmd)
	cmd.Flags().BoolVar(&asJSON, "json", false, "print links as JSON instead of one URL per line")
	return cmd
}

func runLinks(ctx context.Context, url string, asJSON bool, f *readFlags) error {
	opts, err := f.options()
	if err != nil {
		return err
	}
	p, err := yomi.Read(ctx, url, opts)
	if err != nil {
		return fmt.Errorf("read %s: %w", url, err)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(p.Links)
	}
	for _, l := range p.Links {
		fmt.Println(l.URL)
	}
	return nil
}
