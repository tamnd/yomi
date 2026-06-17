# yomi

yomi (読み, "reading") reads a web page, or a whole website, into clean Markdown.
It fetches a page, renders the JavaScript when the page needs it, strips the
navigation and the clutter, and converts the main content to Markdown with a
small front-matter block. Point it at one URL for one document, or crawl a whole
site into a folder of Markdown or a single combined file.

```bash
# one page to stdout
yomi read https://example.com

# one page to a file
yomi read https://example.com -o example.md

# a whole site into a folder of Markdown
yomi site https://example.com -o example-docs

# a whole site collapsed into one file
yomi site https://example.com --single -o example.md
```

## Why

Copy-pasting an article into Markdown drags in the menu, the cookie banner, the
newsletter box, and the share rail, and a "Save As" copy of a JavaScript-built
page is often blank. yomi takes the reader's view: it renders the page the way a
person would see it, keeps the part you came to read, and hands you Markdown you
can store, diff, search, or feed to something else.

It is a sibling to [kage](https://github.com/tamnd/kage), which mirrors a site as
a browsable offline copy. kage keeps the *shape* of a site; yomi keeps the
*reading*. They share the same headless-browser engine, the same scope model,
and the same robots handling, so a yomi crawl and a kage clone agree on what is
in scope.

## What it does

- **Renders when needed.** Static fetch first; yomi escalates to headless Chrome
  only when the page looks JavaScript-gated, so most reads never launch a browser.
- **Extracts the article.** Readability isolates the main content and the
  navigation, footers, and share widgets are dropped before conversion.
- **Converts to Markdown.** Clean GitHub-Flavored Markdown with a YAML
  front-matter header carrying the title, byline, site, language, and reading time.
- **Reads a whole site.** Crawl in scope and write a folder of `.md` files
  mirroring the URL paths with a `SUMMARY.md`, or one combined file with a table
  of contents and per-page sections. Internal links are rewired to local files or
  in-file anchors.
- **Handles images.** Leave them as remote URLs (default), download them next to
  the Markdown, or inline them as data URIs for a self-contained file.

## Install

```bash
# Go
go install github.com/tamnd/yomi/cmd/yomi@latest

# Homebrew
brew install tamnd/tap/yomi

# Scoop (Windows)
scoop bucket add tamnd https://github.com/tamnd/scoop-bucket
scoop install yomi

# Docker (bundles Chromium for the render path)
docker run --rm -v "$PWD/out:/out" ghcr.io/tamnd/yomi read https://example.com
```

Prebuilt archives and Linux packages (deb, rpm, apk) are attached to each
[release](https://github.com/tamnd/yomi/releases). The render path needs a
Chrome or Chromium binary on the host; the container image bundles one.

## Commands

| Command | What it does |
| --- | --- |
| `yomi read <url>` | Read one page to stdout, or to a file with `-o`. |
| `yomi site <url>` | Crawl a site into a folder, or one file with `--single`. |
| `yomi meta <url>` | Print a page's metadata as JSON, without the body. |
| `yomi links <url>` | List the links in a page's article body. |
| `yomi serve [dir]` | Preview a folder of Markdown in your browser. |

Run `yomi <command> --help` for the full flag list.

## Documentation

Full docs are at [yomi.tamnd.com](https://yomi.tamnd.com).

## License

MIT. See [LICENSE](LICENSE).
