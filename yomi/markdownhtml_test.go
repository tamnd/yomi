package yomi

import (
	"strings"
	"testing"
)

func TestRenderMarkdownGFM(t *testing.T) {
	html, err := renderMarkdown("# Hi\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n~~gone~~\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<table>") {
		t.Errorf("GFM table not rendered:\n%s", html)
	}
	if !strings.Contains(html, "<del>") {
		t.Errorf("GFM strikethrough not rendered:\n%s", html)
	}
}

func TestRewriteLinks(t *testing.T) {
	in := `<p>see <a href="https://ex.com/a">a</a> and <a href="https://other.com/x">x</a></p>`
	out := rewriteLinks(in, func(href string) (string, bool) {
		if href == "https://ex.com/a" {
			return "a.html", true
		}
		return "", false
	})
	if !strings.Contains(out, `href="a.html"`) {
		t.Errorf("internal link not rewritten:\n%s", out)
	}
	if !strings.Contains(out, `href="https://other.com/x"`) {
		t.Errorf("external link should be untouched:\n%s", out)
	}
}

func TestPageHTMLSelfContained(t *testing.T) {
	p := &Page{URL: "https://ex.com/p", Title: "A Title", Byline: "Writer", Lang: "en", ReadingMin: 3}
	doc := pageHTML(p, "<p>body</p>", "index.html")
	for _, want := range []string{"<!DOCTYPE html>", "<title>A Title</title>", "<h1>A Title</h1>",
		">source<", "<p>body</p>", `href="index.html"`, "lang=\"en\""} {
		if !strings.Contains(doc, want) {
			t.Errorf("page HTML missing %q:\n%s", want, doc)
		}
	}
}

func TestIndexHTMLListsPages(t *testing.T) {
	pages := []*Page{{Title: "First", ReadingMin: 2}, {Title: "Second", ReadingMin: 5}}
	doc := indexHTML("My Site", "ex.com", pages, []string{"first.html", "second.html"})
	for _, want := range []string{"My Site", "yomi-toc", `href="first.html"`, "First", `href="second.html"`, "Second"} {
		if !strings.Contains(doc, want) {
			t.Errorf("index HTML missing %q:\n%s", want, doc)
		}
	}
}

func TestHTMLName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"index.md", "index.html"},
		{"greatwork.md", "greatwork.html"},
		{"blog/post.md", "blog/post.html"},
	}
	for _, c := range cases {
		if got := htmlName(c.in); got != c.want {
			t.Errorf("htmlName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestUniqueName(t *testing.T) {
	used := map[string]bool{"index.html": true, "index-2.html": true}
	if got := uniqueName(used, "about.html"); got != "about.html" {
		t.Errorf("free name = %q, want about.html", got)
	}
	if got := uniqueName(used, "index.html"); got != "index-3.html" {
		t.Errorf("collision = %q, want index-3.html", got)
	}
	if got := uniqueName(used, "index.html", "contents.html"); got != "contents.html" {
		t.Errorf("fallback candidate = %q, want contents.html", got)
	}
}
