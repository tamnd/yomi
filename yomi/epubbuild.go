package yomi

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"os"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// buildEPUB compiles the pages already in the store into an EPUB 3 book at
// outPath. Each page becomes a well-formed XHTML chapter under OEBPS/text, a
// generated navigation document lists them in crawl order, internal links are
// rewired to point at sibling chapters, every referenced image is pulled into the
// container so the book reads with no network, and a drawn-in-code cover stands
// in front. It returns the number of bytes written.
//
// The chapter bodies come from the same Markdown-to-HTML renderer the ZIM build
// uses, then pass through an XHTML serialiser, because an EPUB content document
// must be XML the reader can parse, not the looser HTML5 the renderer emits. The
// result validates against EPUBCheck: no remote resources and no dangling
// internal links.
func buildEPUB(ctx context.Context, st *store, popts PackOptions, host, outPath string) (int64, error) {
	pages, err := st.allPages()
	if err != nil {
		return 0, err
	}
	if len(pages) == 0 {
		return 0, fmt.Errorf("nothing to pack: the store has no pages")
	}

	seed, _ := st.getMeta("seed")
	title := firstNonEmpty(popts.Title, siteTitleFromPages(host, pages))
	lang := epubLang(popts.Language)

	// Assign a flat, collision-free chapter filename to every page so all content
	// documents are siblings in OEBPS/text, and remember the canonical-URL to
	// filename mapping so links between pages resolve to those siblings.
	used := map[string]bool{}
	names := make([]string, len(pages))
	canon2epub := map[string]string{}
	for i, p := range pages {
		n := uniqueName(used, xhtmlName(p.Path))
		used[n] = true
		names[i] = n
		canon2epub[canonURL(p.URL)] = n
	}

	// Render every chapter to a parsed DOM up front and collect the images they
	// reference, so the images can be fetched once and shared, and so each chapter
	// can be transformed against the final image map in a second pass.
	docs := make([][]*xhtml.Node, len(pages))
	raw := make([]string, len(pages))
	var imgSrcs []string
	imgSeen := map[string]bool{}
	for i, p := range pages {
		body, rerr := renderMarkdown(p.Markdown)
		if rerr != nil {
			return 0, fmt.Errorf("render %s: %w", p.URL, rerr)
		}
		nodes, perr := parseBodyFragment(body)
		if perr != nil {
			raw[i] = body
			continue
		}
		docs[i] = nodes
		for _, n := range nodes {
			collectImageSrcs(n, &imgSrcs, imgSeen)
		}
	}

	images := fetchImages(ctx, imgSrcs, popts.Options, popts.log)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// The mimetype entry must come first and be stored uncompressed, so a reader
	// can identify the archive from its first bytes without inflating anything.
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return 0, err
	}
	if _, err := mw.Write([]byte("application/epub+zip")); err != nil {
		return 0, err
	}

	add := func(name string, data []byte) error {
		w, cerr := zw.Create(name)
		if cerr != nil {
			return cerr
		}
		_, cerr = w.Write(data)
		return cerr
	}

	if err := add("META-INF/container.xml", []byte(epubContainerXML)); err != nil {
		return 0, err
	}
	if err := add("OEBPS/style.css", []byte(epubCSS())); err != nil {
		return 0, err
	}

	// The cover: a drawn-in-code portrait image and the XHTML page that frames it,
	// so the book has a tile in a library and a first page in a reader. A caller-
	// supplied PNG (--icon) wins, matching the ZIM library-icon override.
	cover := coverImage(popts.Icon)
	if err := add("OEBPS/cover.png", cover); err != nil {
		return 0, err
	}
	if err := add("OEBPS/cover.xhtml", []byte(epubCoverXHTML(lang))); err != nil {
		return 0, err
	}

	for _, a := range images.assets {
		if err := add("OEBPS/"+a.name, a.data); err != nil {
			return 0, err
		}
	}

	for i, p := range pages {
		var body string
		if docs[i] != nil {
			body = epubChapterBody(docs[i], canon2epub, images.bySrc)
		} else {
			body = toXHTML(raw[i])
		}
		doc := epubChapterXHTML(p, body, lang)
		if err := add("OEBPS/text/"+names[i], []byte(doc)); err != nil {
			return 0, err
		}
	}

	if err := add("OEBPS/nav.xhtml", []byte(epubNavXHTML(title, lang, pages, names))); err != nil {
		return 0, err
	}
	opf := epubPackageOPF(title, host, seed, lang, popts.Date, pages, names, images.assets)
	if err := add("OEBPS/content.opf", []byte(opf)); err != nil {
		return 0, err
	}

	if err := zw.Close(); err != nil {
		return 0, err
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}

