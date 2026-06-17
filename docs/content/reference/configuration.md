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
| `--links` | `inline` | `inline` or `reference` link style |
| `--no-front-matter` | `false` | Omit the YAML front-matter header |
| `--title-heading` | `false` | Keep the title as an H1 at the top of the body |
| `--wrap` | `0` | Hard-wrap prose at column N (0 = no wrap) |
| `-o, --out` | varies | Output file (`read`), or folder/file (`site`); defaults to the host name for a site |
| `-q, --quiet` | `false` | Suppress progress output |

## Images

| Flag | Default | Meaning |
|------|---------|---------|
| `--images` | `remote` | `remote`, `download`, or `inline` |
| `--max-image-mb` | `16` | Skip images larger than this |

See the [images guide](/guides/images/) for where downloaded and inlined files
land.

## Crawl and scope

These apply to `yomi site`.

| Flag | Default | Meaning |
|------|---------|---------|
| `-s, --single` | `false` | One combined file instead of a folder |
| `-p, --max-pages` | `0` | Stop after N pages (0 = unlimited) |
| `-d, --max-depth` | `0` | Depth cap (0 = unlimited) |
| `--workers` | `4` | Concurrent page workers |
| `--subdomains` | `false` | Treat subdomains of the seed as in scope |
| `--scope-prefix` | | Only crawl pages whose path starts with this prefix |
| `--exclude` | | Path prefixes to skip (repeatable) |
| `--no-robots` | `false` | Ignore `robots.txt` |

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
