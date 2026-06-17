package yomi

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func samplePage(url, title, body string) *Page {
	return &Page{
		URL:        url,
		Path:       "x.md",
		Title:      title,
		Byline:     "A Writer",
		SiteName:   "A Site",
		Lang:       "en",
		Fetched:    "2026-06-17T00:00:00Z",
		WordCount:  len(body),
		ReadingMin: 1,
		Markdown:   body,
		Links: []Link{
			{Text: "in", URL: "https://ex.com/a", Internal: true},
			{Text: "out", URL: "https://other.com/x", Internal: false},
		},
		Images: []Image{{Alt: "pic", URL: "https://ex.com/p.png"}},
	}
}

func openTempStore(t *testing.T) *store {
	t.Helper()
	st, err := openStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { _ = st.close() })
	return st
}

func TestStorePutAndCounts(t *testing.T) {
	st := openTempStore(t)
	ctx := context.Background()
	if err := st.put(ctx, samplePage("https://ex.com/", "Home", "home body words here"), 0, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := st.put(ctx, samplePage("https://ex.com/a", "A", "a body"), 1, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	pages, words, err := st.counts()
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Errorf("pages = %d, want 2", pages)
	}
	if words == 0 {
		t.Errorf("words = %d, want > 0", words)
	}
}

func TestStorePutReplacesSameURL(t *testing.T) {
	st := openTempStore(t)
	ctx := context.Background()
	url := "https://ex.com/p"
	if err := st.put(ctx, samplePage(url, "First", "one"), 0, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := st.put(ctx, samplePage(url, "Second", "two"), 0, "2026-06-18T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	pages, _, _ := st.counts()
	if pages != 1 {
		t.Fatalf("pages = %d, want 1 (same URL should replace)", pages)
	}
	got, err := st.allPages()
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Title != "Second" {
		t.Errorf("title = %q, want Second (latest write wins)", got[0].Title)
	}
	// Replacing must not leave the old page's links and images behind.
	if n := len(got[0].Links); n != 2 {
		t.Errorf("links = %d, want 2 (old links cleared, new ones written)", n)
	}
}

func TestStoreAllPagesRoundTrip(t *testing.T) {
	st := openTempStore(t)
	ctx := context.Background()
	in := samplePage("https://ex.com/post", "Post", "the body")
	in.Path = "post.md"
	if err := st.put(ctx, in, 2, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	pages, err := st.allPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	p := pages[0]
	if p.URL != in.URL || p.Title != in.Title || p.Byline != in.Byline || p.Markdown != in.Markdown {
		t.Errorf("round trip mismatch: %+v", p)
	}
	if len(p.Links) != 2 || len(p.Images) != 1 {
		t.Errorf("links=%d images=%d, want 2 and 1", len(p.Links), len(p.Images))
	}
	if !p.Links[0].Internal || p.Links[1].Internal {
		t.Errorf("internal flags not preserved: %+v", p.Links)
	}
}

func TestStoreLookupResume(t *testing.T) {
	st := openTempStore(t)
	ctx := context.Background()
	if err := st.put(ctx, samplePage("https://ex.com/page", "P", "body"), 0, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)

	// A stored page with no max-age is fresh, and its internal links come back.
	links, fresh, err := st.lookup("https://ex.com/page", 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh {
		t.Fatal("stored page should be fresh with maxAge 0")
	}
	if len(links) != 1 || links[0] != "https://ex.com/a" {
		t.Errorf("internal links = %v, want [https://ex.com/a]", links)
	}

	// A trailing slash should not defeat the resume match.
	if _, fresh, _ := st.lookup("https://ex.com/page/", 0, now); !fresh {
		t.Error("trailing-slash variant should still match")
	}

	// An unknown URL is not fresh.
	if _, fresh, _ := st.lookup("https://ex.com/missing", 0, now); fresh {
		t.Error("unknown URL should not be fresh")
	}
}

func TestStoreLookupMaxAge(t *testing.T) {
	st := openTempStore(t)
	ctx := context.Background()
	if err := st.put(ctx, samplePage("https://ex.com/old", "Old", "body"), 0, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	// One hour later, a 30-minute max-age makes the page stale (refetch).
	now := time.Date(2026, 6, 17, 1, 0, 0, 0, time.UTC)
	if _, fresh, _ := st.lookup("https://ex.com/old", 30*time.Minute, now); fresh {
		t.Error("page older than max-age should be stale")
	}
	// A 2-hour max-age still considers it fresh.
	if _, fresh, _ := st.lookup("https://ex.com/old", 2*time.Hour, now); !fresh {
		t.Error("page within max-age should be fresh")
	}
}

func TestStoreMeta(t *testing.T) {
	st := openTempStore(t)
	if err := st.setMeta("host", "ex.com"); err != nil {
		t.Fatal(err)
	}
	if err := st.setMeta("host", "ex.org"); err != nil { // overwrite
		t.Fatal(err)
	}
	v, err := st.getMeta("host")
	if err != nil {
		t.Fatal(err)
	}
	if v != "ex.org" {
		t.Errorf("host = %q, want ex.org", v)
	}
	if v, _ := st.getMeta("absent"); v != "" {
		t.Errorf("absent key = %q, want empty", v)
	}
}

func TestCanonURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://Ex.com/Page", "https://ex.com/Page"},
		{"https://ex.com/page/", "https://ex.com/page"},
		{"https://ex.com/", "https://ex.com/"},
		{"https://ex.com/page#frag", "https://ex.com/page"},
		{"https://ex.com/a?b=1", "https://ex.com/a?b=1"},
	}
	for _, c := range cases {
		if got := canonURL(c.in); got != c.want {
			t.Errorf("canonURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
