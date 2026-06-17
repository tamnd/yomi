package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLooksJSGated(t *testing.T) {
	rich := []byte("<html><body><article>" +
		strings.Repeat("word ", 60) + "</article></body></html>")
	if _, gated := looksJSGated(rich); gated {
		t.Errorf("rich page wrongly flagged as JS-gated")
	}

	spa := []byte(`<html><body><div id="root"></div></body></html>`)
	reason, gated := looksJSGated(spa)
	if !gated || !strings.Contains(reason, "mount") {
		t.Errorf("empty SPA mount not detected: reason=%q gated=%v", reason, gated)
	}

	noscript := []byte(`<html><body><noscript>You need to enable JavaScript to run this app.</noscript></body></html>`)
	if _, gated := looksJSGated(noscript); !gated {
		t.Errorf("noscript JavaScript notice not detected")
	}

	thin := []byte(`<html><body><p>hi</p></body></html>`)
	if _, gated := looksJSGated(thin); !gated {
		t.Errorf("near-empty body not detected")
	}
}

func TestStaticFetchOff(t *testing.T) {
	body := "<html><body><article>" + strings.Repeat("word ", 60) + "</article></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	f := New(Options{Mode: Off})
	defer func() { _ = f.Close() }()

	res, err := f.Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rendered {
		t.Errorf("off mode should never render")
	}
	if !strings.Contains(string(res.Body), "word") {
		t.Errorf("body not returned:\n%s", res.Body)
	}
}

func TestStaticFetchNonHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	f := New(Options{Mode: Off})
	defer func() { _ = f.Close() }()

	if _, err := f.Fetch(context.Background(), srv.URL); err == nil {
		t.Errorf("expected an error for a non-HTML response")
	}
}
