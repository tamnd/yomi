package yomi

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestToXHTMLSelfClosesVoids(t *testing.T) {
	in := `<p>line<br>break<img src="x.png" alt="x"><hr></p>`
	out := toXHTML(in)
	for _, want := range []string{"<br/>", `<img src="x.png" alt="x"/>`, "<hr/>"} {
		if !strings.Contains(out, want) {
			t.Errorf("void element not self-closed, missing %q:\n%s", want, out)
		}
	}
}

func TestToXHTMLEscapesAndIsWellFormed(t *testing.T) {
	in := `<p>a &amp; b &lt; c</p><pre>x &gt; y</pre><a href="?a=1&b=2">q</a>`
	out := toXHTML(in)
	// The result, wrapped in a single root, must parse as XML.
	if err := xml.Unmarshal([]byte("<root>"+out+"</root>"), new(struct{})); err != nil {
		t.Fatalf("toXHTML output is not well-formed XML: %v\n%s", err, out)
	}
	if strings.Contains(out, "a & b") {
		t.Errorf("bare ampersand left unescaped:\n%s", out)
	}
}

func TestToXHTMLParseFailureReturnsInput(t *testing.T) {
	// An empty fragment round-trips to empty, not a panic.
	if got := toXHTML(""); got != "" {
		t.Errorf("empty fragment = %q, want empty", got)
	}
}

func TestXHTMLEscapeText(t *testing.T) {
	if got := xhtmlEscapeText(`a & b < c > d`); got != `a &amp; b &lt; c &gt; d` {
		t.Errorf("text escape = %q", got)
	}
}

func TestXHTMLEscapeAttr(t *testing.T) {
	if got := xhtmlEscapeAttr(`x"&<>`); got != `x&quot;&amp;&lt;&gt;` {
		t.Errorf("attr escape = %q", got)
	}
}
