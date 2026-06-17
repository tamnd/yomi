---
title: "Images"
description: "The three image policies in yomi: leave them remote, download them next to the output, or inline them as data URIs. Where files land, and the size cap."
weight: 40
---

A page's images can be handled three ways, set with `--images`. The default
leaves them on the live web; the other two make the Markdown more
self-contained, at the cost of size or extra files.

## remote (default)

```bash
yomi read example.com -o page.md --images remote
```

Image URLs are left absolute, pointing at wherever the page references them on the
live web. The Markdown file carries no image bytes, so it stays small, but it
needs a network connection to display the images. This is the default.

## download

```bash
yomi read example.com -o page.md --images download
```

Each image is fetched and saved next to the output, and the Markdown is rewritten
to a relative path. Where the files land depends on the command:

- For a **single read** (`yomi read`), images go into a `<name>.media/` sidecar
  folder next to the output file. Reading to `page.md` puts images in
  `page.media/`.
- For a **site crawl** (`yomi site`, folder output), images go into one shared
  `media/` folder at the root of the output, so an image used on several pages is
  stored once.

This gives you a folder that displays offline, with the images traveling
alongside the Markdown.

## inline

```bash
yomi read example.com -o page.md --images inline
```

Each image is embedded directly in the Markdown as a base64 data URI. The result
is a single self-contained file with no sidecar folder and no network needed to
see the images. The trade is size: base64 inflates each image by about a third,
so an image-heavy page produces a large file. Reach for `inline` when you want one
file and nothing else.

## The size cap

Downloading and inlining both fetch image bytes, so yomi caps how large an image
it will pull. The default cap is 16 MB, set with `--max-image-mb`:

```bash
yomi read example.com -o page.md --images download --max-image-mb 8
```

An image over the cap is left at its remote URL rather than downloaded or inlined,
so an oversized asset never bloats the output or stalls the read.