// parseBodyFragment parses an HTML fragment as the children of a <body>, the same
// context the renderer's output belongs in.
func parseBodyFragment(fragment string) ([]*xhtml.Node, error) {
	ctx := &xhtml.Node{Type: xhtml.ElementNode, Data: "body", DataAtom: atom.Body}
	return xhtml.ParseFragment(strings.NewReader(fragment), ctx)
}

// collectImageSrcs walks a node tree and records every <img src> it has not seen
// before, preserving first-seen order so image naming is deterministic.
func collectImageSrcs(n *xhtml.Node, order *[]string, seen map[string]bool) {
	if n.Type == xhtml.ElementNode && n.DataAtom == atom.Img {
		if src := nodeAttr(n, "src"); src != "" && !seen[src] {
			seen[src] = true
			*order = append(*order, src)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectImageSrcs(c, order, seen)
	}
}

// epubChapterBody transforms a chapter's parsed DOM into a well-formed XHTML
// string: internal page links are pointed at sibling chapters, an image is
// repointed at its embedded copy (or, when it could not be embedded, replaced by
// its alt text or removed), and an internal fragment link whose target does not
// exist in the chapter is defused so the book carries no broken references.
func epubChapterBody(nodes []*xhtml.Node, canon2epub map[string]string, images map[string]embeddedImage) string {
	ids := map[string]bool{}
	for _, n := range nodes {
		collectFragmentIDs(n, ids)
	}
	var drop []*xhtml.Node
	for _, n := range nodes {
		transformEPUBNode(n, canon2epub, images, ids, &drop)
	}
	for _, n := range drop {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}
	var b strings.Builder
	for _, n := range nodes {
		writeXHTML(&b, n)
	}
	return b.String()
}

// collectFragmentIDs records every id and name a node tree defines, the set of
// targets an in-document #fragment link can legally point at.
func collectFragmentIDs(n *xhtml.Node, ids map[string]bool) {
	if n.Type == xhtml.ElementNode {
		if v := nodeAttr(n, "id"); v != "" {
			ids[v] = true
		}
		if v := nodeAttr(n, "name"); v != "" {
			ids[v] = true
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectFragmentIDs(c, ids)
	}
}

// transformEPUBNode rewrites one node and its descendants for the EPUB: links to
// in-book pages, images to embedded copies, and dangling in-document fragments to
// plain text.
func transformEPUBNode(n *xhtml.Node, canon2epub map[string]string, images map[string]embeddedImage, ids map[string]bool, drop *[]*xhtml.Node) {
	if n.Type == xhtml.ElementNode {
		switch n.DataAtom {
		case atom.A:
			rewriteAnchor(n, canon2epub, ids)
		case atom.Img:
			rewriteImage(n, images, drop)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		transformEPUBNode(c, canon2epub, images, ids, drop)
	}
}

// rewriteAnchor points an <a> at a sibling chapter when its href names an in-book
// page, and strips the href of an in-document #fragment link whose target is
// absent, which leaves the text in place but removes the reference EPUBCheck would
// flag as broken. A real in-page anchor and an external link are both left alone.
func rewriteAnchor(n *xhtml.Node, canon2epub map[string]string, ids map[string]bool) {
	for i := range n.Attr {
		if n.Attr[i].Key != "href" {
			continue
		}
		v := n.Attr[i].Val
		if t, ok := canon2epub[canonURL(v)]; ok {
			n.Attr[i].Val = t
			return
		}
		if strings.HasPrefix(v, "#") && !ids[strings.TrimPrefix(v, "#")] {
			n.Attr = removeAttrAt(n.Attr, i)
		}
		return
	}
}

// rewriteImage repoints an <img> at its embedded copy. An image that could not be
// embedded would be a remote reference the format forbids, so it is replaced by
// its alt text when it has any, and otherwise removed.
func rewriteImage(n *xhtml.Node, images map[string]embeddedImage, drop *[]*xhtml.Node) {
	src := nodeAttr(n, "src")
	if emb, ok := images[src]; ok {
		setNodeAttr(n, "src", "../"+emb.name)
		return
	}
	if alt := strings.TrimSpace(nodeAttr(n, "alt")); alt != "" {
		n.Type = xhtml.TextNode
		n.DataAtom = 0
		n.Data = alt
		n.Attr = nil
		return
	}
	*drop = append(*drop, n)
}

// nodeAttr returns the value of a node's attribute, or "" when it is absent.
func nodeAttr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// setNodeAttr sets (or adds) a node attribute.
func setNodeAttr(n *xhtml.Node, key, val string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, xhtml.Attribute{Key: key, Val: val})
}

// removeAttrAt returns the attribute slice with index i removed.
func removeAttrAt(attrs []xhtml.Attribute, i int) []xhtml.Attribute {
	return append(attrs[:i], attrs[i+1:]...)
}

// xhtmlName maps a page's .md path to its EPUB chapter filename: the path with
// directory separators folded to underscores and an .xhtml suffix, so every
// chapter is a sibling in one flat folder and the relative links between them are
// just filenames.
func xhtmlName(mdPath string) string {
	stem := strings.TrimSuffix(mdPath, ".md")
	stem = strings.ReplaceAll(stem, "/", "_")
	if stem == "" {
		stem = "index"
	}
	return stem + ".xhtml"
}

// epubLang maps the pack's ISO 639-3 language code (the ZIM convention, e.g.
// "eng") to the BCP 47 tag EPUB expects (e.g. "en"), falling back to the code
// itself for anything not in the small common set, and to "en" when unset.
func epubLang(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "", "eng", "en":
		return "en"
	case "fra", "fre", "fr":
		return "fr"
	case "deu", "ger", "de":
		return "de"
	case "spa", "es":
		return "es"
	case "ita", "it":
		return "it"
	case "por", "pt":
		return "pt"
	case "nld", "dut", "nl":
		return "nl"
	case "rus", "ru":
		return "ru"
	case "jpn", "ja":
		return "ja"
	case "zho", "chi", "zh":
		return "zh"
	case "kor", "ko":
		return "ko"
	default:
		return code
	}
}

// coverImage returns the cover PNG. A caller-supplied path (the same --icon used
// for the ZIM library icon) wins; a read failure or no path falls back to the
// built-in drawn cover, so a book always has one.
func coverImage(path string) []byte {
	if path != "" {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return data
		}
	}
	return coverPNG(600, 800)
}

