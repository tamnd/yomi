---
title: "CLI reference"
description: "Every yomi command and flag."
weight: 10
---

```
yomi [command] [flags]
```

Six commands: `read` turns one page into Markdown, `site` reads a whole site into
a folder, `pack` bundles a whole site into one SQLite database or ZIM archive,
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

## yomi pack

```
yomi pack <url> [flags]
```

Crawls a whole site and bundles it into one file: a SQLite database of pages,
links, and images (the default), or a ZIM offline archive you can open in Kiwix.
The crawl is backed by the database, so it resumes where it left off and a later
run only fetches what is new.

An explicit `--format` wins; otherwise the output extension picks the format, so
`-o site.zim` builds a ZIM and `-o site.db` builds a database without a
`--format` flag.

### Output

| Flag | Default | Meaning |
|------|---------|---------|
| `--format` | `sqlite` | Output format: `sqlite` or `zim` |
| `-o, --out` | `<host>.db` or `.zim` | Output file |
| `--state` | the output with `.db` | SQLite store path for a ZIM build (the resumable sidecar) |

### Resume and refresh

| Flag | Default | Meaning |
|------|---------|---------|
| `--refresh` | `false` | Re-fetch every page, ignoring what is already stored |
| `--max-age` | `0` | Re-fetch a stored page older than this duration (e.g. `24h`; 0 = never) |

### ZIM metadata

| Flag | Default | Meaning |
|------|---------|---------|
| `--title` | the home page title | Archive title |
| `--description` | | Archive description |
| `--language` | `eng` | Archive language as an ISO 639-3 code |
| `--date` | today (UTC) | Archive date (`YYYY-MM-DD`) |
| `--icon` | a built-in icon | Path to a 48x48 PNG shown as the Kiwix library tile |
| `--no-compress` | `false` | Store every entry raw, with no compression |

### Scope and concurrency

`pack` takes the same scope, limit, worker, and robots flags as
[`yomi site`](#yomi-site): `-p, --max-pages`, `-d, --max-depth`, `--subdomains`,
`--scope-prefix`, `--exclude`, `--workers`, and `--no-robots`.

Plus the [shared read flags](#shared-read-flags), applied to every page in the
crawl.

## yomi meta

```
yomi meta <url> [flags]
```

Prints the page's metadata record as JSON, without the Markdown body. The record
carries `url`, `title`, `byline`, `site_name`, `excerpt`, `lang`, `published`,
`fetched`, `word_count`, `reading_time` (whole minutes), and `rendered`, followed
by `links` and `images` as arrays of objects (each link is `{text, url,
internal}`, each image `{alt, url, local}`). Only non-empty fields appear. Takes
the [shared read flags](#shared-read-flags).

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
| `--user-agent` | a desktop Chrome string | User-Agent for page and asset fetches; the default is a real browser string so a page returns the HTML a person would see |
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
| `reading_time` | Estimated reading time, in whole minutes |

String values are quoted; `word_count` and `reading_time` are written as plain
numbers.
