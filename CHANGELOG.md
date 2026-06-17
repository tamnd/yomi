# Changelog

All notable changes to this project are documented here.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/tamnd/yomi/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tamnd/yomi/releases/tag/v0.1.0
