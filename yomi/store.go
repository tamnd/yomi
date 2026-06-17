package yomi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// store is the SQLite-backed page store that backs the pack command. It is the
// resumable, incremental heart of a pack run: every page read during a crawl is
// written here keyed by its URL, so a re-run skips what is already present and a
// ZIM build reads its pages back out of it. For --format sqlite the store file
// is the deliverable; for --format zim it is a sidecar the archive compiles from.
//
// All access goes through a single underlying connection (SetMaxOpenConns(1)),
// so the crawl's parallel workers serialise their writes through database/sql
// rather than racing for the one writer SQLite allows.
type store struct {
	db   *sql.DB
	path string
}

// storeSchema is the structure every pack store carries. The tables are plain
// and queryable on their own: meta holds the run's provenance, pages holds one
// row per crawled page with its Markdown and metadata, and links and images
// hang off a page by its id.
const storeSchema = `
CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS pages (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  url          TEXT NOT NULL UNIQUE,
  norm         TEXT NOT NULL,
  request_url  TEXT,
  path         TEXT NOT NULL,
  anchor       TEXT,
  title        TEXT,
  byline       TEXT,
  site_name    TEXT,
  excerpt      TEXT,
  lang         TEXT,
  published    TEXT,
  fetched      TEXT,
  word_count   INTEGER NOT NULL DEFAULT 0,
  reading_time INTEGER NOT NULL DEFAULT 0,
  rendered     INTEGER NOT NULL DEFAULT 0,
  depth        INTEGER NOT NULL DEFAULT 0,
  markdown     TEXT NOT NULL DEFAULT '',
  content_hash TEXT,
  updated_at   TEXT
);
CREATE INDEX IF NOT EXISTS pages_norm ON pages(norm);
CREATE INDEX IF NOT EXISTS pages_path ON pages(path);
CREATE TABLE IF NOT EXISTS links (
  page_id  INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
  ord      INTEGER NOT NULL,
  text     TEXT,
  url      TEXT NOT NULL,
  internal INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS links_page ON links(page_id);
CREATE TABLE IF NOT EXISTS images (
  page_id  INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
  ord      INTEGER NOT NULL,
  alt      TEXT,
  url      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS images_page ON images(page_id);
`

// openStore opens (creating if needed) the SQLite store at path and ensures the
// schema exists. The pragmas trade a little durability for crawl throughput,
// which is safe because a pack run can always be repeated.
func openStore(path string) (*store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite pragma %q: %w", pragma, err)
		}
	}
	if _, err := db.Exec(storeSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite schema: %w", err)
	}
	return &store{db: db, path: path}, nil
}

// close folds the write-ahead log back into the main file and closes the
// database, so the store is left as a single self-contained file with no
// sidecar journal.
func (s *store) close() error {
	if s.db == nil {
		return nil
	}
	_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	_, _ = s.db.Exec("PRAGMA journal_mode=DELETE")
	err := s.db.Close()
	s.db = nil
	return err
}

// setMeta writes (or replaces) one provenance key.
func (s *store) setMeta(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, value)
	return err
}

