package yomi

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// imageSink builds the mdconv image callback for the configured policy. It also
// records each handled image on the Page. For ImageDownload it writes files into
// rw.imageDir and returns a Markdown path under rw.imageRel; with no directory it
// falls back to leaving the image remote.
func (r *reader) imageSink(ctx context.Context, base *url.URL, rw rewrite, p *Page) func(abs, alt string) string {
	switch r.opts.Images {
	case ImageDownload:
		if rw.imageDir == "" {
			return r.remoteSink(p)
		}
		return r.downloadSink(ctx, rw, p)
	case ImageInline:
		return r.inlineSink(ctx, p)
	default:
		return r.remoteSink(p)
	}
}

// remoteSink records the image and keeps its absolute URL.
func (r *reader) remoteSink(p *Page) func(abs, alt string) string {
	return func(abs, alt string) string {
		p.Images = append(p.Images, Image{Alt: alt, URL: abs})
		return "" // empty target tells mdconv to keep the absolute URL
	}
}

// downloadSink fetches each image into the page's image directory and rewrites
// the Markdown target to a relative path. A failed download falls back to the
// remote URL so the article still renders.
func (r *reader) downloadSink(ctx context.Context, rw rewrite, p *Page) func(abs, alt string) string {
	if err := os.MkdirAll(rw.imageDir, 0o755); err != nil {
		r.opts.log("image dir: %v", err)
		return r.remoteSink(p)
	}
	return func(abs, alt string) string {
		u, err := url.Parse(abs)
		if err != nil {
			return r.recordRemote(p, abs, alt)
		}
		res, err := r.dl.Get(ctx, u, "")
		if err != nil {
			r.opts.log("image skipped (%v): %s", err, abs)
			return r.recordRemote(p, abs, alt)
		}
		name := imageName(abs, res.ContentType)
		full := filepath.Join(rw.imageDir, name)
		if err := os.WriteFile(full, res.Body, 0o644); err != nil {
			r.opts.log("image write: %v", err)
			return r.recordRemote(p, abs, alt)
		}
		target := path.Join(rw.imageRel, name)
		p.Images = append(p.Images, Image{Alt: alt, URL: target, Local: full})
		return target
	}
}

// inlineSink embeds each image as a data: URI so the Markdown is self-contained.
func (r *reader) inlineSink(ctx context.Context, p *Page) func(abs, alt string) string {
	return func(abs, alt string) string {
		u, err := url.Parse(abs)
		if err != nil {
			return r.recordRemote(p, abs, alt)
		}
		res, err := r.dl.Get(ctx, u, "")
		if err != nil {
			r.opts.log("image skipped (%v): %s", err, abs)
			return r.recordRemote(p, abs, alt)
		}
		ct := res.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		data := "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(res.Body)
		p.Images = append(p.Images, Image{Alt: alt, URL: "data:" + ct})
		return data
	}
}

func (r *reader) recordRemote(p *Page, abs, alt string) string {
	p.Images = append(p.Images, Image{Alt: alt, URL: abs})
	return abs
}

// imageName derives a stable, collision-resistant filename for a downloaded
// image from its URL, keeping a sensible extension from the URL or content type.
func imageName(abs, contentType string) string {
	sum := sha1.Sum([]byte(abs))
	stub := fmt.Sprintf("%x", sum[:6])
	ext := path.Ext(path.Base(abs))
	if ext == "" || len(ext) > 5 {
		ext = extForType(contentType)
	}
	return stub + ext
}

func extForType(ct string) string {
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "svg"):
		return ".svg"
	default:
		return ".img"
	}
}
