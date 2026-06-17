package yomi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const articleHTML = `<!doctype html>
<html lang="en">
<head>
  <title>Reading Test</title>
  <meta property="og:site_name" content="Test Site">
</head>
<body>
  <nav><a href="/">Home</a></nav>
  <article>
    <h1>Reading Test</h1>
    <p>This first paragraph is long enough that readability keeps the article
       block as the document's main content rather than the navigation.</p>
    <p>A second real paragraph adds more sentences so the extractor stays
       confident, and links to a <a href="/next">next page</a> inside the site.</p>
  </article>
  <footer>chrome</footer>
</body>
</html>`

// newArticleServer serves the article HTML for every path, so a read or a small
// crawl over it stays self-contained and needs no network or browser.
func newArticleServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML))
	}))
}

func TestReadStatic(t *testing.T) {
	srv := newArticleServer()
	defer srv.Close()

	opts := Defaults()
	opts.Render = RenderOff // never launch a browser
	opts.Fetched = "2026-06-17T00:00:00Z"

	p, err := Read(context.Background(), srv.URL, opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Reading Test" {
		t.Errorf("title = %q", p.Title)
	}
	if p.SiteName != "Test Site" {
		t.Errorf("site = %q", p.SiteName)
	}
	if !strings.Contains(p.Markdown, "first paragraph") {
		t.Errorf("body missing article text:\n%s", p.Markdown)
	}
	if strings.Contains(p.Markdown, "chrome") {
		t.Errorf("footer chrome leaked into body:\n%s", p.Markdown)
	}
	if p.WordCount == 0 || p.ReadingMin < 1 {
		t.Errorf("word/reading stats not computed: words=%d min=%d", p.WordCount, p.ReadingMin)
	}
	doc := Document(p, opts)
	if !strings.HasPrefix(doc, "---\n") {
		t.Errorf("front matter missing:\n%s", doc)
	}
}

func TestEnsureScheme(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example.com", "https://example.com"},
		{"example.com/post", "https://example.com/post"},
		{"http://example.com", "http://example.com"},
		{"https://example.com/x", "https://example.com/x"},
		{"//cdn.example.com/x", "https://cdn.example.com/x"},
		{"  example.com/p  ", "https://example.com/p"},
		{"", ""},
	}
	for _, c := range cases {
		if got := ensureScheme(c.in); got != c.want {
			t.Errorf("ensureScheme(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSiteFolder(t *testing.T) {
	srv := newArticleServer()
	defer srv.Close()

	dir := t.TempDir()
	opts := Defaults()
	opts.Render = RenderOff
	opts.Out = dir
	opts.MaxPages = 3
	opts.Robots = false
	opts.Workers = 2

	res, err := Site(context.Background(), srv.URL, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pages) == 0 {
		t.Fatalf("no pages read")
	}
	if res.Single {
		t.Errorf("expected folder output")
	}
}
