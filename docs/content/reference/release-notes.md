---
title: "Release notes"
description: "What changed in each yomi release."
weight: 40
---

The authoritative, commit-level history lives in [`CHANGELOG.md`](https://github.com/tamnd/yomi/blob/main/CHANGELOG.md) and on the [releases page](https://github.com/tamnd/yomi/releases). This page summarises each version.

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
