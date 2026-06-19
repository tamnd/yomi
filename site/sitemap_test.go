package site

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"

	"github.com/tamnd/kage/urlx"
)

func TestSitemapLines(t *testing.T) {
	robotsTxt := "User-agent: *\nDisallow: /private\nSitemap: https://ex.com/sitemap.xml\n" +
		"sitemap:https://ex.com/news.xml\n  Sitemap:   https://ex.com/extra.xml  \nNot-a-sitemap: x\n"
	got := sitemapLines(robotsTxt)
	want := []string{"https://ex.com/sitemap.xml", "https://ex.com/news.xml", "https://ex.com/extra.xml"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSitemapDocParsesBothForms(t *testing.T) {
	urlset := `<?xml version="1.0"?><urlset><url><loc>https://ex.com/a</loc></url>` +
		`<url><loc>https://ex.com/b</loc></url></urlset>`
	index := `<?xml version="1.0"?><sitemapindex><sitemap><loc>https://ex.com/s1.xml</loc></sitemap></sitemapindex>`

	c := &crawler{cfg: Config{Logf: func(string, ...any) {}}}
	urls, idx := parseDoc(t, c, urlset)
	if len(urls) != 2 || len(idx) != 0 {
		t.Errorf("urlset parse = %v / %v", urls, idx)
	}
	urls, idx = parseDoc(t, c, index)
	if len(urls) != 0 || len(idx) != 1 {
		t.Errorf("sitemapindex parse = %v / %v", urls, idx)
	}
}

// parseDoc serves doc from a test server and runs fetchSitemap against it.
func parseDoc(t *testing.T, c *crawler, doc string) (urls, indexes []string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(doc))
	}))
	defer srv.Close()
	return c.fetchSitemap(context.Background(), srv.Client(), srv.URL)
}

func TestIsGzip(t *testing.T) {
	if !isGzip([]byte{0x1f, 0x8b, 0x08}) {
		t.Error("gzip magic not detected")
	}
	if isGzip([]byte("<?xml")) {
		t.Error("plain text flagged as gzip")
	}
}

func TestGunzipRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte("hello"))
	_ = zw.Close()
	out, err := gunzip(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello" {
		t.Errorf("gunzip = %q", out)
	}
}

// TestLoadSitemapSeedsFrontier serves a sitemap index that fans out to a urlset
// and checks the crawler enqueues the in-scope page URLs it lists.
func TestLoadSitemapSeedsFrontier(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><sitemapindex><sitemap><loc>` +
			hostBase + `/pages.xml</loc></sitemap></sitemapindex>`))
	})
	mux.HandleFunc("/pages.xml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset>` +
			`<url><loc>` + hostBase + `/a</loc></url>` +
			`<url><loc>` + hostBase + `/b</loc></url>` +
			`<url><loc>https://elsewhere.example/c</loc></url>` +
			`</urlset>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	hostBase = srv.URL // the sitemap docs use the live test-server origin

	seed, _ := url.Parse(srv.URL + "/")
	var mu sync.Mutex
	var enqueued []string
	c := &crawler{
		cfg: Config{
			Seed:    seed,
			Timeout: 0,
			Logf:    func(string, ...any) {},
		},
		visited: map[string]bool{},
		jobs:    make(chan job, 16),
	}
	// Capture enqueues by draining the channel as the loader fills it.
	go func() {
		for j := range c.jobs {
			mu.Lock()
			enqueued = append(enqueued, j.u.Path)
			mu.Unlock()
			c.wg.Done()
		}
	}()

	c.loadSitemap(context.Background())
	c.wg.Wait()
	close(c.jobs)

	mu.Lock()
	got := append([]string(nil), enqueued...)
	mu.Unlock()
	sort.Strings(got)

	want := []string{"/a", "/b"}
	if len(got) != len(want) {
		t.Fatalf("enqueued %v, want %v (off-site /c must be dropped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("enqueued %v, want %v", got, want)
		}
	}
	// Belt and braces: the off-site URL is genuinely out of scope.
	off, _ := url.Parse("https://elsewhere.example/c")
	if urlx.InScope(seed, off, c.cfg.Scope) {
		t.Error("off-site URL counted in scope")
	}
}

// hostBase is the test-server origin the sitemap documents above point at; it is
// set inside the test once the server is up.
var hostBase string
