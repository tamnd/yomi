---
title: "CLI reference"
description: "Every yomi command and flag."
weight: 10
---

```
yomi [command] [flags]
```

Five commands: `read` turns one page into Markdown, `site` reads a whole site,
`meta` prints a page's metadata as JSON, `links` lists a page's outbound links,
and `serve` previews a folder of Markdown. Run `yomi <command> --help` for the
canonical, up-to-date list.

The read flags in the last section are shared: they apply to `read`, `site`,
`meta`, and `links`, since each fetches and extracts a page before doing its job.

## yomi read

```
yomi read <url> [flags]
```

Reads one page into Markdown, printing to stdout or writing to a file. Fetches the
page, renders it in headless Chrome only when needed, extracts the main content,
and converts it to GitHub-Flavored Markdown with a YAML front-matter header.

| Flag | Default | Meaning |
|------|---------|---------|
| `-o, --out` | stdout | Write the Markdown to a file instead of stdout |

Plus the [shared read flags](#shared-read-flags).

## yomi site

```
yomi site <url> [flags]
```

Crawls a whole site breadth-first from the seed URL, reading each in-scope page
into Markdown. The default output is a folder of `.md` files mirroring the URL
paths, with a `SUMMARY.md` table of contents and a shared `media/` folder.
`--single` assembles one combined file instead.

### Output

| Flag | Default | Meaning |
|------|---------|---------|
| `-o, --out` | host name | Output folder, or file path with `--single` |
| `-s, --single` | `false` | Assemble one combined `.md` file with a table of contents and per-page sections |

### Scope

| Flag | Default | Meaning |
|------|---------|---------|
| `-p, --max-pages` | `0` | Stop after N pages (0 = unlimited) |
| `-d, --max-depth` | `0` | Link-follow depth cap (0 = unlimited) |
| `--subdomains` | `false` | Treat subdomains of the seed host as in scope |
| `--scope-prefix` | | Only crawl pages whose path starts with this prefix |
| `--exclude` | | Path prefixes to skip (repeatable) |
| `--no-robots` | `false` | Ignore `robots.txt` |

### Concurrency

| Flag | Default | Meaning |
|------|---------|---------|
| `--workers` | `4` | Concurrent page workers |

Plus the [shared read flags](#shared-read-flags), applied to every page in the
crawl.

## yomi meta

```
yomi meta <url> [flags]
```

Prints the page's metadata record as JSON (title, byline, site, language,
word_count, reading_time, links, images), without the Markdown body. Takes the
[shared read flags](#shared-read-flags).

## yomi links

```
yomi links <url> [flags]
```

Lists the outbound links found in the page's article body, one URL per line.

| Flag | Default | Meaning |
|------|---------|---------|
| `--json` | `false` | Emit the links as JSON instead of one URL per line |

Plus the [shared read flags](#shared-read-flags).

## yomi serve

```
yomi serve [dir] [flags]
```

Runs a local static file server over a folder of Markdown. With no `dir`, serves
the current directory.

| Flag | Default | Meaning |
|------|---------|---------|
| `-a, --addr` | `127.0.0.1:8800` | Address to listen on |

## Shared read flags

These apply to `read`, `site`, `meta`, and `links`.

### Fetching and rendering

| Flag | Default | Meaning |
|------|---------|---------|
| `--render` | `auto` | `auto` static-fetches first and renders in headless Chrome only when the page looks JavaScript-gated; `on` always renders; `off` never launches a browser |
| `--scroll` | `false` | Auto-scroll the page in render mode to trigger lazy loading |
| `--timeout` | `30s` | Per-request timeout |
| `--user-agent` | | User-Agent for fetches |
| `--chrome` | | Path to the Chrome/Chromium binary |
| `--control-url` | | Attach to a running Chrome DevTools endpoint |

### Extraction and output

| Flag | Default | Meaning |
|------|---------|---------|
| `--links` | `inline` | Link style: `inline` (`[text](url)`) or `reference` (definitions at the bottom) |
| `--no-front-matter` | `false` | Omit the YAML front-matter header |
| `--title-heading` | `false` | Keep the title as an H1 at the top of the body |
| `--wrap` | `0` | Hard-wrap prose at column N (0 = no wrap) |
| `-q, --quiet` | `false` | Suppress progress output |

### Images

| Flag | Default | Meaning |
|------|---------|---------|
| `--images` | `remote` | `remote` leaves image URLs absolute; `download` fetches images next to the output and rewrites to relative paths; `inline` embeds images as base64 data URIs |
| `--max-image-mb` | `16` | Skip images larger than this; leave them at their remote URL |

## Front-matter fields

`read` and `site` write a YAML front-matter header on each Markdown file. The
fields appear in this fixed order, and only non-empty ones are written:

| Field | Meaning |
|-------|---------|
| `title` | Page title |
| `url` | Source URL |
| `site` | Site name |
| `byline` | Author |
| `published` | Published date |
| `fetched` | When yomi read the page |
| `lang` | Content language |
| `word_count` | Words in the extracted article |
| `reading_time` | Estimated reading time |
