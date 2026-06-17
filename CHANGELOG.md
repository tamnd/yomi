# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to
follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `yomi read` reads one page into Markdown, to stdout or a file, with a YAML
  front-matter header.
- `yomi site` crawls a whole site in scope and writes either a folder of Markdown
  mirroring the URL paths with a `SUMMARY.md`, or one combined file with a table
  of contents and per-page sections (`--single`).
- `yomi meta` and `yomi links` expose a page's metadata and article links as JSON.
- `yomi serve` previews a folder of Markdown over a local static server.
- Auto render mode: static fetch first, escalating to headless Chrome only when a
  page looks JavaScript-gated.
- Image policies: leave remote (default), download next to the Markdown, or inline
  as data URIs.

### Changed

- Fenced code blocks now carry a language info string. Readability used to strip
  the highlighter class that names the language, so every fence came out bare. The
  extractor now keeps that class and reads the language from the conventions doc
  sites use: `language-`/`lang-` classes, a GitHub `highlight-source-` wrapper, a
  Sphinx/Pygments `highlight-` wrapper, MDN's `brush:` marker, and `data-language`
  attributes, with a few lexer aliases folded to their common name.
- HTML tables now convert to Markdown tables instead of a flattened run of cells,
  and strikethrough is recognised, by enabling the GitHub-Flavored table and
  strikethrough converters.
- Headings drop their permalink decorations (a trailing pilcrow or hash link, an
  empty link, or a heading that is only a self-anchor), while real cross-reference
  links in a heading are left intact.
- Code whose highlighter laid each line out as its own element, with no literal
  newline between lines, regains its line breaks.
- Standalone preview-counter gutters (a column of bare numbers like 01, 02, 03 that component docs render next to an example) are dropped, while a lone number in prose and any number inside code are kept.
