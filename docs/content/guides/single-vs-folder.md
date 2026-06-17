---
title: "Single file vs folder"
description: "The two output shapes for yomi site: a folder of files mirroring the URL paths, or one combined Markdown file. When to use each, and how the single file's table of contents, anchors, and heading demotion work."
weight: 30
---

`yomi site` reads a whole site into Markdown in one of two shapes. The default is
a folder of files; `--single` (or `-s`) assembles one combined file. They hold
the same content, organised differently.

## The folder shape (default)

```bash
yomi site paulgraham.com -o pg/
```

A folder crawl writes one `.md` file per page, mirroring the URL paths, with a
`SUMMARY.md` table of contents and a shared `media/` folder:

```
pg/
├── SUMMARY.md            # table of contents, one row per page
├── index.md              # the home page (/)
├── greatwork.md          # /greatwork.html
├── articles.md           # /articles.html
└── media/                # downloaded images, shared across pages
```

In this shape, internal links between crawled pages are rewired to relative
`.md` links: a link to `/articles.html` becomes a link to `articles.md`. The
folder mirrors the site, one file per page, and you can open any single file on
its own.

Use the folder shape when you want each page as its own document: to drop pages
into a notes app or a docs repo, to edit them individually, or to keep the site's
structure as a navigable tree.

## The single shape

```bash
yomi site paulgraham.com --single -o paulgraham.md
```

`--single` collapses the whole crawl into one Markdown file. When the output is a
single file, `-o` is a file path rather than a folder. Use it when you want the
whole site as one document: to read top to bottom, to feed to a tool that wants
one file, or to hand someone a single self-contained artifact.

### Table of contents

The combined file opens with a table of contents listing every page. Each entry
links to that page's section by its in-file anchor, so the top of the document is
an index you can jump from.

### Per-page sections and anchors

Each crawled page becomes its own section in the file, in crawl order. Every
section gets an `<a id>` anchor so the table of contents and any cross-page links
can target it directly.

### Heading demotion

Each page brings its own heading hierarchy, and a page's top heading is usually an
H1. Stacking many pages with their own H1s would produce a document with many
competing top-level headings. To keep one clean outline, yomi demotes each page's
headings: the page title becomes a section heading under the document, and the
page's own headings are pushed down a level beneath it. The result is a single
document with one consistent outline rather than a pile of unrelated H1s.

### How links are rewired

The two shapes rewire internal links differently, because the destination is
different:

- In the **folder** shape, a link to another crawled page becomes a relative
  `.md` link to that page's file (`articles.md`).
- In the **single** shape, a link to another crawled page becomes an in-file
  `#anchor` link that jumps to that page's section in the same document.

In both shapes, links to pages outside the crawl are left pointing at their
original URL.

## Choosing

| You want… | Use |
|-----------|-----|
| Each page as its own editable file | folder (default) |
| The site's structure as a navigable tree | folder (default) |
| The whole site as one readable document | `--single` |
| One self-contained file to hand off or feed to a tool | `--single` |
