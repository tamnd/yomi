---
title: "Crawling a site"
description: "Read a whole site into Markdown with yomi site: scope, robots, limits, workers, how links are rewired, and the media folder."
weight: 20
---

`yomi read` handles one page. `yomi site` reads a whole site: it crawls
breadth-first from a seed URL, reads each in-scope page into Markdown, and writes
the result as a folder (the default) or a single file (with `--single`). This
guide covers the default folder shape and how to keep the crawl in bounds; the
[single vs folder](/guides/single-vs-folder/) guide covers the two output
shapes.

```bash
yomi site paulgraham.com -o pg/
```

By default `-o` defaults to the host name, so `yomi site paulgraham.com` writes
into `paulgraham.com/` if you do not pass one.

## What lands on disk

A folder crawl writes one `.md` file per page, mirroring the URL paths, plus a
table of contents and a shared media folder:

```
pg/
├── SUMMARY.md            # table of contents, one row per page
├── index.md              # the home page (/)
├── greatwork.md          # /greatwork.html
├── articles.md           # /articles.html
└── media/                # downloaded images, shared across pages
```

`SUMMARY.md` is a plain Markdown list linking every page by its title, so the
folder reads like a small book.

## A structured dataset instead of a folder

When you want the crawl as data rather than as files to read, `--format json` or
`--format jsonl` writes one structured file of every page instead of a folder:

```bash
# One JSON array of every page
yomi site paulgraham.com --format json -o pg.json

# One page record per line, for streaming
yomi site paulgraham.com --format jsonl -o pg.jsonl
```

Each record carries the page's full metadata, its links and images, and its
Markdown body, so the dataset is the whole reading, not just an index. `jsonl` is
the form to pipe into `jq` or load row by row; `json` is one pretty-printed array.
With no `-o`, the dataset is written to `<host>.json` or `<host>.jsonl`. For a
single readable file or an offline archive instead, see
[`--single`](/guides/single-vs-folder/) and [`yomi pack`](/guides/packing-a-site/).

## Scope

By default a crawl stays on the exact seed host and reads every in-scope page it
can reach. These flags bound it.

### Subdomains

To treat subdomains of the seed as in scope:

```bash
yomi site example.com --subdomains
```

Now `blog.example.com` and `docs.example.com` are crawled too.

### A path prefix

To read just one section of a site, restrict the crawl to a path prefix:

```bash
yomi site example.com --scope-prefix /docs
```

Only pages whose path starts with `/docs` are followed.

### Excluding paths

To skip parts of a site, exclude path prefixes. The flag is repeatable:

```bash
yomi site example.com --exclude /archive --exclude /tags
```

## Limits

```bash
# Stop after 200 pages
yomi site example.com --max-pages 200

# Only follow links three hops from the seed
yomi site example.com --max-depth 3
```

`--max-pages 0` (the default) means unlimited pages; `--max-depth 0` (the
default) means unlimited depth. Combine them to put a hard ceiling on a run.

## Workers

yomi reads pages in parallel. The default is four concurrent workers; raise or
lower it with `--workers`:

```bash
yomi site example.com --workers 8
```

## Robots

yomi honours `robots.txt` by default, the same as kage. If you are reading a site
you control, or you have a reason to ignore the robots rules, you can turn them
off, but do so responsibly:

```bash
yomi site example.com --no-robots
```

## Seeding from the sitemap

A breadth-first walk only reaches pages that are linked from other pages. Many
sites also publish a `sitemap.xml` that lists pages which may not be linked from
anywhere obvious. `--sitemap` seeds the crawl from it before the link walk begins:

```bash
yomi site example.com --sitemap
```

yomi reads the site's `/sitemap.xml` and any `Sitemap:` lines in `robots.txt`,
follows a sitemap index one level down to the sitemaps it points at, and queues
every in-scope, page-like URL it lists. The link walk then proceeds as usual, so
the sitemap adds reach without changing anything else. A site with no sitemap is
unaffected, so the flag is always safe to pass. `yomi pack` takes it too.

## Resuming an interrupted crawl

A folder or single-file Markdown crawl is resumable. As it reads each page, yomi
writes it to disk and records it in a small `.yomi-state.jsonl` sidecar. If a run
is interrupted, re-run it with `--resume` and it picks up where it left off,
skipping the pages already done instead of reading the whole site again:

```bash
yomi site paulgraham.com -o pg/            # interrupted partway through
yomi site paulgraham.com -o pg/ --resume   # continues, skipping finished pages
```

The sidecar lives inside the output folder (or beside the file for `--single`). A
run without `--resume` starts fresh and clears any stale sidecar. This mirrors how
[`yomi pack`](/guides/packing-a-site/) resumes from its SQLite store; the
`--format json`/`jsonl` datasets are written whole each run and are not resumable.

## How internal links are rewired

A page in the crawl often links to other pages in the same crawl. yomi rewires
those in-scope links so they point at the other Markdown files instead of the
live web: a link to `/articles.html` in the folder output becomes a relative link
to `articles.md`. Links to pages outside the crawl are left pointing at their
original URL. The result is a folder you can navigate without going back online.
(In `--single` output the same internal links become in-file `#anchor` links;
see [single vs folder](/guides/single-vs-folder/).)

## The media folder

When you download images with `--images download`, a site crawl puts them in one
shared `media/` folder at the root of the output, and every page's image links
point into it. Sharing one folder means an image used on several pages is stored
once. The [images guide](/guides/images/) covers the image policies and the size
cap in full.

## The shared read flags apply

Everything from [reading a page](/guides/reading-a-page/), the render mode,
front-matter, title heading, wrap, links style, images, timeout, user agent, and
the browser flags, applies to `yomi site` too. They are applied to every page in
the crawl:

```bash
yomi site example.com --render off --images download --wrap 80 -o site/
```