// epubContainerXML is the fixed OCF entry point that points a reader at the
// package document.
const epubContainerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

// epubCSS returns the reading stylesheet as a standalone CSS file, the same rules
// the ZIM build inlines, with the surrounding <style> tags stripped.
func epubCSS() string {
	css := strings.TrimSpace(zimStyle)
	css = strings.TrimPrefix(css, "<style>")
	css = strings.TrimSuffix(css, "</style>")
	return strings.TrimSpace(css) + "\n"
}

// epubChapterXHTML wraps a page's XHTML body in a complete content document: the
// title, a small source-and-byline line, and the article, linked to the shared
// stylesheet one directory up.
func epubChapterXHTML(p *Page, body, lang string) string {
	var b strings.Builder
	b.WriteString(xmlProlog)
	fmt.Fprintf(&b, "<html xmlns=\"http://www.w3.org/1999/xhtml\" xml:lang=%q lang=%q>\n", lang, lang)
	b.WriteString("<head>\n<meta charset=\"utf-8\"/>\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(titleOr(p)))
	b.WriteString("<link rel=\"stylesheet\" type=\"text/css\" href=\"../style.css\"/>\n")
	b.WriteString("</head>\n<body>\n<main>\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(titleOr(p)))
	b.WriteString("<p class=\"yomi-meta\">")
	if p.Byline != "" {
		fmt.Fprintf(&b, "%s &#183; ", html.EscapeString(p.Byline))
	}
	fmt.Fprintf(&b, "<a href=%q>source</a>", html.EscapeString(p.URL))
	if p.ReadingMin > 0 {
		fmt.Fprintf(&b, " &#183; %d min read", p.ReadingMin)
	}
	b.WriteString("</p>\n")
	b.WriteString(body)
	b.WriteString("\n</main>\n</body>\n</html>\n")
	return b.String()
}

// epubNavXHTML builds the EPUB navigation document: the table of contents a
// reader uses to jump between chapters, listing every page in crawl order.
func epubNavXHTML(title, lang string, pages []*Page, names []string) string {
	var b strings.Builder
	b.WriteString(xmlProlog)
	fmt.Fprintf(&b, "<html xmlns=\"http://www.w3.org/1999/xhtml\" xmlns:epub=\"http://www.idpf.org/2007/ops\" xml:lang=%q lang=%q>\n", lang, lang)
	b.WriteString("<head>\n<meta charset=\"utf-8\"/>\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString("<link rel=\"stylesheet\" type=\"text/css\" href=\"style.css\"/>\n")
	b.WriteString("</head>\n<body>\n")
	b.WriteString("<nav epub:type=\"toc\" id=\"toc\">\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", html.EscapeString(title))
	b.WriteString("<ol>\n")
	for i, p := range pages {
		fmt.Fprintf(&b, "<li><a href=%q>%s</a></li>\n",
			"text/"+names[i], html.EscapeString(titleOr(p)))
	}
	b.WriteString("</ol>\n</nav>\n</body>\n</html>\n")
	return b.String()
}

// epubCoverXHTML frames the cover image as the book's first page, scaled to fit
// whatever screen the reader uses.
func epubCoverXHTML(lang string) string {
	return xmlProlog +
		fmt.Sprintf("<html xmlns=\"http://www.w3.org/1999/xhtml\" xml:lang=%q lang=%q>\n", lang, lang) +
		"<head>\n<meta charset=\"utf-8\"/>\n<title>Cover</title>\n" +
		"<style>html,body{margin:0;padding:0;height:100%;}" +
		"img{display:block;width:100%;height:100%;object-fit:contain;}</style>\n" +
		"</head>\n<body>\n<img src=\"cover.png\" alt=\"Cover\"/>\n</body>\n</html>\n"
}

// epubPackageOPF builds the package document: the Dublin Core metadata, the
// manifest of every file in the book, and the spine that orders them for reading.
func epubPackageOPF(title, host, seed, lang, date string, pages []*Page, names []string, images []epubImageAsset) string {
	id := seed
	if id == "" {
		id = "urn:yomi:" + host
	}
	modified := firstNonEmpty(date, "1970-01-01") + "T00:00:00Z"

	var b strings.Builder
	b.WriteString(xmlProlog)
	b.WriteString("<package xmlns=\"http://www.idpf.org/2007/opf\" version=\"3.0\" unique-identifier=\"bookid\">\n")
	b.WriteString("<metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\n")
	fmt.Fprintf(&b, "<dc:identifier id=\"bookid\">%s</dc:identifier>\n", html.EscapeString(id))
	fmt.Fprintf(&b, "<dc:title>%s</dc:title>\n", html.EscapeString(title))
	fmt.Fprintf(&b, "<dc:language>%s</dc:language>\n", html.EscapeString(lang))
	fmt.Fprintf(&b, "<dc:creator>%s</dc:creator>\n", html.EscapeString(host))
	fmt.Fprintf(&b, "<dc:publisher>%s</dc:publisher>\n", "yomi")
	if seed != "" {
		fmt.Fprintf(&b, "<dc:source>%s</dc:source>\n", html.EscapeString(seed))
	}
	if date != "" {
		fmt.Fprintf(&b, "<dc:date>%s</dc:date>\n", html.EscapeString(date))
	}
	fmt.Fprintf(&b, "<meta property=\"dcterms:modified\">%s</meta>\n", modified)

	// Accessibility metadata, the discovery fields an EPUB is expected to carry: the
	// content is reflowable text a reader can navigate by its table of contents, and
	// the text alone conveys the work.
	b.WriteString("<meta property=\"schema:accessMode\">textual</meta>\n")
	if len(images) > 0 {
		b.WriteString("<meta property=\"schema:accessMode\">visual</meta>\n")
	}
	b.WriteString("<meta property=\"schema:accessModeSufficient\">textual</meta>\n")
	b.WriteString("<meta property=\"schema:accessibilityFeature\">tableOfContents</meta>\n")
	b.WriteString("<meta property=\"schema:accessibilityFeature\">structuralNavigation</meta>\n")
	b.WriteString("<meta property=\"schema:accessibilityFeature\">readingOrder</meta>\n")
	b.WriteString("<meta property=\"schema:accessibilityHazard\">none</meta>\n")
	fmt.Fprintf(&b, "<meta property=\"schema:accessibilitySummary\">Reflowable text with a navigable table of contents, packed from %s by yomi.</meta>\n",
		html.EscapeString(host))

	b.WriteString("<meta name=\"cover\" content=\"cover-image\"/>\n")
	b.WriteString("</metadata>\n")

	b.WriteString("<manifest>\n")
	b.WriteString("<item id=\"nav\" href=\"nav.xhtml\" media-type=\"application/xhtml+xml\" properties=\"nav\"/>\n")
	b.WriteString("<item id=\"css\" href=\"style.css\" media-type=\"text/css\"/>\n")
	b.WriteString("<item id=\"cover\" href=\"cover.xhtml\" media-type=\"application/xhtml+xml\"/>\n")
	b.WriteString("<item id=\"cover-image\" href=\"cover.png\" media-type=\"image/png\" properties=\"cover-image\"/>\n")
	for i, n := range names {
		fmt.Fprintf(&b, "<item id=\"p%d\" href=%q media-type=\"application/xhtml+xml\"/>\n",
			i, "text/"+n)
	}
	for _, a := range images {
		fmt.Fprintf(&b, "<item id=%q href=%q media-type=%q/>\n",
			a.id, html.EscapeString(a.name), html.EscapeString(a.mediaType))
	}
	b.WriteString("</manifest>\n")

	b.WriteString("<spine>\n")
	b.WriteString("<itemref idref=\"cover\"/>\n")
	b.WriteString("<itemref idref=\"nav\"/>\n")
	for i := range names {
		fmt.Fprintf(&b, "<itemref idref=\"p%d\"/>\n", i)
	}
	b.WriteString("</spine>\n")

	b.WriteString("</package>\n")
	return b.String()
}

// xmlProlog is the XML declaration and XHTML doctype every EPUB content document
// opens with.
const xmlProlog = "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n" +
	"<!DOCTYPE html>\n"
