package yomi

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// readZipEntry returns the bytes of one entry in a zip archive.
func readZipEntry(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer func() { _ = rc.Close() }()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return b
		}
	}
	t.Fatalf("entry %q not found in archive", name)
	return nil
}

func buildTestEPUB(t *testing.T) (*zip.Reader, string) {
	t.Helper()
	st := openTempStore(t)
	ctx := context.Background()
	_ = st.setMeta("seed", "https://ex.com/")

	home := samplePage("https://ex.com/", "Home", "Welcome to [the about page](https://ex.com/about).")
	home.Path = "index.md"
	about := samplePage("https://ex.com/about", "About Us", "An about page with a <br> break and an image ![pic](https://ex.com/p.png).")
	about.Path = "about.md"
	if err := st.put(ctx, home, 0, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := st.put(ctx, about, 1, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "book.epub")
	popts := PackOptions{Format: PackEPUB, Out: out, Date: "2026-06-17", Language: "eng", Version: "test"}
	n, err := buildEPUB(st, popts, "ex.com", out)
	if err != nil {
		t.Fatalf("buildEPUB: %v", err)
	}
	if n <= 0 {
		t.Fatalf("buildEPUB wrote %d bytes", n)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatalf("the archive does not open as a zip: %v", err)
	}
	t.Cleanup(func() { _ = zr.Close() })
	return &zr.Reader, out
}

func TestEPUBMimetypeFirstAndStored(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	first := zr.File[0]
	if first.Name != "mimetype" {
		t.Fatalf("first entry = %q, want mimetype", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("mimetype is compressed (method %d), want stored", first.Method)
	}
	if got := string(readZipEntry(t, zr, "mimetype")); got != "application/epub+zip" {
		t.Errorf("mimetype = %q", got)
	}
}

func TestEPUBContainerPointsAtPackage(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	c := string(readZipEntry(t, zr, "META-INF/container.xml"))
	if !strings.Contains(c, `full-path="OEBPS/content.opf"`) {
		t.Errorf("container does not point at the package:\n%s", c)
	}
}

func TestEPUBPackageManifestAndSpine(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	var pkg struct {
		Manifest struct {
			Items []struct {
				ID         string `xml:"id,attr"`
				Href       string `xml:"href,attr"`
				MediaType  string `xml:"media-type,attr"`
				Properties string `xml:"properties,attr"`
			} `xml:"item"`
		} `xml:"manifest"`
		Spine struct {
			Refs []struct {
				IDRef string `xml:"idref,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
	}
	if err := xml.Unmarshal(readZipEntry(t, zr, "OEBPS/content.opf"), &pkg); err != nil {
		t.Fatalf("package is not well-formed XML: %v", err)
	}

	// Every manifest href must exist as an archive entry, and every spine idref
	// must name a manifest item.
	ids := map[string]string{}
	for _, it := range pkg.Manifest.Items {
		ids[it.ID] = it.Href
		readZipEntry(t, zr, "OEBPS/"+it.Href) // fails the test if absent
	}
	if len(pkg.Spine.Refs) == 0 {
		t.Fatal("spine is empty")
	}
	for _, r := range pkg.Spine.Refs {
		if _, ok := ids[r.IDRef]; !ok {
			t.Errorf("spine references unknown manifest id %q", r.IDRef)
		}
	}

	// The two pages, the nav, and the cover are all in the manifest.
	hrefs := strings.Join(valuesOf(ids), " ")
	for _, want := range []string{"nav.xhtml", "cover.xhtml", "cover.png", "text/index.xhtml", "text/about.xhtml"} {
		if !strings.Contains(hrefs, want) {
			t.Errorf("manifest missing %q (have %q)", want, hrefs)
		}
	}
}

func TestEPUBContentDocsAreWellFormed(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	for _, name := range []string{
		"OEBPS/nav.xhtml", "OEBPS/cover.xhtml",
		"OEBPS/text/index.xhtml", "OEBPS/text/about.xhtml",
	} {
		data := readZipEntry(t, zr, name)
		if err := xml.Unmarshal(data, new(struct{})); err != nil {
			t.Errorf("%s is not well-formed XML: %v", name, err)
		}
	}
}

func TestEPUBInternalLinkRewritten(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	// The home page links to /about, which must point at the sibling chapter, not
	// back out to the live web.
	idx := string(readZipEntry(t, zr, "OEBPS/text/index.xhtml"))
	if !strings.Contains(idx, `href="about.xhtml"`) {
		t.Errorf("internal link to /about not rewritten to sibling chapter:\n%s", idx)
	}
}

func TestXHTMLName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"index.md", "index.xhtml"},
		{"about.md", "about.xhtml"},
		{"blog/post.md", "blog_post.xhtml"},
		{".md", "index.xhtml"},
	}
	for _, c := range cases {
		if got := xhtmlName(c.in); got != c.want {
			t.Errorf("xhtmlName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEPUBLang(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "en"}, {"eng", "en"}, {"en", "en"},
		{"fra", "fr"}, {"jpn", "ja"}, {"zho", "zh"},
		{"xyz", "xyz"},
	}
	for _, c := range cases {
		if got := epubLang(c.in); got != c.want {
			t.Errorf("epubLang(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func valuesOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
