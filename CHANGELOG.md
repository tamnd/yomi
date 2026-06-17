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
