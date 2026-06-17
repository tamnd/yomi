---
title: "Introduction"
description: "Why yomi renders only when it has to, and what it means to read a page into Markdown instead of saving it."
weight: 10
---

A web article is wrapped in a lot of things that are not the article: a nav bar, a cookie banner, a newsletter prompt, a footer full of links, a share rail down the side. On top of that, a growing number of pages send a near-empty shell over the wire and build the real content in your browser with JavaScript, so a plain fetch gets you nothing to read. Copying any of this into a clean Markdown file by hand is tedious and easy to get wrong.

yomi reads the page for you. Say you want to keep Paul Graham's essays as Markdown:

```bash
yomi read paulgraham.com/greatwork.html -o greatwork.md
```

You get the essay, its title, byline, and dates in a front-matter block, and nothing else. yomi treats a read as four steps in order.

## 1. Fetch

yomi starts with a plain HTTP request, the cheapest way to get a page. Most pages are served as real HTML, and for those this is all that is needed.

## 2. Render, but only when needed

The default render mode is `auto`. After the static fetch, yomi looks at what came back. If the page looks JavaScript-gated, an empty single-page-app mount like `#root`, `#__next`, or `#app`, a `<noscript>` block saying JavaScript is required, or under 25 words of visible text, it escalates to headless Chrome through the DevTools protocol, loads the page for real, optionally scrolls for lazy content, and serialises the final DOM. A page that already arrived as readable HTML never launches a browser. You can force the choice with `--render on` or `--render off`.

## 3. Extract

From the page yomi runs readability extraction: it finds the main content and discards the rest. Out go the nav, the cookie banner, the footer, the share rail, the related-posts grid. What remains is the article body, plus the metadata yomi reads from the page (title, byline, site name, published date, language).

## 4. Convert

The extracted content is converted to GitHub-Flavored Markdown: headings, lists, tables, code blocks, links, and images. yomi adds a YAML front-matter header with the fields it found, in a fixed order, and writes only the ones that are non-empty:

```yaml
---
title: "How to Do Great Work"
url: "https://paulgraham.com/greatwork.html"
byline: "Paul Graham"
fetched: "2026-06-17T09:30:00Z"
word_count: 11856
reading_time: 59
---
```

The string values are quoted, `reading_time` is whole minutes, and a page that also exposes a site name, language, or published date gets `site`, `lang`, and `published` lines too. Empty fields are left out.

## How it differs from kage, and from "copy into Markdown"

[kage](https://github.com/tamnd/kage) is yomi's sibling. kage clones a site into a browsable offline HTML copy and keeps its shape, the layout, the CSS, the look of the live site frozen and inert. yomi throws the shape away on purpose: it keeps the reading, as Markdown. They share the same headless-browser engine, scope model, and robots handling, but they answer different questions. kage is "let me browse this offline"; yomi is "let me read this in my editor".

It also differs from pasting a page into a Markdown converter. A converter takes whatever HTML you give it, furniture and all, and it cannot run the page's JavaScript, so a single-page app converts to an empty document. yomi renders when it has to and extracts before it converts, so the output is the article and not the page around it.

## Reading a whole site

A single page is the small case. `yomi site` crawls breadth-first from a seed URL, staying within the seed's host (and optionally its subdomains), honouring `robots.txt` the same way kage does. By default it writes a folder of `.md` files mirroring the URL paths, with a `SUMMARY.md` index and a shared `media/` folder. With `--single` it assembles the whole crawl into one Markdown file instead, with a table of contents and per-page sections. The [single vs folder](/guides/single-vs-folder/) guide covers when to use each.

Next: [install yomi](/getting-started/installation/).
