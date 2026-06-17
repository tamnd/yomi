// Package yomi reads a web page, or a whole website, into clean Markdown. It
// renders JavaScript when a page needs it (reusing kage's headless browser),
// extracts the main content (dropping the navigation, the cookie banner, the
// share rail), and converts what is left to GitHub-Flavored Markdown with a
// small front-matter block.
//
// The single-page path is Read; the whole-site path is Site, which crawls in
// scope and assembles the pages into a folder of Markdown or one combined file.
package yomi

// Page is one document read by yomi. It is the unit every operation produces:
// Read returns one, Site collects many, and the assemblers stitch them into a
// folder or a single file.
type Page struct {
	URL        string  `json:"url"`
	RequestURL string  `json:"request_url,omitempty"`
	Path       string  `json:"path,omitempty"`   // the .md path this page maps to in a folder
	Anchor     string  `json:"anchor,omitempty"` // the stable in-file anchor for single-file assembly
	Title      string  `json:"title"`
	Byline     string  `json:"byline,omitempty"`
	SiteName   string  `json:"site_name,omitempty"`
	Excerpt    string  `json:"excerpt,omitempty"`
	Lang       string  `json:"lang,omitempty"`
	Published  string  `json:"published,omitempty"`
	Fetched    string  `json:"fetched,omitempty"`
	WordCount  int     `json:"word_count"`
	ReadingMin int     `json:"reading_time"`
	Rendered   bool    `json:"rendered"`
	Markdown   string  `json:"markdown,omitempty"`
	Links      []Link  `json:"links,omitempty"`
	Images     []Image `json:"images,omitempty"`
}

// Link is one outbound hyperlink found in a page's article body, resolved to an
// absolute URL. Internal is true when the link points within the crawl scope, so
// a site build can rewire it to a local file or an in-file anchor.
type Link struct {
	Text     string `json:"text,omitempty"`
	URL      string `json:"url"`
	Internal bool   `json:"internal,omitempty"`
}

// Image is one image found in a page's article body. URL is the final Markdown
// target (an absolute URL, a relative path, or a data URI, per the image policy);
// Local is the on-disk path when the image was downloaded.
type Image struct {
	Alt   string `json:"alt,omitempty"`
	URL   string `json:"url"`
	Local string `json:"local,omitempty"`
}
