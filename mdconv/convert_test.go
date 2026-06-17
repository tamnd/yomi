package mdconv

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parse(t *testing.T, frag string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(frag))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return doc
}

func TestConvertBasic(t *testing.T) {
	node := parse(t, "<h1>Title</h1><p>Hello <strong>world</strong>.</p>")
	got, err := Convert(node, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# Title") {
		t.Errorf("missing heading:\n%s", got)
	}
	if !strings.Contains(got, "**world**") {
		t.Errorf("missing bold:\n%s", got)
	}
}

func TestConvertResolvesRelativeLink(t *testing.T) {
	base, _ := url.Parse("https://ex.com/blog/")
	node := parse(t, `<p><a href="../about">about</a></p>`)
	got, err := Convert(node, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "https://ex.com/about") {
		t.Errorf("relative link not resolved to absolute:\n%s", got)
	}
}

func TestConvertRewriteLink(t *testing.T) {
	base, _ := url.Parse("https://ex.com/")
	node := parse(t, `<p><a href="https://ex.com/about">about</a></p>`)
	got, err := Convert(node, Options{
		Base: base,
		RewriteLink: func(abs string) string {
			if abs == "https://ex.com/about" {
				return "about.md"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "(about.md)") {
		t.Errorf("link not rewritten to local target:\n%s", got)
	}
}

func TestConvertRewriteImage(t *testing.T) {
	base, _ := url.Parse("https://ex.com/")
	node := parse(t, `<p><img src="/img/a.png" alt="A"></p>`)
	var sawAbs, sawAlt string
	got, err := Convert(node, Options{
		Base: base,
		RewriteImage: func(abs, alt string) string {
			sawAbs, sawAlt = abs, alt
			return "media/a.png"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sawAbs != "https://ex.com/img/a.png" {
		t.Errorf("image abs = %q", sawAbs)
	}
	if sawAlt != "A" {
		t.Errorf("image alt = %q", sawAlt)
	}
	if !strings.Contains(got, "media/a.png") {
		t.Errorf("image not rewritten:\n%s", got)
	}
}

func TestConvertTable(t *testing.T) {
	node := parse(t, `<table><thead><tr><th>Class</th><th>Styles</th></tr></thead>`+
		`<tbody><tr><td>flex-1</td><td>flex: 1;</td></tr></tbody></table>`)
	got, err := Convert(node, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "| Class") || !strings.Contains(got, "| Styles") {
		t.Errorf("table header not rendered as Markdown table:\n%s", got)
	}
	if !strings.Contains(got, "---") {
		t.Errorf("table delimiter row missing:\n%s", got)
	}
	if !strings.Contains(got, "flex-1") || !strings.Contains(got, "flex: 1;") {
		t.Errorf("table cells missing:\n%s", got)
	}
}

func TestConvertCodeLanguageFence(t *testing.T) {
	node := parse(t, `<pre><code class="language-go">package main</code></pre>`)
	got, err := Convert(node, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "```go") {
		t.Errorf("language info string missing from fence:\n%s", got)
	}
}

func TestCleanHeadings(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"sphinx-pilcrow",
			`## Numbers[¶](#numbers "Link to this heading")`,
			"## Numbers"},
		{"node-empty-and-hash",
			`### Promise example[]()[#](#promise-example)`,
			"### Promise example"},
		{"self-anchor-unwrap",
			`## [Examples](#examples)`,
			"## Examples"},
		{"keep-crossref",
			"### [path()](https://docs/ref#path \"path\") argument: route",
			"### [path()](https://docs/ref#path \"path\") argument: route"},
		{"not-a-heading",
			`See [¶](#x) here`,
			`See [¶](#x) here`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cleanHeadings(c.in); got != c.want {
				t.Errorf("cleanHeadings(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestCleanHeadingsSkipsFence(t *testing.T) {
	in := "```\n# Numbers[¶](#numbers)\n```\n"
	if got := cleanHeadings(in); got != in {
		t.Errorf("heading inside fence rewritten:\n%s", got)
	}
}

func TestDropPreviewCounters(t *testing.T) {
	in := "Use flex utilities:\n\n01\n\n02\n\n03\n\n```\n<div>1</div>\n```\n"
	got := dropPreviewCounters(in)
	if strings.Contains(got, "\n01\n") || strings.Contains(got, "\n02\n") {
		t.Errorf("gutter numbers not dropped:\n%s", got)
	}
	if !strings.Contains(got, "Use flex utilities:") {
		t.Errorf("prose dropped:\n%s", got)
	}
}

func TestDropPreviewCountersKeepsLoneNumber(t *testing.T) {
	in := "The answer is\n\n42\n\nas expected.\n"
	if got := dropPreviewCounters(in); !strings.Contains(got, "42") {
		t.Errorf("lone number wrongly dropped:\n%s", got)
	}
}

func TestDropPreviewCountersKeepsCodeNumbers(t *testing.T) {
	in := "```\n4\n20\n```\n"
	if got := dropPreviewCounters(in); got != in {
		t.Errorf("numbers inside fence dropped:\n%s", got)
	}
}

func TestDropDuplicateCaptions(t *testing.T) {
	in := "![Image by DALL-E](https://x/a.jpg)\n\nImage by DALL-E\n\nReal text.\n"
	got := dropDuplicateCaptions(in)
	if strings.Count(got, "Image by DALL-E") != 1 {
		t.Errorf("duplicate caption not collapsed:\n%s", got)
	}
	if !strings.Contains(got, "![Image by DALL-E](https://x/a.jpg)") {
		t.Errorf("image line lost:\n%s", got)
	}
	if !strings.Contains(got, "Real text.") {
		t.Errorf("following prose dropped:\n%s", got)
	}
}

func TestDropDuplicateCaptionsKeepsRealCaption(t *testing.T) {
	in := "![Diagram](https://x/a.png)\n\nFigure 1: the full pipeline.\n"
	if got := dropDuplicateCaptions(in); !strings.Contains(got, "Figure 1: the full pipeline.") {
		t.Errorf("distinct caption wrongly dropped:\n%s", got)
	}
}

func TestDropWidgetLinks(t *testing.T) {
	in := "Last paragraph.\n\n[Share](https://blog/p/x?action=share)\n"
	got := dropWidgetLinks(in)
	if strings.Contains(got, "Share]") {
		t.Errorf("widget link not dropped:\n%s", got)
	}
	if !strings.Contains(got, "Last paragraph.") {
		t.Errorf("prose dropped:\n%s", got)
	}
}

func TestDropWidgetLinksKeepsRealLink(t *testing.T) {
	in := "See [Comment guidelines](https://blog/rules) before posting.\n"
	if got := dropWidgetLinks(in); !strings.Contains(got, "Comment guidelines") {
		t.Errorf("inline link wrongly dropped:\n%s", got)
	}
}

func TestUnwrapSelfLinkedImage(t *testing.T) {
	node := parse(t, `<p><a href="https://cdn/x/a_1600x900.jpeg"><img src="https://cdn/x/a.jpeg" alt="A"></a></p>`)
	got, err := Convert(node, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "](https://cdn/x/a_1600x900.jpeg)") {
		t.Errorf("image still wrapped in self link:\n%s", got)
	}
	if !strings.Contains(got, "![A](https://cdn/x/a.jpeg)") {
		t.Errorf("bare image lost:\n%s", got)
	}
}

func TestUnwrapSelfLinkedImageKeepsArticleLink(t *testing.T) {
	node := parse(t, `<p><a href="https://site/article"><img src="https://cdn/thumb.png" alt="T"></a></p>`)
	got, err := Convert(node, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "](https://site/article)") {
		t.Errorf("link to article wrongly unwrapped:\n%s", got)
	}
}

func TestTidy(t *testing.T) {
	if got := Tidy("a\n\n\n\nb\n\n\n"); got != "a\n\nb\n" {
		t.Errorf("Tidy = %q", got)
	}
}
