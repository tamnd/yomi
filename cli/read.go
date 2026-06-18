package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tamnd/yomi/yomi"
)

func newReadCmd() *cobra.Command {
	f := &readFlags{}
	var out, format, base string
	cmd := &cobra.Command{
		Use:   "read <url | file | ->",
		Short: "Read one page into Markdown, JSON, or HTML",
		Long: "read fetches a single URL, extracts the main content, and prints it as\n" +
			"Markdown to stdout. The argument can also be a local .html file or - to read\n" +
			"HTML from stdin, so a page you already have converts without a fetch. With\n" +
			"--format it prints JSON, JSONL, or a self-contained HTML document instead,\n" +
			"and with --out it writes a file (downloaded images land in a sidecar).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRead(cmd.Context(), args[0], out, format, base, f)
		},
	}
	f.register(cmd)
	fs := cmd.Flags()
	fs.StringVarP(&out, "out", "o", "", "write to this file instead of stdout")
	fs.StringVarP(&format, "format", "f", "md", "output format: md, json, jsonl, or html")
	fs.StringVar(&base, "base", "", "base URL to resolve relative links for stdin or a local file")
	return cmd
}

func runRead(ctx context.Context, src, out, format, base string, f *readFlags) error {
	if !validReadFormat(format) {
		return fmt.Errorf("invalid --format %q: want md, json, jsonl, or html", format)
	}
	opts, err := f.options()
	if err != nil {
		return err
	}
	opts.Out = out

	p, err := readSource(ctx, src, base, opts)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	doc, err := renderRead(p, format, opts)
	if err != nil {
		return err
	}

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

// readSource reads the page from one of three input sources: standard input
// (the argument "-"), a local HTML file (a file:// URL or a path to an existing
// file), or a live URL (everything else). The stdin and file paths bypass the
// fetcher entirely and feed the bytes straight to the extractor.
func readSource(ctx context.Context, src, base string, opts yomi.Options) (*yomi.Page, error) {
	switch {
	case src == "-":
		body, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		return yomi.ReadHTML(ctx, body, base, opts)
	case isLocalHTML(src):
		path, fileURL := localFile(src)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if base == "" {
			base = fileURL
		}
		return yomi.ReadHTML(ctx, body, base, opts)
	default:
		return yomi.Read(ctx, src, opts)
	}
}

// isLocalHTML reports whether src names a local file rather than a URL to fetch:
// a file:// URL, or a scheme-less path that exists on disk. A bare host like
// example.com/post has no scheme and does not exist on disk, so it stays a fetch.
func isLocalHTML(src string) bool {
	if strings.HasPrefix(src, "file://") {
		return true
	}
	if strings.Contains(src, "://") {
		return false
	}
	info, err := os.Stat(src)
	return err == nil && !info.IsDir()
}

// localFile resolves a local source to the filesystem path to read and the
// file:// URL to use as the base for relative links.
func localFile(src string) (path, fileURL string) {
	if strings.HasPrefix(src, "file://") {
		if u, err := url.Parse(src); err == nil {
			return u.Path, src
		}
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		abs = src
	}
	return src, "file://" + filepath.ToSlash(abs)
}

// validReadFormat reports whether format is one read knows how to emit.
func validReadFormat(format string) bool {
	switch format {
	case "md", "json", "jsonl", "html":
		return true
	default:
		return false
	}
}

// renderRead turns a Page into the requested output: the Markdown document, a
// self-contained HTML document, or the Page record as indented JSON or one-line
// JSONL (both carrying the Markdown body, unlike the meta view).
func renderRead(p *yomi.Page, format string, opts yomi.Options) (string, error) {
	switch format {
	case "html":
		return yomi.StandaloneHTML(p)
	case "json":
		b, err := json.MarshalIndent(p, "", "  ")
		return string(b) + "\n", err
	case "jsonl":
		b, err := json.Marshal(p)
		return string(b) + "\n", err
	default:
		return yomi.Document(p, opts), nil
	}
}
