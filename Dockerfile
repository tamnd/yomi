# Consumed by GoReleaser: it copies the already cross-compiled binary out of the
# build context rather than compiling, so the image build is fast and uses the
# same static binary every other artifact ships.
#
# yomi renders JavaScript with a real headless Chrome when a page needs it, so
# unlike a plain CLI image this one bundles Chromium. CHROME_BIN points yomi at
# the system binary so it never tries to download its own.
#
# GoReleaser builds one multi-platform image with buildx and stages each
# platform's binary under a $TARGETPLATFORM directory (e.g. linux/amd64/) in the
# build context, so the COPY line selects the right one through the automatic
# TARGETPLATFORM build arg.
FROM alpine:3.21

ARG TARGETPLATFORM

# chromium for rendering; ca-certificates for HTTPS; tzdata for sane timestamps;
# the font package so rendered pages have glyphs to lay out.
RUN apk add --no-cache chromium ca-certificates tzdata font-noto \
 && adduser -D -H -u 10001 yomi \
 && mkdir -p /out \
 && chown yomi:yomi /out

COPY $TARGETPLATFORM/yomi /usr/bin/yomi

USER yomi
WORKDIR /out

# Point yomi at the bundled Chromium and write output under /out by default:
#
#   docker run -v "$PWD/out:/out" ghcr.io/tamnd/yomi site example.com
#
# The yomi user has no home directory of its own, so HOME points at the mounted
# /out volume. That keeps Chrome's profile and crash database writable; without
# it the render path fails with a permission error in the container.
ENV CHROME_BIN=/usr/bin/chromium-browser \
    HOME=/out

VOLUME ["/out"]

ENTRYPOINT ["/usr/bin/yomi"]
