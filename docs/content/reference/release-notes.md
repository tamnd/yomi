---
title: "Release notes"
description: "What changed in each yomi release."
weight: 40
---

The authoritative, commit-level history lives in [`CHANGELOG.md`](https://github.com/tamnd/yomi/blob/main/CHANGELOG.md) and on the [releases page](https://github.com/tamnd/yomi/releases). This page summarises each version.

## Unreleased

A round on input and output: more ways to feed yomi a page, and more shapes to get one back.

- **An EPUB book.** `yomi pack --format epub` (or a `.epub` output name) builds an EPUB 3 book from a crawl, readable on any e-reader. Each page becomes a well-formed XHTML chapter, in-scope links are rewired to sibling chapters, every referenced image is pulled into the book so it reads with no network, a generated navigation document lists every page, and a drawn-in-code cover stands in front. The book passes EPUBCheck, the official validator, with no errors or warnings, and carries accessibility metadata. `--icon` supplies your own cover, and `--title`, `--language`, and `--date` fill the book's metadata. The crawl keeps its SQLite store as a sidecar for the next incremental run, the same as a ZIM build.
- **More ways in.** `yomi read -` reads HTML from standard input and `yomi read page.html` reads a local file, so you can convert a page you already have without a fetch. `--base` sets the URL that relative links resolve against.
- **More shapes out.** `yomi read -f json|jsonl` emits the full page record instead of a Markdown document, and `-f html` emits a self-contained HTML article. `yomi site --format json|jsonl` writes one dataset file of every crawled page, for feeding a pipeline.
- **Reach more pages.** `yomi site --sitemap` (and `pack --sitemap`) seeds the crawl from the site's `sitemap.xml` and any `robots.txt` `Sitemap:` lines, following an index one level, so the crawl reaches pages that are listed but not linked.
- **Resume a site crawl.** `yomi site --resume` continues an interrupted Markdown crawl. The crawl records each page in a `.yomi-state.jsonl` sidecar as it goes, so a re-run skips the pages already done, the way `pack` resumes from its store.

## v0.2.1

A pass to make a packed ZIM open nicely in Kiwix.

- **A library icon.** Every ZIM build now embeds a 48x48 icon, so Kiwix shows a real book tile instead of a blank placeholder. yomi draws a built-in reading icon by default; `--icon` with a PNG uses the site's own logo instead.
- **Metadata Kiwix reads.** The archive now carries a `Counter` of the packed pages for the library listing, `Creator` is the packed site rather than the tool version, and `Scraper` names yomi and its version, so the three keys read the way Kiwix expects.

## v0.2.0

A new way to keep a whole site: one file instead of a folder.

- **`yomi pack <url>`** crawls a site and bundles it into one file. The default is a SQLite database with clean `pages`, `links`, and `images` tables you can query with SQL; `--format zim` (or a `.zim` output name) builds a ZIM offline archive you can read in Kiwix, with each page rendered to a self-contained HTML document, internal links rewired to the sibling entries, and a generated contents page.
- **Resumable, incremental crawls.** The crawl is backed by the SQLite store, so a pack resumes where it left off and a later run keeps every page already stored, fetching only what is new. `--refresh` re-reads everything; `--max-age` re-reads only the pages older than a cutoff, so a daily mirror stays current without reading the whole site. A ZIM build keeps its store as a sidecar for the next run.
- **The format picks itself.** A `.db`/`.sqlite` or `.zim` output name selects the format, so `-o site.zim` just builds a ZIM. ZIM metadata flags (`--title`, `--description`, `--language`, `--date`) and `--no-compress` round out the archive, and pack takes the same scope, limit, worker, and robots flags as `yomi site`.

## v0.1.2

A small pass for the people actually typing the commands.

- **Bare hosts read.** `yomi read`, `yomi meta`, and `yomi links` now accept a URL with no scheme: `yomi read example.com/post` defaults to `https://`, the shorthand `yomi site` already took. Every example in the docs uses it, so now they all run exactly as written.
- **Friendlier docs.** A rewritten README with a terminal demo of the read, save, and metadata loop, and a warmer pass over the homepage and guides.

## v0.1.1

A quality pass on the extracted Markdown, a cleaner `yomi meta`, and docs that match the real output.

- **Cleaner article Markdown.** A caption that only repeats an image's alt text is dropped, a picture linked to its own full-size file is unwrapped to a plain image, and a stray share or subscribe button left in the body is removed. A bare angle bracket the converter had escaped is restored in prose (so `a -> b` and `<placeholder>` read as written), a needless backslash before an underscore or tilde is dropped, and a link with no destination renders as its text alone. Code blocks keep their bytes exactly.
- **`yomi meta` is just metadata.** The JSON record no longer carries an empty `markdown` field. It is the page's `url`, `title`, `byline`, `site_name`, `excerpt`, `lang`, `published`, `fetched`, `word_count`, `reading_time`, and `rendered`, followed by `links` and `images` as arrays of objects.
- **Docs match the code.** The front-matter and `meta` examples now show quoted string values, `reading_time` as a whole-minute number, and `links` and `images` as arrays rather than counts.

## v0.1.0

The first release. yomi reads a web page, or a whole website, into clean Markdown: fetch the page, render the JavaScript only when the page needs it, extract the main content, and convert what is left to GitHub-Flavored Markdown with a YAML front-matter block.

- **`yomi read <url>`** turns one page into Markdown, printing to stdout or writing to a file with `-o`. The default `--render auto` static-fetches first and escalates to headless Chrome only when the page looks JavaScript-gated, so most reads never launch a browser. Readability extraction drops the nav, cookie banners, footers, and share rails before conversion.
- **`yomi site <url>`** reads a whole site. The default output is a folder of `.md` files mirroring the URL paths, with a `SUMMARY.md` table of contents and a shared `media/` folder; `--single` assembles one combined file with a table of contents, per-page sections and anchors, and demoted headings for a single clean outline.
- **`yomi meta <url>`** prints a page's metadata record as JSON, and **`yomi links <url>`** lists the outbound links in the article body, with `--json` for a structured list.
- **`yomi serve [dir]`** previews a folder of Markdown over a local static file server.
- **Three image policies.** `--images remote` (the default) leaves image URLs absolute, `download` fetches them next to the output, and `inline` embeds them as base64 data URIs, with a `--max-image-mb` cap.
- **Polite by default.** Honours `robots.txt`, scopes to the seed host, and reads pages with four parallel workers.
- **Packaged everywhere.** Archives, `.deb`/`.rpm`/`.apk`, a multi-arch GHCR image with Chromium bundled, plus Homebrew and Scoop.
