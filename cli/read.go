package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tamnd/yomi/yomi"
)

func newReadCmd() *cobra.Command {
	f := &readFlags{}
	var out string
	cmd := &cobra.Command{
		Use:   "read <url>",
		Short: "Read one page into Markdown",
		Long: "read fetches a single URL, extracts the main content, and prints it as\n" +
			"Markdown to stdout. With --out it writes a file instead, and downloaded\n" +
			"images land in a sidecar next to it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRead(cmd.Context(), args[0], out, f)
		},
	}
	f.register(cmd)
	cmd.Flags().StringVarP(&out, "out", "o", "", "write Markdown to this file instead of stdout")
	return cmd
}

func runRead(ctx context.Context, url, out string, f *readFlags) error {
	opts, err := f.options()
	if err != nil {
		return err
	}
	opts.Out = out

	p, err := yomi.Read(ctx, url, opts)
	if err != nil {
		return fmt.Errorf("read %s: %w", url, err)
	}

	doc := yomi.Document(p, opts)
	if out == "" {
		fmt.Print(doc)
		if len(doc) > 0 && doc[len(doc)-1] != '\n' {
			fmt.Println()
		}
		return nil
	}
	if dir := filepath.Dir(out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(out, []byte(doc), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s  %s %d words, %d min\n",
		styleOK.Render("read"), styleTitle.Render(out),
		styleDim.Render("·"), p.WordCount, p.ReadingMin)
	return nil
}
