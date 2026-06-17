package extract

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// langOf parses an HTML fragment, runs the code normalisation, and returns the
// language class set on its first <pre> or <code>, plus the node's text.
func langOf(t *testing.T, frag string) (string, string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(frag))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	normalizeCode(doc)
	pre := findTag(doc, "pre")
	if pre == nil {
		t.Fatalf("no <pre> in fragment")
	}
	target := firstCode(pre)
	if target == nil {
		target = pre
	}
	return attr(target, "class"), text(pre)
}

func findTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if got := findTag(c, tag); got != nil {
			return got
		}
	}
	return nil
}

func TestNormalizeCodeLanguage(t *testing.T) {
	cases := []struct {
		name, frag, want string
	}{
		{"code-language", `<pre><code class="language-go">x</code></pre>`, "language-go"},
		{"pre-language", `<pre class="language-rust"><code>x</code></pre>`, "language-rust"},
		{"sphinx-parent", `<div class="highlight-python3 notranslate"><div class="highlight"><pre><code>x</code></pre></div></div>`, "language-python"},
		{"github-source", `<div class="highlight highlight-source-go"><pre><code>x</code></pre></div>`, "language-go"},
		{"mdn-brush", `<pre class="brush: js notranslate"><code>x</code></pre>`, "language-javascript"},
		{"data-language", `<pre data-language="ruby"><code>x</code></pre>`, "language-ruby"},
		{"alias-js", `<pre><code class="lang-js">x</code></pre>`, "language-javascript"},
		{"placeholder-default", `<div class="highlight-default"><pre><code>x</code></pre></div>`, ""},
		{"no-hint", `<pre><code>x</code></pre>`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := langOf(t, c.frag)
			if got != c.want {
				t.Errorf("class = %q, want %q", got, c.want)
			}
		})
	}
}

func TestNormalizeCodeStripsOtherClasses(t *testing.T) {
	doc, _ := html.Parse(strings.NewReader(
		`<div class="leak"><p class="noise">hi</p><pre><code class="language-go hljs">x</code></pre></div>`))
	normalizeCode(doc)
	if c := attr(findTag(doc, "p"), "class"); c != "" {
		t.Errorf("class on <p> not stripped: %q", c)
	}
	if c := attr(findTag(doc, "div"), "class"); c != "" {
		t.Errorf("class on <div> not stripped: %q", c)
	}
	// The code keeps only the normalised language class, not the highlighter noise.
	if c := attr(firstCode(findTag(doc, "pre")), "class"); c != "language-go" {
		t.Errorf("code class = %q, want language-go", c)
	}
}

func TestRestoreCodeLines(t *testing.T) {
	// A highlighter that wrapped each line in <span class="line"> with no literal
	// newline between them. The block should regain its line breaks.
	frag := `<pre><code><span class="line">one</span><span class="line">two</span><span class="line">three</span></code></pre>`
	_, body := langOf(t, frag)
	if got := strings.Count(body, "\n"); got != 2 {
		t.Errorf("restored newlines = %d, want 2 in %q", got, body)
	}
}

func TestRestoreCodeLinesLeavesCleanCode(t *testing.T) {
	// Code that already has real newlines must be left untouched.
	frag := "<pre><code>one\ntwo\n</code></pre>"
	_, body := langOf(t, frag)
	if got := strings.Count(body, "\n"); got != 2 {
		t.Errorf("newline count changed: %d in %q", got, body)
	}
}
