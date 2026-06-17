---
title: "Reading a page"
description: "Turn one URL into clean Markdown with yomi read: render modes, front-matter, the title heading, images, and the meta and links subcommands."
weight: 10
---

`yomi read` is the core command: one URL in, clean Markdown out. By default it
prints to stdout, so it pipes and redirects like any Unix tool.

```bash
yomi read paulgraham.com/greatwork.html
yomi read paulgraham.com/greatwork.html -o greatwork.md
```

`-o/--out` writes to a file instead of stdout.

## Render modes

The default mode is `--render auto`. yomi fetches the page with a plain HTTP
request first and only escalates to headless Chrome when the page looks
JavaScript-gated: an empty single-page-app mount like `#root`, `#__next`, or
`#app`, a `<noscript>` block saying JavaScript is required, or under 25 words of
visible text. A page that already arrived as readable HTML is never sent to the
browser.

```bash
# Default: static fetch, render only when the page needs it
yomi read example.com

# Always render in headless Chrome
yomi read example.com --render on

# Never launch a browser; read whatever the static HTML gives
yomi read example.com --render off
```

Use `--render on` for a site you already know is a single-page app, and `--render
off` when you want to stay fast and you know the page is static, or when there is
no browser available.

If a page lazy-loads content as you scroll, add `--scroll` so the render path
scrolls the page before snapshotting:

```bash
yomi read example.com --render on --scroll
```

## Front-matter

Every Markdown file opens with a YAML front-matter block carrying the metadata
yomi read from the page. The fields appear in a fixed order, and only non-empty
ones are written:

```yaml
---
title: "How to Do Great Work"
url: "https://paulgraham.com/greatwork.html"
site: "Paul Graham"
byline: "Paul Graham"
published: "2023-07-01"
fetched: "2026-06-17T09:30:00Z"
lang: "en"
word_count: 11856
reading_time: 59
---
```

String values are quoted, and `reading_time` is the estimate in whole minutes.
A page that does not expose a site name, byline, language, or published date
simply omits those lines.

To omit the header entirely and emit only the body:

```bash
yomi read example.com --no-front-matter
```

## The title as a heading

By default the title lives in the front-matter, not in the body. To keep it as an
H1 at the top of the Markdown body as well:

```bash
yomi read example.com --title-heading
```

## Wrapping prose

By default prose is left unwrapped, one paragraph per line. To hard-wrap at a
column:

```bash
yomi read example.com --wrap 80
```

`--wrap 0` (the default) means no wrapping.

## Links

Links are written inline by default (`[text](url)`). To collect them as reference
definitions at the bottom of the document instead:

```bash
yomi read example.com --links reference
```

## Images

By default image URLs are left absolute, pointing at the live web (`--images
remote`). To download each image next to the output and rewrite to a relative
path, or to embed images as base64 data URIs for a self-contained file:

```bash
# Download images into a sidecar folder
yomi read example.com -o page.md --images download

# Embed images inline as data URIs
yomi read example.com -o page.md --images inline
```

For a single read, `--images download` writes images into a `<name>.media/`
sidecar folder next to the output file. The [images guide](/guides/images/)
covers all three policies and the size cap.

## Just the metadata

`yomi meta` prints the page's metadata record as JSON and skips the Markdown body
entirely:

```bash
yomi meta paulgraham.com/greatwork.html
```

```json
{
  "url": "https://paulgraham.com/greatwork.html",
  "title": "How to Do Great Work",
  "excerpt": "If you collected lists of techniques for doing great work in a lot of different fields, what would the intersection look like? I decided to find out by making it.",
  "fetched": "2026-06-17T09:30:00Z",
  "word_count": 11856,
  "reading_time": 59,
  "rendered": false,
  "links": [
    { "url": "https://paulgraham.com/index.html" }
  ],
  "images": [
    { "alt": "How to Do Great Work", "url": "https://s.turbifycdn.com/.../how-to-do-great-work-2.gif" }
  ]
}
```

The record is the page's full metadata: `url`, `title`, `byline`, `site_name`,
`excerpt`, `lang`, `published`, `fetched`, `word_count`, `reading_time` (whole
minutes), and `rendered` (whether the page needed the browser), followed by the
`links` and `images` it found as arrays of objects. Only non-empty fields appear,
so a page without a byline or site name omits them, as this essay does.

This is handy for scripting: feed a list of URLs through `yomi meta`, then pull a
field with `jq`, and you get a structured row per page without converting any
prose. For example, `yomi meta <url> | jq .reading_time`.

## Just the links

`yomi links` lists the outbound links found in the page's article body, one URL
per line:

```bash
yomi links paulgraham.com/greatwork.html
```

Add `--json` for a structured list. Because the links come from the extracted
article body and not the whole page, you get the links the author wrote, not the
nav and footer links around them.

All the shared read flags (`--render`, `--scroll`, `--timeout`, `--user-agent`,
`--chrome`, and the rest) apply to `meta` and `links` too, since both have to
fetch and extract the page before they can report on it.
