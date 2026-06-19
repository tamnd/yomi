---
title: "Installation"
description: "Install yomi from Go, Homebrew, Scoop, a release archive, a Linux package, or the container image, and know when it needs a browser."
weight: 20
---

yomi is a single binary. Pick whichever channel suits you.

## Go

```bash
go install github.com/tamnd/yomi/cmd/yomi@latest
```

## Homebrew (macOS)

```bash
brew install tamnd/tap/yomi
```

The cask installs the prebuilt macOS binary. On Linux, use the packages below or
`go install`.

## Scoop (Windows)

```bash
scoop bucket add tamnd https://github.com/tamnd/scoop-bucket
scoop install yomi
```

## Linux (apt and dnf)

A signed apt and dnf repository tracks every release, so `apt upgrade` and
`dnf upgrade` keep yomi current.

```bash
# Debian, Ubuntu
curl -fsSL https://tamnd.github.io/linux-repo/gpg.key \
  | sudo gpg --dearmor -o /usr/share/keyrings/tamnd.gpg
echo "deb [signed-by=/usr/share/keyrings/tamnd.gpg] https://tamnd.github.io/linux-repo/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/tamnd.list
sudo apt update && sudo apt install yomi

# Fedora, RHEL
sudo dnf config-manager --add-repo https://tamnd.github.io/linux-repo/dnf/tamnd.repo
sudo dnf install yomi
```

## Release archives and Linux packages

Every [release](https://github.com/tamnd/yomi/releases) attaches `tar.gz`
archives (and a `.zip` for Windows) for Linux, macOS, Windows, and FreeBSD, plus
`.deb`, `.rpm`, and `.apk` packages. Download the one for your platform, extract
`yomi`, and put it on your `PATH`. To install a package directly without the repo
above:

```bash
# Debian/Ubuntu
sudo dpkg -i yomi_*_amd64.deb

# Fedora/RHEL
sudo rpm -i yomi-*.x86_64.rpm
```

## Container

The image bundles Chromium and sets `CHROME_BIN`, so the render path works out of
the box:

```bash
docker run -v "$PWD:/out" ghcr.io/tamnd/yomi read example.com -o /out/page.md
```

## When you need a browser

yomi fetches a static page with a plain HTTP request, no browser involved. It
launches headless Chrome only on the render path: when `--render auto` decides a
page is JavaScript-gated, or when you pass `--render on`. In `auto` mode many
pages never start a browser at all, so for a lot of reads you need nothing extra.

When yomi does render, it needs Chrome or Chromium on the machine. It looks for a
system install automatically (Google Chrome on macOS and Windows,
`google-chrome`/`chromium` on Linux). To point it at a specific binary:

```bash
yomi read example.com --chrome /path/to/chromium
# or
export CHROME_BIN=/path/to/chromium
```

The container image sets `CHROME_BIN` to its bundled Chromium for you. If you
only ever read static pages, you can run with `--render off` and never touch a
browser.

Next: [the quick start](/getting-started/quick-start/).