// getMeta reads one provenance key, returning "" when it is absent.
func (s *store) getMeta(key string) (string, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// lookup reports whether a page for rawURL is already stored and still fresh
// enough to skip. A page counts as fresh when maxAge is zero (any stored copy
// is kept) or its fetched time is within maxAge of now. When it is fresh, the
// page's internal links are returned so the crawl can keep walking the graph
// without re-fetching the page itself.
func (s *store) lookup(rawURL string, maxAge time.Duration, now time.Time) (links []string, fresh bool, err error) {
	var (
		id      int64
		fetched sql.NullString
	)
	row := s.db.QueryRow("SELECT id, fetched FROM pages WHERE norm=? LIMIT 1", canonURL(rawURL))
	switch err = row.Scan(&id, &fetched); err {
	case sql.ErrNoRows:
		return nil, false, nil
	case nil:
	default:
		return nil, false, err
	}
	if maxAge > 0 && stale(fetched.String, maxAge, now) {
		return nil, false, nil
	}
	links, err = s.internalLinks(id)
	return links, true, err
}

// internalLinks returns the in-scope link targets recorded for a stored page,
// in their original order.
func (s *store) internalLinks(pageID int64) ([]string, error) {
	rows, err := s.db.Query("SELECT url FROM links WHERE page_id=? AND internal=1 ORDER BY ord", pageID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// put writes a page and its links and images, replacing any earlier copy of the
// same URL. The whole write is one transaction so a reader never sees a page
// without its links, and an interrupted crawl leaves a consistent file.
func (s *store) put(ctx context.Context, p *Page, depth int, updatedAt string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	hash := sha256.Sum256([]byte(p.Markdown))
	var id int64
	err = tx.QueryRowContext(ctx, `
INSERT INTO pages(url, norm, request_url, path, anchor, title, byline, site_name, excerpt,
                  lang, published, fetched, word_count, reading_time, rendered, depth,
                  markdown, content_hash, updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(url) DO UPDATE SET
  norm=excluded.norm, request_url=excluded.request_url, path=excluded.path, anchor=excluded.anchor,
  title=excluded.title, byline=excluded.byline, site_name=excluded.site_name, excerpt=excluded.excerpt,
  lang=excluded.lang, published=excluded.published, fetched=excluded.fetched,
  word_count=excluded.word_count, reading_time=excluded.reading_time, rendered=excluded.rendered,
  depth=excluded.depth, markdown=excluded.markdown, content_hash=excluded.content_hash,
  updated_at=excluded.updated_at
RETURNING id`,
		p.URL, canonURL(p.URL), p.RequestURL, p.Path, p.Anchor, p.Title, p.Byline, p.SiteName, p.Excerpt,
		p.Lang, p.Published, p.Fetched, p.WordCount, p.ReadingMin, boolToInt(p.Rendered), depth,
		p.Markdown, hex.EncodeToString(hash[:]), updatedAt,
	).Scan(&id)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM links WHERE page_id=?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM images WHERE page_id=?", id); err != nil {
		return err
	}
	for i, l := range p.Links {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO links(page_id, ord, text, url, internal) VALUES(?,?,?,?,?)",
			id, i, l.Text, l.URL, boolToInt(l.Internal)); err != nil {
			return err
		}
	}
	for i, im := range p.Images {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO images(page_id, ord, alt, url) VALUES(?,?,?,?)",
			id, i, im.Alt, im.URL); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// counts returns the number of stored pages and the sum of their word counts.
func (s *store) counts() (pages, words int, err error) {
	err = s.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(word_count), 0) FROM pages").Scan(&pages, &words)
	return
}

// allPages loads every stored page with its links and images, ordered the same
// way a folder or single-file site build orders them: shallower paths first,
// ties broken by path, so a ZIM built from the store is deterministic.
func (s *store) allPages() ([]*Page, error) {
	rows, err := s.db.Query(`
SELECT id, url, request_url, path, anchor, title, byline, site_name, excerpt, lang, published,
       fetched, word_count, reading_time, rendered, markdown
FROM pages`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pages []*Page
	ids := map[int64]*Page{}
	for rows.Next() {
		var (
			id         int64
			p          Page
			requestURL sql.NullString
			anchor     sql.NullString
			byline     sql.NullString
			siteName   sql.NullString
			excerpt    sql.NullString
			lang       sql.NullString
			published  sql.NullString
			fetched    sql.NullString
			rendered   int
		)
		if err := rows.Scan(&id, &p.URL, &requestURL, &p.Path, &anchor, &p.Title, &byline, &siteName,
			&excerpt, &lang, &published, &fetched, &p.WordCount, &p.ReadingMin, &rendered, &p.Markdown); err != nil {
			return nil, err
		}
		p.RequestURL = requestURL.String
		p.Anchor = anchor.String
		p.Byline = byline.String
		p.SiteName = siteName.String
		p.Excerpt = excerpt.String
		p.Lang = lang.String
		p.Published = published.String
		p.Fetched = fetched.String
		p.Rendered = rendered != 0
		pp := &p
		pages = append(pages, pp)
		ids[id] = pp
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.loadLinks(ids); err != nil {
		return nil, err
	}
	if err := s.loadImages(ids); err != nil {
		return nil, err
	}
	sortPages(pages)
	return pages, nil
}

func (s *store) loadLinks(ids map[int64]*Page) error {
	rows, err := s.db.Query("SELECT page_id, text, url, internal FROM links ORDER BY page_id, ord")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			pid      int64
			l        Link
			text     sql.NullString
			internal int
		)
		if err := rows.Scan(&pid, &text, &l.URL, &internal); err != nil {
			return err
		}
		l.Text = text.String
		l.Internal = internal != 0
		if p := ids[pid]; p != nil {
			p.Links = append(p.Links, l)
		}
	}
	return rows.Err()
}

func (s *store) loadImages(ids map[int64]*Page) error {
	rows, err := s.db.Query("SELECT page_id, alt, url FROM images ORDER BY page_id, ord")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var (
			pid int64
			im  Image
			alt sql.NullString
		)
		if err := rows.Scan(&pid, &alt, &im.URL); err != nil {
			return err
		}
		im.Alt = alt.String
		if p := ids[pid]; p != nil {
			p.Images = append(p.Images, im)
		}
	}
	return rows.Err()
}

// canonURL reduces a URL to a stable key for resume matching: lower-case scheme
// and host, no fragment, and no trailing slash except on the bare root. It is
// deliberately simple and self-contained, so the same page maps to the same key
// across runs. A redirect that changes the path still re-fetches once, which is
// correct because the unique URL keeps the row from duplicating.
func canonURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if len(u.Path) > 1 {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String()
}

// stale reports whether a stored fetched timestamp is older than maxAge before
// now. An unparseable or empty timestamp is treated as stale, so a malformed row
// is refreshed rather than trusted forever.
func stale(fetched string, maxAge time.Duration, now time.Time) bool {
	if fetched == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, fetched)
	if err != nil {
		return true
	}
	return now.Sub(t) > maxAge
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
