package yomi

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// resumeStateName is the sidecar a whole-site crawl writes so an interrupted run
// can continue. It holds one JSON-lines Page record per completed page, the
// simple inspectable form `site` favours, mirroring what the SQLite store does
// for `pack`.
const resumeStateName = ".yomi-state.jsonl"

// resumeStatePath is where the sidecar lives: inside the output folder for a
// folder crawl, and beside the file for a --single crawl.
func resumeStatePath(outRoot string, single bool) string {
	if single {
		out := outRoot
		if !strings.HasSuffix(out, ".md") {
			out += ".md"
		}
		return out + resumeStateName
	}
	return filepath.Join(outRoot, resumeStateName)
}

// loadResumeState reads a sidecar into a map keyed by each page's canonical URL.
// A missing file is not an error: it returns an empty map, so a first --resume
// run with nothing to resume simply starts fresh. A malformed line is skipped.
func loadResumeState(path string) (map[string]*Page, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Page{}, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	done := map[string]*Page{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var p Page
		if err := json.Unmarshal(line, &p); err != nil {
			continue
		}
		done[canonURL(p.URL)] = &p
	}
	return done, sc.Err()
}

// resumeWriter appends completed pages to the sidecar as they are read. Callers
// serialise append through their own mutex.
type resumeWriter struct {
	f  *os.File
	bw *bufio.Writer
}

// openResumeWriter opens the sidecar for appending. When resume is false it
// truncates any stale sidecar, so a fresh crawl starts from an empty state; when
// resume is true it appends, keeping the records a previous run wrote.
func openResumeWriter(path string, resume bool) (*resumeWriter, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	flag := os.O_CREATE | os.O_WRONLY
	if resume {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return nil, err
	}
	return &resumeWriter{f: f, bw: bufio.NewWriter(f)}, nil
}

// append writes one page as a JSON line and flushes it, so an interrupt right
// after a page is read still leaves that page recoverable.
func (w *resumeWriter) append(p *Page) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err := w.bw.Write(b); err != nil {
		return err
	}
	if err := w.bw.WriteByte('\n'); err != nil {
		return err
	}
	return w.bw.Flush()
}

func (w *resumeWriter) close() error {
	if w == nil || w.f == nil {
		return nil
	}
	err := w.bw.Flush()
	if cerr := w.f.Close(); err == nil {
		err = cerr
	}
	w.f = nil
	return err
}
