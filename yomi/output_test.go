package yomi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadHTMLFromBytes(t *testing.T) {
	opts := Defaults()
	opts.Render = RenderOff
	opts.Fetched = "2026-06-17T00:00:00Z"

	p, err := ReadHTML(context.Background(), []byte(articleHTML), "https://ex.com/post", opts)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Reading Test" {
		t.Errorf("title = %q", p.Title)
	}
	if !strings.Contains(p.Markdown, "first paragraph") {
		t.Errorf("body missing article text:\n%s", p.Markdown)
	}
	if strings.Contains(p.Markdown, "chrome") {
		t.Errorf("footer chrome leaked into body:\n%s", p.Markdown)
	}
	// A relative in-page link resolves to an absolute URL against the base.
	var found bool
	for _, l := range p.Links {
		if l.URL == "https://ex.com/next" {
			found = true
		}
	}
	if !found {
		t.Errorf("relative link not resolved against base: %+v", p.Links)
	}
}

func TestStandaloneHTMLIsSelfContained(t *testing.T) {
	p := &Page{URL: "https://ex.com/p", Title: "A Title", Byline: "Writer", Lang: "en",
		ReadingMin: 3, Markdown: "# Heading\n\nSome **bold** body."}
	doc, err := StandaloneHTML(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!DOCTYPE html>", "<title>A Title</title>", "<h1>A Title</h1>",
		">source<", "<strong>bold</strong>", "lang=\"en\""} {
		if !strings.Contains(doc, want) {
			t.Errorf("standalone HTML missing %q:\n%s", want, doc)
		}
	}
	// Standalone output has no offline contents-page footer.
	if strings.Contains(doc, "Contents</a>") {
		t.Errorf("standalone HTML should not carry a contents link:\n%s", doc)
	}
}

func TestWriteDatasetJSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site.json")
	pages := []*Page{
		samplePage("https://ex.com/", "Home", "home"),
		samplePage("https://ex.com/a", "A", "a"),
	}
	if err := writeDataset(out, pages, SiteJSON); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var got []*Page
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON dataset is not a valid array: %v", err)
	}
	if len(got) != 2 || got[0].Title != "Home" || got[1].Title != "A" {
		t.Errorf("round trip = %+v", got)
	}
	if !strings.Contains(string(data), "\n  ") {
		t.Errorf("JSON dataset is not indented:\n%s", data)
	}
}

func TestWriteDatasetJSONL(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site.jsonl")
	pages := []*Page{
		samplePage("https://ex.com/", "Home", "home"),
		samplePage("https://ex.com/a", "A", "a"),
	}
	if err := writeDataset(out, pages, SiteJSONL); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), data)
	}
	for i, ln := range lines {
		var p Page
		if err := json.Unmarshal([]byte(ln), &p); err != nil {
			t.Errorf("line %d is not a valid JSON record: %v", i, err)
		}
	}
}
