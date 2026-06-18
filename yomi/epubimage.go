package yomi

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// embeddedImage is an image that was pulled into the EPUB container: its path
// under OEBPS and the media type recorded for it in the package manifest.
type embeddedImage struct {
	name      string // e.g. "images/img1.png", relative to OEBPS
	mediaType string
}

// epubImageAsset is one embedded image with its bytes, ready to write into the
// archive and list in the manifest.
type epubImageAsset struct {
	id        string
	name      string
	mediaType string
	data      []byte
}

// imageSet is the result of fetching every image referenced by a book: a lookup
// from the original src to its embedded copy, and the ordered assets to write.
type imageSet struct {
	bySrc  map[string]embeddedImage
	assets []epubImageAsset
}

// epubImageExt maps the EPUB core image media types to the file extension used
// for the embedded copy. EPUB 3 allows exactly these raster and vector types as
// content-document images, so anything that does not resolve to one of them is
// dropped rather than embedded.
var epubImageExt = map[string]string{
	"image/jpeg":    ".jpg",
	"image/png":     ".png",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/svg+xml": ".svg",
}

// fetchImages pulls every referenced image into memory so it can be embedded in
// the EPUB. Remote image references are not allowed in an EPUB content document,
// so a high-quality book carries its images inside the container; this is where
// they are gathered. The srcs are visited in their given order and named
// deterministically by success order, so the same crawl produces the same book.
// An image that fails to fetch, exceeds the size cap, or is not a supported image
// type is simply left out of the set, and the caller drops its <img> tag.
func fetchImages(ctx context.Context, srcs []string, opts Options, logf func(string, ...any)) imageSet {
	maxBytes := opts.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = 16 << 20
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	type result struct {
		src       string
		data      []byte
		mediaType string
		ext       string
	}
	results := make([]*result, len(srcs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, src := range srcs {
		wg.Add(1)
		go func(i int, src string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, mediaType, ext, ok := fetchOneImage(ctx, client, src, ua, maxBytes)
			if !ok {
				logf("epub: skipped image %s", src)
				return
			}
			results[i] = &result{src: src, data: data, mediaType: mediaType, ext: ext}
		}(i, src)
	}
	wg.Wait()

	set := imageSet{bySrc: map[string]embeddedImage{}}
	n := 0
	for _, r := range results {
		if r == nil {
			continue
		}
		if _, done := set.bySrc[r.src]; done {
			continue
		}
		n++
		id := fmt.Sprintf("img%d", n)
		name := "images/" + id + r.ext
		set.bySrc[r.src] = embeddedImage{name: name, mediaType: r.mediaType}
		set.assets = append(set.assets, epubImageAsset{
			id: id, name: name, mediaType: r.mediaType, data: r.data,
		})
	}
	return set
}

// fetchOneImage retrieves a single image and classifies it. A data: URI is
// decoded in place; anything else is fetched over HTTP. It returns the bytes, the
// media type, and the file extension, or ok=false when the resource cannot be
// embedded as a supported image.
func fetchOneImage(ctx context.Context, client *http.Client, src, ua string, maxBytes int64) (data []byte, mediaType, ext string, ok bool) {
	if strings.HasPrefix(src, "data:") {
		return decodeDataImage(src, maxBytes)
	}
	u := src
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return nil, "", "", false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", "", false
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil || int64(len(body)) > maxBytes || len(body) == 0 {
		return nil, "", "", false
	}
	mediaType, ext, ok = classifyImage(resp.Header.Get("Content-Type"), body, src)
	if !ok {
		return nil, "", "", false
	}
	return body, mediaType, ext, true
}

// decodeDataImage decodes a base64 data: URI into image bytes.
func decodeDataImage(src string, maxBytes int64) (data []byte, mediaType, ext string, ok bool) {
	comma := strings.IndexByte(src, ',')
	if comma < 0 {
		return nil, "", "", false
	}
	header := src[len("data:"):comma]
	payload := src[comma+1:]
	if !strings.Contains(header, "base64") {
		return nil, "", "", false
	}
	mt := strings.TrimSpace(strings.Split(header, ";")[0])
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(decoded) == 0 || int64(len(decoded)) > maxBytes {
		return nil, "", "", false
	}
	mediaType, ext, ok = classifyImage(mt, decoded, "")
	if !ok {
		return nil, "", "", false
	}
	return decoded, mediaType, ext, true
}

// classifyImage decides the media type and extension for an image from its
// declared content type, its bytes, and its URL, returning ok=false when it is
// not one of the EPUB core image types. The declared type is trusted first, then
// the bytes are sniffed, and SVG is recognised from the URL or a leading <svg.
func classifyImage(contentType string, data []byte, src string) (mediaType, ext string, ok bool) {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if e, found := epubImageExt[ct]; found {
		return ct, e, true
	}

	sniff := strings.Split(http.DetectContentType(data), ";")[0]
	if e, found := epubImageExt[sniff]; found {
		return sniff, e, true
	}

	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	if strings.Contains(strings.ToLower(src), ".svg") || bytes.Contains(head, []byte("<svg")) {
		return "image/svg+xml", ".svg", true
	}
	return "", "", false
}
