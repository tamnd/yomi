package yomi

import (
	"path/filepath"
	"testing"
)

func TestResumeStatePath(t *testing.T) {
	if got := resumeStatePath("/out/site", false); got != filepath.Join("/out/site", resumeStateName) {
		t.Errorf("folder path = %q", got)
	}
	if got := resumeStatePath("/out/site", true); got != "/out/site.md"+resumeStateName {
		t.Errorf("single path = %q", got)
	}
	if got := resumeStatePath("/out/site.md", true); got != "/out/site.md"+resumeStateName {
		t.Errorf("single path with .md = %q", got)
	}
}

func TestResumeRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")

	w, err := openResumeWriter(path, false)
	if err != nil {
		t.Fatal(err)
	}
	pages := []*Page{
		samplePage("https://ex.com/", "Home", "home body"),
		samplePage("https://ex.com/a", "A", "a body"),
	}
	for _, p := range pages {
		if err := w.append(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.close(); err != nil {
		t.Fatal(err)
	}

	done, err := loadResumeState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 2 {
		t.Fatalf("loaded %d pages, want 2", len(done))
	}
	got, ok := done[canonURL("https://ex.com/a")]
	if !ok {
		t.Fatal("page /a not recovered, keyed by canonical URL")
	}
	if got.Title != "A" || got.Markdown != "a body" {
		t.Errorf("round trip mismatch: %+v", got)
	}
	// A trailing-slash variant keys to the same record.
	if _, ok := done[canonURL("https://ex.com/a/")]; !ok {
		t.Error("trailing-slash URL should resolve to the same key")
	}
}

func TestResumeAppendKeepsPriorRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")

	w1, err := openResumeWriter(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := w1.append(samplePage("https://ex.com/", "Home", "h")); err != nil {
		t.Fatal(err)
	}
	_ = w1.close()

	// A resume run opens for append and adds more without losing the first.
	w2, err := openResumeWriter(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := w2.append(samplePage("https://ex.com/b", "B", "b")); err != nil {
		t.Fatal(err)
	}
	_ = w2.close()

	done, err := loadResumeState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 2 {
		t.Fatalf("after append got %d pages, want 2", len(done))
	}
}

func TestResumeMissingFileIsEmpty(t *testing.T) {
	done, err := loadResumeState(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err != nil {
		t.Fatalf("missing sidecar should not error: %v", err)
	}
	if len(done) != 0 {
		t.Errorf("missing sidecar = %d pages, want 0", len(done))
	}
}

func TestResumeTruncateDiscardsStale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.jsonl")
	w1, _ := openResumeWriter(path, false)
	_ = w1.append(samplePage("https://ex.com/old", "Old", "o"))
	_ = w1.close()

	// A fresh (non-resume) run truncates the stale sidecar.
	w2, _ := openResumeWriter(path, false)
	_ = w2.append(samplePage("https://ex.com/new", "New", "n"))
	_ = w2.close()

	done, _ := loadResumeState(path)
	if len(done) != 1 {
		t.Fatalf("fresh run kept %d pages, want 1 (stale truncated)", len(done))
	}
	if _, ok := done[canonURL("https://ex.com/new")]; !ok {
		t.Error("fresh run should hold only the new page")
	}
}
