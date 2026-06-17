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
