package yomi

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/kage/zim"
)

// newSiteServer serves a tiny three-page site whose pages link to each other, so
// a pack crawl discovers and stores all of them without a network or browser.
func newSiteServer() *httptest.Server {
	page := func(title, body, links string) string {
		return fmt.Sprintf(`<!doctype html><html lang="en"><head><title>%s</title>
<meta property="og:site_name" content="Pack Site"></head><body>
<nav><a href="/">Home</a></nav>
<article><h1>%s</h1>
<p>%s This paragraph is long enough that readability keeps the article body as
the main content of the document instead of treating it as boilerplate.</p>
<p>Another sentence or two of real prose keeps the extractor confident, and the
page links onward to %s within the same site.</p>
</article><footer>site chrome footer</footer></body></html>`, title, title, body, links)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(page("Home", "Welcome to the home page.",
				`<a href="/alpha">Alpha</a> and <a href="/beta">Beta</a>`)))
		case "/alpha":
			_, _ = w.Write([]byte(page("Alpha", "Alpha is the first essay.",
				`<a href="/beta">Beta</a> and <a href="https://external.example/x">an outside link</a>`)))
		case "/beta":
			_, _ = w.Write([]byte(page("Beta", "Beta is the second essay.",
				`<a href="/">Home</a> and <a href="/alpha-alias">Alpha again</a>`)))
		case "/alpha-alias":
			// A second URL that redirects onto an already-crawled page, so the
			// crawler only learns it is a duplicate after following the redirect.
			http.Redirect(w, r, "/alpha", http.StatusMovedPermanently)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func packDefaults(out, state string, format PackFormat) PackOptions {
	o := Defaults()
	o.Render = RenderOff
	o.Fetched = "2026-06-17T00:00:00Z"
	o.Workers = 2
	o.Robots = false
	return PackOptions{Options: o, Format: format, Out: out, State: state, Language: "eng", Date: "2026-06-17"}
}

func TestPackSQLite(t *testing.T) {
	srv := newSiteServer()
	defer srv.Close()
	db := filepath.Join(t.TempDir(), "site.db")

	res, err := Pack(context.Background(), srv.URL, packDefaults(db, db, PackSQLite))
	if err != nil {
		t.Fatal(err)
	}
	// Three real pages; the /alpha-alias redirect collapses onto /alpha rather
	// than landing as a fourth row.
	if res.Pages != 3 {
		t.Fatalf("pages = %d, want 3", res.Pages)
	}
	if res.Added != 3 {
		t.Errorf("added = %d, want 3 (the redirect alias must not add a page)", res.Added)
	}
	if res.OutPath != db {
		t.Errorf("out = %q, want the db itself", res.OutPath)
	}

	// Reopen the store and confirm the structured tables hold the crawl.
	st, err := openStore(db)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.close() }()
	pages, err := st.allPages()
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]bool{}
	for _, p := range pages {
		titles[p.Title] = true
	}
	for _, want := range []string{"Home", "Alpha", "Beta"} {
		if !titles[want] {
			t.Errorf("missing page %q in store", want)
		}
	}
	if host, _ := st.getMeta("host"); host == "" {
		t.Error("meta host not recorded")
	}
}

func TestPackResumeSkips(t *testing.T) {
	srv := newSiteServer()
	defer srv.Close()
	db := filepath.Join(t.TempDir(), "site.db")
	opts := packDefaults(db, db, PackSQLite)

	if _, err := Pack(context.Background(), srv.URL, opts); err != nil {
		t.Fatal(err)
	}
	// Second run over the same store re-fetches nothing.
	res, err := Pack(context.Background(), srv.URL, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 0 {
		t.Errorf("added = %d on resume, want 0 (nothing should be re-fetched into the store)", res.Added)
	}
	if res.Skipped < 3 {
		t.Errorf("skipped = %d on resume, want at least the 3 stored pages", res.Skipped)
	}

	// With --refresh, the same run re-fetches every page.
	opts.Refresh = true
	res, err = Pack(context.Background(), srv.URL, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 3 {
		t.Errorf("added = %d with refresh, want 3", res.Added)
	}
}

func TestPackZIM(t *testing.T) {
	srv := newSiteServer()
	defer srv.Close()
	dir := t.TempDir()
	out := filepath.Join(dir, "site.zim")
	state := filepath.Join(dir, "site.db")

	res, err := Pack(context.Background(), srv.URL, packDefaults(out, state, PackZIM))
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes == 0 {
		t.Error("zim size not reported")
	}

	r, err := zim.Open(out)
	if err != nil {
		t.Fatalf("open zim: %v", err)
	}
	defer func() { _ = r.Close() }()

	// The main page is the generated contents page and lists the articles.
	ns, url, ok := r.MainPageRef()
	if !ok || ns != zim.NamespaceContent {
		t.Fatalf("main page ref = %c/%s ok=%v", ns, url, ok)
	}
	main, err := r.MainPage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main.Data), "yomi-toc") {
		t.Error("contents page missing the table of contents")
	}

	// An article entry is a self-contained HTML document.
	home, err := r.Get(zim.NamespaceContent, "index.html")
	if err != nil {
		t.Fatalf("get index.html: %v", err)
	}
	body := string(home.Data)
	if !strings.Contains(body, "<h1>") || !strings.Contains(body, ">source<") {
		t.Errorf("article HTML not well formed:\n%s", body[:min(len(body), 400)])
	}
}

// TestPackZIMMetadata locks in the metadata Kiwix needs to show the archive
// nicely: the descriptive keys, a Counter, and a PNG library icon.
func TestPackZIMMetadata(t *testing.T) {
	srv := newSiteServer()
	defer srv.Close()
	dir := t.TempDir()
	out := filepath.Join(dir, "site.zim")
	state := filepath.Join(dir, "site.db")

	if _, err := Pack(context.Background(), srv.URL, packDefaults(out, state, PackZIM)); err != nil {
		t.Fatal(err)
	}
	r, err := zim.Open(out)
	if err != nil {
		t.Fatalf("open zim: %v", err)
	}
	defer func() { _ = r.Close() }()

	meta := map[string]zim.Entry{}
	for i := uint32(0); i < r.Count(); i++ {
		e, err := r.EntryAt(i)
		if err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		if e.Namespace == zim.NamespaceMetadata {
			meta[e.URL] = e
		}
	}

	for _, k := range []string{"Title", "Description", "Language", "Name", "Date", "Creator", "Publisher", "Scraper", "Counter", "Illustration_48x48@1"} {
		if _, ok := meta[k]; !ok {
			t.Errorf("missing metadata %q", k)
		}
	}
	// Creator is the site, not the tool version.
	if got := string(meta["Creator"].Data); got == "" || got == "dev" {
		t.Errorf("Creator = %q, want the content origin", got)
	}
	// Scraper names the tool.
	if got := string(meta["Scraper"].Data); !strings.HasPrefix(got, "yomi") {
		t.Errorf("Scraper = %q, want a yomi prefix", got)
	}
	// Counter reports the HTML page count.
	if got := string(meta["Counter"].Data); !strings.Contains(got, "text/html=") {
		t.Errorf("Counter = %q, want a text/html tally", got)
	}
	// The icon is a real 48x48 PNG.
	icon := meta["Illustration_48x48@1"]
	if icon.MimeType != "image/png" {
		t.Errorf("icon mime = %q, want image/png", icon.MimeType)
	}
	img, err := png.Decode(bytes.NewReader(icon.Data))
	if err != nil {
		t.Fatalf("decode icon: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 48 || b.Dy() != 48 {
		t.Errorf("icon size = %dx%d, want 48x48", b.Dx(), b.Dy())
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
