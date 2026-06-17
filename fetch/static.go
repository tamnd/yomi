package fetch

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tamnd/kage/browser"
	"golang.org/x/net/html/charset"
)

// staticFetcher fetches a page over plain HTTP and decodes it to UTF-8. It is
// the default backend and is correct for the bulk of server-rendered pages.
type staticFetcher struct {
	client    *http.Client
	userAgent string
}

func newStatic(userAgent string, timeout time.Duration) *staticFetcher {
	return &staticFetcher{
		client:    &http.Client{Timeout: timeout},
		userAgent: userAgent,
	}
}

// maxStaticBytes caps a static page read so a misrouted large file cannot exhaust
// memory before the content-type check rejects it.
const maxStaticBytes = 8 << 20

// fetch retrieves url, returning a typed *browser.ErrNotHTML when the response is
// not an HTML document so the caller can map it to the right exit code.
func (s *staticFetcher) fetch(ctx context.Context, url string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode/100 != 2 {
		return nil, &asStatus{code: resp.StatusCode, url: url}
	}

	ct := resp.Header.Get("Content-Type")
	if !isHTML(ct) {
		return nil, &browser.ErrNotHTML{URL: url, ContentType: firstField(ct)}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxStaticBytes))
	if err != nil {
		return nil, err
	}
	body := decodeUTF8(raw, ct)

	final := url
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return &Response{URL: final, Body: body, MIME: firstField(ct)}, nil
}

// isHTML reports whether a Content-Type names an HTML document. An empty type is
// treated as HTML, since some servers omit it for pages.
func isHTML(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if ct == "" {
		return true
	}
	return strings.HasPrefix(ct, "text/html") || strings.HasPrefix(ct, "application/xhtml")
}

// decodeUTF8 reads raw through a charset-aware reader so a non-UTF-8 page becomes
// valid UTF-8. On any decode error it returns the bytes unchanged.
func decodeUTF8(raw []byte, contentType string) []byte {
	r, err := charset.NewReader(bytes.NewReader(raw), contentType)
	if err != nil {
		return raw
	}
	out, err := io.ReadAll(r)
	if err != nil {
		return raw
	}
	return out
}

func firstField(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.TrimSpace(ct)
}

// asStatus reports a non-2xx HTTP response from the static fetcher.
type asStatus struct {
	code int
	url  string
}

func (e *asStatus) Error() string {
	return fmt.Sprintf("http %d for %s", e.code, e.url)
}

// Code returns the HTTP status code, so callers can recognise a block (403/429).
func (e *asStatus) Code() int { return e.code }
