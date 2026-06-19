---
title: "Quick start"
description: "From an empty terminal to clean Markdown: one page, a whole site as a folder, and a single combined file."
weight: 30
---

This walks the core loop: read one page, save it to a file, read a whole site
into a folder, collapse a site into one file, and preview a folder in the
browser.

## 1. Read one page

```bash
yomi read paulgraham.com/greatwork.html
```

A bare host is enough; yomi fills in `https://` when you leave the scheme off.
It fetches the page, renders it only if it looks JavaScript-gated, extracts the
article, and prints Markdown to stdout. The document opens with a front-matter
block:

```markdown
---
title: "How to Do Great Work"
url: "https://paulgraham.com/greatwork.html"
byline: "Paul Graham"
fetched: "2026-06-17T09:30:00Z"
word_count: 11856
reading_time: 59
---

If you collected lists of techniques for doing great work in a lot
of different fields, what would the intersection look like? ...
```

Strings are quoted, `reading_time` is the estimate in whole minutes, and only the
fields the page actually provides are written. This essay carries no site name,
language, or published date, so those lines are simply absent.

## 2. Save it to a file

```bash
yomi read paulgraham.com/greatwork.html -o greatwork.md
```

`-o` writes the Markdown to a file instead of stdout. Open `greatwork.md` in any
editor and you have the essay, no nav, no footer, no cookie banner.

## 3. Read a whole site into a folder

```bash
yomi site paulgraham.com -o pg/
```

yomi crawls the site in scope, writes one `.md` file per page mirroring the URL
paths, and adds a `SUMMARY.md` table of contents. Downloaded images, if you ask
for them, share a `media/` folder.

```
pg/
├── SUMMARY.md            # table of contents, one row per page
├── index.md              # the home page (/)
├── greatwork.md          # /greatwork.html
├── articles.md           # /articles.html
└── media/                # shared images (with --images download)
```

`SUMMARY.md` is a plain Markdown list linking the pages by title, so the folder
reads like a small book.

## 4. Collapse a site into one file

```bash
yomi site paulgraham.com --single -o paulgraham.md
```

`--single` (or `-s`) assembles the whole crawl into one Markdown document: a
table of contents at the top, then every page as its own section with an anchor,
with each page's headings demoted so the file keeps a single clean outline. One
file you can read top to bottom.

## 5. Pack a site into one file

```bash
yomi pack paulgraham.com -o pg.db     # a SQLite database of the whole site
yomi pack paulgraham.com -o pg.zim    # a ZIM archive you can open in Kiwix
yomi pack paulgraham.com -o pg.epub   # an EPUB book for an e-reader
```

`yomi pack` bundles a whole crawl into one file instead of a folder: a SQLite
database with `pages`, `links`, and `images` tables, a ZIM offline archive, or an
EPUB book. The output extension picks the format. The crawl resumes, so running it
again keeps every page already stored and fetches only what is new.

## 6. Preview a folder

```bash
yomi serve pg/
# open http://127.0.0.1:8800
```

`yomi serve` runs a local static file server over a folder of Markdown so you can
click through it in a browser.

## Where to go next

- The [guides](/guides/) cover reading a page, crawling a site, packing a site
  into one file, the single vs folder choice, and images in depth.
- The [CLI reference](/reference/cli/) lists every command and flag.
