---
title: "Packing a site into one file"
description: "Bundle a whole crawl into one SQLite database or one ZIM archive with yomi pack, and keep it current with a resumable, incremental crawl."
weight: 25
---

`yomi site` writes a folder of Markdown files. `yomi pack` writes one file: a
crawl of the whole site bundled into a single SQLite database, or a single ZIM
archive you can open in [Kiwix](https://kiwix.org). Both are backed by the same
database, so a pack resumes where it left off and a later run only fetches what
changed.

```bash
# A SQLite database of the whole site (the default format)
yomi pack paulgraham.com -o pg.db

# A ZIM offline archive, browsable in Kiwix
yomi pack paulgraham.com -o pg.zim
```

The output extension picks the format. `-o pg.zim` builds a ZIM and `-o pg.db`
builds a database without you passing `--format` as well. With no `-o`, pack
writes `<host>.db` (or `<host>.zim` when `--format zim` is set). An explicit
`--format` always wins over the extension.

## SQLite: a site you can query

The default format is a SQLite database with clean, structured tables. Every page
is a row in `pages`, and its links and images live in `links` and `images` tables
that join back by `page_id`.

```bash
yomi pack paulgraham.com -o pg.db
```

The `pages` table carries one row per page: its `url`, `title`, `byline`,
`site_name`, `excerpt`, `lang`, `published`, `fetched`, `word_count`,
`reading_time`, `depth`, and the `markdown` body. So a query over the site is one
line of SQL:

```bash
# The five longest essays
sqlite3 pg.db "select title, word_count, reading_time from pages order by word_count desc limit 5;"

# Every outbound link the author wrote, across the whole site
sqlite3 pg.db "select p.title, l.url from links l join pages p on p.id = l.page_id where l.internal = 0;"
```

A `meta` table records the crawl itself: the seed, the host, when it was created
and last updated, the page count, and the yomi version that built it.

## ZIM: a site you can read offline

A ZIM archive is the format to reach for when you want to read the site offline.
pack renders each page to a self-contained HTML document, rewires the in-scope
links to point at the sibling entries, generates a contents page as the landing
page, and writes one [OpenZIM](https://openzim.org) file.

```bash
yomi pack paulgraham.com -o pg.zim
```

Open the result in Kiwix on any device, or serve it over HTTP:

```bash
kiwix-serve --port 8080 pg.zim
```

A ZIM build keeps its SQLite store next to the archive as a sidecar (`pg.db` for
`pg.zim`), so the next run is incremental too. Point `--state` somewhere else to
keep the store apart from the archive.

The ZIM metadata flags set what a reader sees in Kiwix:

```bash
yomi pack paulgraham.com -o pg.zim \
  --title "Paul Graham's Essays" \
  --description "An offline archive of paulgraham.com" \
  --language eng \
  --date 2026-06-18
```

`--title` defaults to the home page title, `--language` to `eng`, and `--date` to
today. Pass `--no-compress` to store every entry raw, which makes a larger file
that opens without decompression.

## The crawl resumes

A pack is resumable because the database is the crawl's own backing store. Run
pack again over the same output and it keeps every page already stored, fetching
only pages it has not seen:

```bash
yomi pack paulgraham.com -o pg.db   # first run: reads the whole site
yomi pack paulgraham.com -o pg.db   # again: new 0, every page kept
```

The summary line reports `new` (pages fetched this run) and `kept` (pages already
stored and skipped without re-fetching). On a settled site the second run reads
nothing. If a run is interrupted, the pages it had already written stay in the
store, so re-running it picks up the rest rather than starting over.

## Keeping a pack current

Two flags drive a refresh.

`--refresh` re-fetches every page, ignoring what is stored. Reach for it when the
whole site has changed and you want a clean rebuild:

```bash
yomi pack paulgraham.com -o pg.db --refresh
```

`--max-age` re-fetches only the pages older than a cutoff, leaving fresher ones
untouched. A daily mirror stays current without reading the whole site each time:

```bash
yomi pack paulgraham.com -o pg.db --max-age 24h
```

Anything stored longer ago than the duration is re-read; everything newer is
kept. Without either flag a stored page is never re-fetched.

## Scope, limits, and politeness

pack takes the same scope and crawl controls as
[`yomi site`](/guides/crawling-a-site/), and they mean the same thing:

```bash
# Just one section, two hundred pages at most, ignoring a subtree
yomi pack go.dev -o go.db --scope-prefix /doc --max-pages 200 --exclude /blog

# Pull in subdomains, eight workers
yomi pack example.com -o example.zim --subdomains --workers 8
```

`--scope-prefix`, `--max-pages`, `--max-depth`, `--subdomains`, `--exclude`,
`--workers`, and `--no-robots` all behave exactly as they do for a folder crawl.
A pack honours `robots.txt` by default.

## Which format

Reach for **SQLite** when you want to query the site, feed it to a tool, or keep a
structured record you can diff and join. Reach for **ZIM** when you want to read
the site offline in Kiwix on a phone, a laptop, or a server. Either way the crawl
is the same, and the SQLite store is always there to resume from.
