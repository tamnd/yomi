package extract

import (
	"strings"
	"testing"
)

const samplePage = `<!doctype html>
<html lang="en">
<head>
  <title>Real Title</title>
  <meta name="description" content="A short summary.">
  <meta property="og:title" content="Real Title">
  <meta property="og:site_name" content="Example Blog">
  <meta name="author" content="Jane Doe">
</head>
<body>
  <nav><a href="/">Home</a></nav>
  <article>
    <h1>Real Title</h1>
    <p>This is the first substantial paragraph of the article body, long enough
       that readability keeps it as the main content of the page.</p>
    <p>A second paragraph continues the article with more real sentences so the
       extractor is confident this block is the story and not the chrome.</p>
    <p>See the <a href="/related">related post</a> for more detail.</p>
  </article>
  <footer><a href="/about">About</a></footer>
</body>
</html>`

func TestFromHTMLMetadata(t *testing.T) {
	art, err := FromHTML([]byte(samplePage), "https://ex.com/post")
	if err != nil {
		t.Fatal(err)
	}
	if art.Title != "Real Title" {
		t.Errorf("title = %q, want Real Title", art.Title)
	}
	if art.Lang != "en" {
		t.Errorf("lang = %q, want en", art.Lang)
	}
	if art.SiteName != "Example Blog" {
		t.Errorf("site = %q, want Example Blog", art.SiteName)
	}
	if art.Byline != "Jane Doe" {
		t.Errorf("byline = %q, want Jane Doe", art.Byline)
	}
	if art.Excerpt != "A short summary." {
		t.Errorf("excerpt = %q", art.Excerpt)
	}
}

func TestFromHTMLLinksResolved(t *testing.T) {
	art, err := FromHTML([]byte(samplePage), "https://ex.com/post")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, l := range art.Links {
		if l.URL == "https://ex.com/related" {
			found = true
		}
		if strings.HasPrefix(l.URL, "/") {
			t.Errorf("link not resolved to absolute: %q", l.URL)
		}
	}
	if !found {
		t.Errorf("expected the related link to be collected; got %+v", art.Links)
	}
}

func TestFromHTMLNode(t *testing.T) {
	art, err := FromHTML([]byte(samplePage), "https://ex.com/post")
	if err != nil {
		t.Fatal(err)
	}
	if art.Node == nil {
		t.Fatalf("expected an article node")
	}
}
