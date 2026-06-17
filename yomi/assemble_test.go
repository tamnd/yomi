package yomi

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestMdPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://ex.com/", "index.md"},
		{"https://ex.com", "index.md"},
		{"https://ex.com/about", "about.md"},
		{"https://ex.com/blog/", "blog/index.md"},
		{"https://ex.com/blog/post-1", "blog/post-1.md"},
		{"https://ex.com/docs/intro.html", "docs/intro.md"},
	}
	for _, c := range cases {
		if got := mdPath(mustURL(t, c.in)); got != c.want {
			t.Errorf("mdPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAnchorFor(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://ex.com/", "index"},
		{"https://ex.com/blog/post-1", "blog-post-1"},
		{"https://ex.com/A/B_c", "a-b-c"},
	}
	for _, c := range cases {
		if got := anchorFor(mustURL(t, c.in)); got != c.want {
			t.Errorf("anchorFor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRelToRoot(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"index.md", ""},
		{"about.md", ""},
		{"blog/index.md", "../"},
		{"a/b/c.md", "../../"},
	}
	for _, c := range cases {
		if got := relToRoot(c.in); got != c.want {
			t.Errorf("relToRoot(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDemoteHeadings(t *testing.T) {
	in := "# Title\n\nbody\n\n## Sub\n\n```\n# not a heading\n```\n"
	got := demoteHeadings(in, 2)
	if !strings.Contains(got, "### Title") {
		t.Errorf("h1 not demoted to h3:\n%s", got)
	}
	if !strings.Contains(got, "#### Sub") {
		t.Errorf("h2 not demoted to h4:\n%s", got)
	}
	if !strings.Contains(got, "# not a heading") {
		t.Errorf("heading inside code fence was rewritten:\n%s", got)
	}
}

func TestDemoteHeadingsCaps(t *testing.T) {
	got := demoteHeadings("##### deep\n", 3)
	if !strings.Contains(got, "###### deep") {
		t.Errorf("heading not capped at 6: %q", got)
	}
}

func TestWriteFolder(t *testing.T) {
	dir := t.TempDir()
	pages := []*Page{
		{URL: "https://ex.com/", Path: "index.md", Title: "Home", Markdown: "home body"},
		{URL: "https://ex.com/blog/p1", Path: "blog/p1.md", Title: "Post 1", Markdown: "post body"},
	}
	opts := Defaults()
	if err := writeFolder(dir, mustURL(t, "https://ex.com/"), pages, opts); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"index.md", "blog/p1.md", "SUMMARY.md"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
	summary, _ := os.ReadFile(filepath.Join(dir, "SUMMARY.md"))
	if !strings.Contains(string(summary), "[Home](index.md)") {
		t.Errorf("summary missing home link:\n%s", summary)
	}
	if !strings.Contains(string(summary), "  - [Post 1](blog/p1.md)") {
		t.Errorf("summary missing nested post link:\n%s", summary)
	}
}

func TestWriteSingle(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "site")
	pages := []*Page{
		{URL: "https://ex.com/", Path: "index.md", Anchor: "index", Title: "Home", Markdown: "# Welcome\n\nbody", ReadingMin: 1},
		{URL: "https://ex.com/about", Path: "about.md", Anchor: "about", Title: "About", Markdown: "about body"},
	}
	opts := Defaults()
	if err := writeSingle(out, mustURL(t, "https://ex.com/"), pages, opts); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(out + ".md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "## Contents") {
		t.Errorf("missing contents heading:\n%s", s)
	}
	if !strings.Contains(s, "- [Home](#index)") {
		t.Errorf("missing TOC anchor link:\n%s", s)
	}
	if !strings.Contains(s, `<a id="about"></a>`) {
		t.Errorf("missing section anchor:\n%s", s)
	}
	// The root page's own h1 should be demoted under its section heading.
	if !strings.Contains(s, "### Welcome") {
		t.Errorf("page heading not demoted:\n%s", s)
	}
}
