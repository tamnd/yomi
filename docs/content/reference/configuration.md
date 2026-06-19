---
title: "Configuration"
description: "The yomi flags grouped by concern, their defaults, and the CHROME_BIN environment variable the render path honors."
weight: 20
---

yomi is configured through command-line flags (see the
[CLI reference](/reference/cli/)). This page groups them by the job they do and
covers the one environment variable yomi reads.

## Fetching and render

How yomi gets the page, and when it launches a browser.

| Flag | Default | Meaning |
|------|---------|---------|
| `--render` | `auto` | `auto` = static fetch first, render only when the page looks JavaScript-gated; `on` = always render; `off` = never launch a browser |
| `--scroll` | `false` | Auto-scroll in render mode for lazy-loaded content |
| `--timeout` | `30s` | Per-request timeout |
| `--user-agent` | a desktop Chrome string | User-Agent for fetches; the default is a real browser string |
| `--chrome` | | Path to the Chrome/Chromium binary |
| `--control-url` | | Attach to a running Chrome DevTools endpoint |

In `auto` mode yomi decides a page is JavaScript-gated when the static HTML has an
empty single-page-app mount (`#root`, `#__next`, `#app`), a `<noscript>` saying
JavaScript is required, or under 25 words of visible text. Only then does it
escalate to the browser.

## Extraction and output

What the Markdown looks like.

| Flag | Default | Meaning |
|------|---------|---------|
| `-f, --format` | `md` | Output shape. `read`: `md`, `json`, `jsonl`, or `html`. `site`: `md`, `json`, or `jsonl` |
| `--base` | | `read` only: base URL that relative links resolve against, for file or stdin input |
| `--links` | `inline` | `inline` or `reference` link style |
| `--no-front-matter` | `false` | Omit the YAML front-matter header |
| `--title-heading` | `false` | Keep the title as an H1 at the top of the body |
| `--wrap` | `0` | Hard-wrap prose at column N (0 = no wrap) |
| `-o, --out` | varies | Output file (`read`), or folder/file (`site`); defaults to the host name for a site |
| `-q, --quiet` | `false` | Suppress progress output |

`yomi read` accepts a URL, a local `.html` file, or `-` for HTML on standard
input, so you can convert a page you already have without a fetch.

## Images

| Flag | Default | Meaning |
|------|---------|---------|
| `--images` | `remote` | `remote`, `download`, or `inline` |
| `--max-image-mb` | `16` | Skip images larger than this |

See the [images guide](/guides/images/) for where downloaded and inlined files
land.

## Crawl and scope

The scope and limit flags apply to both `yomi site` and `yomi pack`; `--single`
is specific to `yomi site`.

| Flag | Default | Meaning |
|------|---------|---------|
| `-s, --single` | `false` | One combined file instead of a folder (`site` only) |
| `--resume` | `false` | Continue an interrupted Markdown crawl from its sidecar (`site` only; `pack` resumes from its store) |
| `--sitemap` | `false` | Seed the crawl from the site's sitemap (`site` and `pack`) |
| `-p, --max-pages` | `0` | Stop after N pages (0 = unlimited) |
| `-d, --max-depth` | `0` | Depth cap (0 = unlimited) |
| `--workers` | `4` | Concurrent page workers |
| `--subdomains` | `false` | Treat subdomains of the seed as in scope |
| `--scope-prefix` | | Only crawl pages whose path starts with this prefix |
| `--exclude` | | Path prefixes to skip (repeatable) |
| `--no-robots` | `false` | Ignore `robots.txt` |

## Pack

These apply to `yomi pack`, which bundles a crawl into one SQLite database, ZIM
archive, or EPUB book. See the [packing a site](/guides/packing-a-site/) guide for
the full walkthrough.

| Flag | Default | Meaning |
|------|---------|---------|
| `--format` | `sqlite` | Output format: `sqlite`, `zim`, or `epub` (also inferred from a `.db`/`.zim`/`.epub` output name) |
| `-o, --out` | `<host>.db`/`.zim`/`.epub` | Output file |
| `--state` | the output with `.db` | SQLite store path for a ZIM or EPUB build (the resumable sidecar) |
| `--refresh` | `false` | Re-fetch every page, ignoring what is stored |
| `--max-age` | `0` | Re-fetch a stored page older than this duration (0 = never) |
| `--title` | home page title | ZIM/EPUB title |
| `--language` | `eng` | Language (ISO 639-3; mapped to a BCP 47 tag for EPUB) |
| `--date` | today (UTC) | Date (`YYYY-MM-DD`) |
| `--icon` | a built-in image | ZIM library tile (48x48) or EPUB cover, as a PNG |
| `--description` | | ZIM only: archive description |
| `--no-compress` | `false` | ZIM only: store every entry raw, with no compression |

A ZIM build writes the metadata Kiwix reads to present the archive: a `Title`,
`Description`, `Language`, `Date`, `Name`, `Creator` (the site), `Publisher` and
`Scraper` (yomi), a `Counter` of the packed pages, and an `Illustration_48x48@1`
library icon. An EPUB build writes the Dublin Core metadata a reader shows: the
title, language, the site as creator, the seed as source, the date, and the
cover. Pass `--icon` to replace the built-in icon or cover with the site's own
artwork.

## Environment variables

yomi reads one environment variable, for locating the browser on the render path.

| Variable | Meaning |
|----------|---------|
| `CHROME_BIN` | Path to the Chrome/Chromium binary. Equivalent to `--chrome`. |

The render path needs a Chrome or Chromium binary on the host. yomi looks for a
system install automatically (Google Chrome on macOS and Windows,
`google-chrome`/`chromium` on Linux); `CHROME_BIN` or `--chrome` points it at a
specific one. The Docker image
([`ghcr.io/tamnd/yomi`](https://github.com/tamnd/yomi/pkgs/container/yomi))
bundles Chromium and sets `CHROME_BIN` for you. In `--render auto` many pages are
served as real HTML and never touch the browser, so for a lot of reads no browser
is needed at all; `--render off` guarantees it.
