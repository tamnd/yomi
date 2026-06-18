# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

A round on input and output: more ways to feed yomi a page, and more shapes to get one back.

### Added

- `yomi pack --format epub` (or a `.epub` output name) builds an EPUB 3 book from a crawl, readable on any e-reader. Each page becomes a well-formed XHTML chapter, in-scope links are rewired to sibling chapters, every referenced image is pulled into the book so it reads with no network, a generated navigation document lists every page, and a drawn-in-code cover stands in front. The book passes EPUBCheck, the official validator, with no errors or warnings, and carries accessibility metadata. `--icon <png>` supplies your own cover, and the existing `--title`, `--language`, and `--date` flags fill the book's metadata. The crawl keeps its SQLite store as a sidecar for the next incremental run, exactly as the ZIM format does.
- `yomi read` now takes more than a URL. `yomi read -` reads HTML from standard input, and `yomi read page.html` reads a local file, so you can convert a page you already have without a fetch. `--base <url>` sets the URL that relative links resolve against.
- Structured output for `yomi read`: `-f json` and `-f jsonl` emit the full page record (metadata, links, images, and the Markdown body) instead of a Markdown document, and `-f html` emits a self-contained HTML article. The default stays Markdown.
- Structured output for `yomi site`: `--format json` and `--format jsonl` write one dataset file of every crawled page rather than a folder of Markdown, for feeding a pipeline.
- `yomi site --sitemap` seeds the crawl from the site's `sitemap.xml` (and any `Sitemap:` lines in `robots.txt`), following a sitemap index one level, so a crawl reaches pages that are listed but not linked. `pack` honours the same option.
- `yomi site --resume` continues an interrupted Markdown crawl. The crawl writes each page as it is read and records it in a `.yomi-state.jsonl` sidecar, so a re-run with `--resume` skips the pages already done, mirroring how `pack` resumes from its store.

## [0.2.1] - 2026-06-18

A pass to make a packed ZIM open nicely in Kiwix.

### Added

- A 48x48 library icon in every ZIM build, so Kiwix shows a real book tile instead of a blank placeholder. yomi draws a built-in reading icon; `--icon <png>` uses the site's own logo instead.
- ZIM `Counter` metadata recording the packed page count by MIME type, which Kiwix reads for its library listing.

### Changed

- ZIM `Creator` metadata is now the packed site rather than the tool version, and `Scraper` names yomi and its version, matching how Kiwix expects the two keys to read.

## [0.2.0] - 2026-06-18

A new way to keep a whole site: one file instead of a folder.

### Added

- `yomi pack <url>` crawls a site and bundles it into one file. The default format is a SQLite database with structured `pages`, `links`, and `images` tables; `--format zim` (or a `.zim` output name) builds a ZIM offline archive browsable in Kiwix, with each page rendered to a self-contained HTML document, in-scope links rewired to the sibling entries, and a generated contents page as the landing page.
- A resumable, incremental crawl backing `pack`. The SQLite store is the crawl's backing store, so a re-run keeps every page already stored and fetches only what is new. `--refresh` re-fetches every page; `--max-age <dur>` re-fetches only the pages older than the cutoff. A ZIM build keeps its store as a sidecar (`--state` to relocate it) for the next incremental run.
- Format inference from the output extension: `-o site.db`/`.sqlite` packs SQLite and `-o site.zim` packs ZIM without an explicit `--format`. ZIM metadata flags (`--title`, `--description`, `--language`, `--date`) and `--no-compress`. `pack` takes the same scope, limit, worker, and robots flags as `yomi site`.

## [0.1.2] - 2026-06-18

A small pass for the people actually typing the commands.

### Changed

- `yomi read`, `yomi meta`, and `yomi links` now accept a bare host. A URL without a scheme, like `example.com/post`, defaults to `https://`, the same shorthand `yomi site` already took. Every example in the docs uses it, and now they all run as written.

### Documentation

- A rewritten README with a terminal demo of the read, save, and meta loop.
- A friendlier pass over the homepage and guides, with the bare-host shorthand called out where you first meet a URL.

## [0.1.1] - 2026-06-18

### Metadata

- `yomi meta` no longer prints an empty `markdown` field, so the metadata view is just metadata. The record is the page's `url`, `title`, `byline`, `site_name`, `excerpt`, `lang`, `published`, `fetched`, `word_count`, `reading_time`, and `rendered`, followed by `links` and `images` as arrays.

### Documentation

- The front-matter and `yomi meta` examples in the docs now match the real output exactly: quoted string values, `reading_time` as a whole-minute number, and `links` and `images` as arrays of objects rather than counts.
- The CLI reference and configuration pages note that the default `--user-agent` is a real desktop Chrome string.

### Markdown quality

- A caption that only repeats the alt text of the image it follows is dropped, so an article figure no longer prints the same line twice, while a caption that adds information is kept.
- A picture linked to its own full-size file is unwrapped to a plain image, since that lightbox link does nothing in a Markdown document, while an image that links to an article or any other page keeps its link.
- A standalone share-or-subscribe button left in the body, such as a lone Share or Subscribe link on its own line, is removed.
- A bare angle bracket the converter escaped to keep the Markdown safe is restored to its literal character in prose, so an arrow like a -> b, a comparison like >=22.12.0, and a placeholder like <namespace> read as written instead of showing &gt; and &lt;. Code keeps its bytes exactly as they are.
- A backslash the converter added in front of a character that did not need escaping is dropped in prose, so an identifier like system_specs and an approximation like ~2ms lose the stray backslash, while a backslash inside code and an escape that would otherwise start emphasis or strikethrough are kept.
- A link the page left without a destination renders as its text alone, instead of an empty link with nowhere to go.

## [0.1.0] - 2026-06-17

First release. yomi reads a web page, or a whole website, into clean Markdown.

### Commands

- `yomi read` reads one page into Markdown, to stdout or a file, with a YAML front-matter header.
- `yomi site` crawls a whole site in scope and writes either a folder of Markdown mirroring the URL paths with a `SUMMARY.md`, or one combined file with a table of contents and per-page sections (`--single`).
- `yomi meta` and `yomi links` expose a page's metadata and article links as JSON.
- `yomi serve` previews a folder of Markdown over a local static server.

### Fetching and rendering

- Auto render mode fetches statically first and escalates to headless Chrome only when a page looks JavaScript-gated, so the fast path stays fast.
- Image policies leave images remote (default), download them next to the Markdown, or inline them as data URIs.

### Markdown quality

- Fenced code blocks carry a language info string.
- The extractor keeps the highlighter class that names the language, which readability would otherwise strip, and reads the language from the conventions documentation sites use: `language-`/`lang-` classes, a GitHub `highlight-source-` wrapper, a Sphinx or Pygments `highlight-` wrapper, MDN's `brush:` marker, and `data-language` attributes.
- A few highlighter lexer names fold to their common Markdown name, so `python3` becomes `python` and `js` becomes `javascript`.
- HTML tables convert to Markdown tables instead of a flattened run of cells, and strikethrough is recognised.
- Headings drop their permalink decorations, whether a trailing pilcrow or hash link, an empty link, or a heading that is only a self-anchor, while a real cross-reference link in a heading is left intact.
- Code whose highlighter laid each line out as its own element, with no literal newline between lines, regains its line breaks.
- Standalone preview-counter gutters, the column of bare numbers like 01, 02, 03 that component docs render next to an example, are dropped, while a lone number in prose and any number inside code are kept.

[Unreleased]: https://github.com/tamnd/yomi/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/tamnd/yomi/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/tamnd/yomi/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/tamnd/yomi/releases/tag/v0.1.0
