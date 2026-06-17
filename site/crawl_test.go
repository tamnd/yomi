package site

import (
	"context"
	"net/url"
	"sort"
	"sync"
	"testing"

	"github.com/tamnd/kage/urlx"
)

// graph is a tiny fake site: each path maps to the links found on that page.
var graph = map[string][]string{
	"/":    {"/a", "/b", "https://other.com/x"},
	"/a":   {"/a/1", "/"},
	"/b":   {"/a/1"},
	"/a/1": {"/a", "/b"},
}

func seedURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestCrawlVisitsInScope(t *testing.T) {
	var mu sync.Mutex
	visited := map[string]bool{}

	visit := func(_ context.Context, rawURL string, _ int) ([]string, error) {
		u, _ := url.Parse(rawURL)
		mu.Lock()
		visited[u.Path] = true
		mu.Unlock()
		return graph[u.Path], nil
	}

	cfg := Config{
		Seed:    seedURL(t, "https://ex.com/"),
		Workers: 2,
		Robots:  false,
	}
	if err := Crawl(context.Background(), cfg, visit); err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(visited))
	for p := range visited {
		got = append(got, p)
	}
	sort.Strings(got)
	want := []string{"/", "/a", "/a/1", "/b"}
	if len(got) != len(want) {
		t.Fatalf("visited %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visited %v, want %v", got, want)
		}
	}
	if visited["other.com"] {
		t.Errorf("crawler left scope")
	}
}

func TestCrawlMaxPages(t *testing.T) {
	var mu sync.Mutex
	count := 0
	visit := func(_ context.Context, rawURL string, _ int) ([]string, error) {
		u, _ := url.Parse(rawURL)
		mu.Lock()
		count++
		mu.Unlock()
		return graph[u.Path], nil
	}
	cfg := Config{
		Seed:     seedURL(t, "https://ex.com/"),
		Workers:  1,
		MaxPages: 2,
		Robots:   false,
	}
	if err := Crawl(context.Background(), cfg, visit); err != nil {
		t.Fatal(err)
	}
	if count > 2 {
		t.Errorf("visited %d pages, want at most 2", count)
	}
}

func TestCrawlMaxDepth(t *testing.T) {
	var mu sync.Mutex
	visited := map[string]bool{}
	visit := func(_ context.Context, rawURL string, _ int) ([]string, error) {
		u, _ := url.Parse(rawURL)
		mu.Lock()
		visited[u.Path] = true
		mu.Unlock()
		return graph[u.Path], nil
	}
	cfg := Config{
		Seed:     seedURL(t, "https://ex.com/"),
		Workers:  2,
		MaxDepth: 1,
		Robots:   false,
		Scope:    urlx.ScopeConfig{},
	}
	if err := Crawl(context.Background(), cfg, visit); err != nil {
		t.Fatal(err)
	}
	// Depth 0 = "/", depth 1 = "/a" and "/b". "/a/1" sits at depth 2 and must
	// not be visited.
	if visited["/a/1"] {
		t.Errorf("depth cap not honoured: /a/1 visited")
	}
	if !visited["/a"] || !visited["/b"] {
		t.Errorf("depth-1 pages missing: %v", visited)
	}
}
