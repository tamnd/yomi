package yomi

import (
	"encoding/json"
	"strings"
	"testing"
)

// The meta command blanks the body and encodes the Page as JSON. With the body
// empty the markdown field should drop out, so a metadata view stays a metadata
// view rather than carrying an empty "markdown": "" line.
func TestPageJSONOmitsEmptyMarkdown(t *testing.T) {
	p := Page{URL: "https://ex.com/a", Title: "A", WordCount: 10, ReadingMin: 1}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "markdown") {
		t.Errorf("empty markdown not omitted:\n%s", b)
	}
}

// A page that does carry a body still serializes it, so the field is omitted
// only when empty.
func TestPageJSONKeepsMarkdown(t *testing.T) {
	p := Page{URL: "https://ex.com/a", Title: "A", Markdown: "# A\n\nbody"}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"markdown"`) {
		t.Errorf("non-empty markdown wrongly omitted:\n%s", b)
	}
}
