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
	// A data: URI image is embedded without any network access, and a link to a
	// fragment that no element defines is a dangling reference the build must defuse.
	about := samplePage("https://ex.com/about", "About Us",
		"An about page with a <br> break, a [missing note](#nope), and an image ![pic]("+onePixelPNG+").")
	about.Path = "about.md"
	if err := st.put(ctx, home, 0, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if err := st.put(ctx, about, 1, "2026-06-17T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "book.epub")
	popts := PackOptions{Format: PackEPUB, Out: out, Date: "2026-06-17", Language: "eng", Version: "test"}
	n, err := buildEPUB(ctx, st, popts, "ex.com", out)
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

// onePixelPNG is a 1x1 transparent PNG as a data: URI, embedded by the EPUB build
// without touching the network.
const onePixelPNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

func TestEPUBEmbedsImageAndDropsRemoteSrc(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	// The data: URI image is pulled into the container and the chapter points at the
	// local copy, so the book carries no remote image reference.
	readZipEntry(t, zr, "OEBPS/images/img1.png")
	about := string(readZipEntry(t, zr, "OEBPS/text/about.xhtml"))
	if !strings.Contains(about, `src="../images/img1.png"`) {
		t.Errorf("image not repointed at the embedded copy:\n%s", about)
	}
	if strings.Contains(about, `src="data:`) || strings.Contains(about, `src="http`) {
		t.Errorf("chapter still carries a non-local image reference:\n%s", about)
	}
	// The image is also declared in the manifest.
	opf := string(readZipEntry(t, zr, "OEBPS/content.opf"))
	if !strings.Contains(opf, `href="images/img1.png"`) || !strings.Contains(opf, `media-type="image/png"`) {
		t.Errorf("embedded image missing from manifest:\n%s", opf)
	}
}

func TestEPUBDefusesDanglingFragment(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	about := string(readZipEntry(t, zr, "OEBPS/text/about.xhtml"))
	// The link text survives, but the broken #fragment href is gone.
	if strings.Contains(about, `href="#nope"`) {
		t.Errorf("dangling fragment link not defused:\n%s", about)
	}
	if !strings.Contains(about, "missing note") {
		t.Errorf("link text was lost along with the broken href:\n%s", about)
	}
}

func TestEPUBHasAccessibilityMetadata(t *testing.T) {
	zr, _ := buildTestEPUB(t)
	opf := string(readZipEntry(t, zr, "OEBPS/content.opf"))
	for _, want := range []string{
		`property="schema:accessMode">textual`,
		`property="schema:accessModeSufficient">textual`,
		`property="schema:accessibilityFeature">tableOfContents`,
		`property="schema:accessibilityHazard">none`,
	} {
		if !strings.Contains(opf, want) {
			t.Errorf("package missing accessibility metadata %q", want)
		}
	}
}

func TestClassifyImage(t *testing.T) {
	cases := []struct {
		ct, src   string
		data      []byte
		wantMedia string
		wantExt   string
		wantOK    bool
	}{
		{"image/png", "", []byte("\x89PNG\r\n"), "image/png", ".png", true},
		{"image/jpeg; charset=binary", "", nil, "image/jpeg", ".jpg", true},
		{"", "logo.svg", []byte("<svg xmlns=\"\">"), "image/svg+xml", ".svg", true},
		{"text/html", "page", []byte("<!doctype html>"), "", "", false},
		{"application/octet-stream", "x", []byte("GIF89a....."), "image/gif", ".gif", true},
	}
	for _, c := range cases {
		gotMedia, gotExt, gotOK := classifyImage(c.ct, c.data, c.src)
		if gotMedia != c.wantMedia || gotExt != c.wantExt || gotOK != c.wantOK {
			t.Errorf("classifyImage(%q,%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.ct, c.src, gotMedia, gotExt, gotOK, c.wantMedia, c.wantExt, c.wantOK)
		}
	}
}
