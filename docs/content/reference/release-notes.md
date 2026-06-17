---
title: "Release notes"
description: "What changed in each yomi release."
weight: 40
---

The authoritative, commit-level history lives in [`CHANGELOG.md`](https://github.com/tamnd/yomi/blob/main/CHANGELOG.md) and on the [releases page](https://github.com/tamnd/yomi/releases). This page summarises each version.

## v0.1.0

The first release. yomi reads a web page, or a whole website, into clean Markdown: fetch the page, render the JavaScript only when the page needs it, extract the main content, and convert what is left to GitHub-Flavored Markdown with a YAML front-matter block.

- **`yomi read <url>`** turns one page into Markdown, printing to stdout or writing to a file with `-o`. The default `--render auto` static-fetches first and escalates to headless Chrome only when the page looks JavaScript-gated, so most reads never launch a browser. Readability extraction drops the nav, cookie banners, footers, and share rails before conversion.
- **`yomi site <url>`** reads a whole site. The default output is a folder of `.md` files mirroring the URL paths, with a `SUMMARY.md` table of contents and a shared `media/` folder; `--single` assembles one combined file with a table of contents, per-page sections and anchors, and demoted headings for a single clean outline.
- **`yomi meta <url>`** prints a page's metadata record as JSON, and **`yomi links <url>`** lists the outbound links in the article body, with `--json` for a structured list.
- **`yomi serve [dir]`** previews a folder of Markdown over a local static file server.
- **Three image policies.** `--images remote` (the default) leaves image URLs absolute, `download` fetches them next to the output, and `inline` embeds them as base64 data URIs, with a `--max-image-mb` cap.
- **Polite by default.** Honours `robots.txt`, scopes to the seed host, and reads pages with four parallel workers.
- **Packaged everywhere.** Archives, `.deb`/`.rpm`/`.apk`, a multi-arch GHCR image with Chromium bundled, plus Homebrew and Scoop.
