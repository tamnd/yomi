package yomi

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// defaultIllustration returns a 48x48 PNG used as the archive's Kiwix library
// icon when the caller supplies none. Kiwix reads it from the
// Illustration_48x48@1 metadata entry and shows it as the book tile; without it
// the library falls back to a generic placeholder. The icon is a folded page on
// an indigo rounded square, a plain reading motif, drawn here so the binary
// carries no image asset. It is rendered at 4x and box-downscaled so the edges
// and the rounded corners are smooth.
func defaultIllustration() []byte {
	const (
		size  = 48
		scale = 4
	)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for oy := 0; oy < size; oy++ {
		for ox := 0; ox < size; ox++ {
			var r, g, b, a float64
			for sj := 0; sj < scale; sj++ {
				for si := 0; si < scale; si++ {
					fx := float64(ox) + (float64(si)+0.5)/scale
					fy := float64(oy) + (float64(sj)+0.5)/scale
					cr, cg, cb, ca := iconColorAt(fx, fy)
					r, g, b, a = r+cr, g+cg, b+cb, a+ca
				}
			}
			n := float64(scale * scale)
			img.SetNRGBA(ox, oy, color.NRGBA{
				R: uint8(r/n + 0.5),
				G: uint8(g/n + 0.5),
				B: uint8(b/n + 0.5),
				A: uint8(a/n + 0.5),
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// coverPNG draws an EPUB cover: a portrait canvas filled in the reading-motif
// indigo with the folded-page mark centred in the upper half, the same drawn-in-
// code choice the ZIM library icon makes so the binary carries no image asset.
// The mark reuses the 48-unit page geometry, scaled onto the cover and sampled
// at 3x for smooth edges. It returns nil only if PNG encoding fails.
func coverPNG(w, h int) []byte {
	const (
		bgR, bgG, bgB = 37, 99, 235 // #2563eb
		supersample   = 3
	)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))

	// The motif box: a square in the upper-middle of the cover that maps onto the
	// 48-unit icon space the page geometry is drawn in.
	motif := float64(w) * 0.62
	originX := (float64(w) - motif) / 2
	originY := float64(h) * 0.20

	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			var r, g, b float64
			for sj := 0; sj < supersample; sj++ {
				for si := 0; si < supersample; si++ {
					fx := float64(px) + (float64(si)+0.5)/supersample
					fy := float64(py) + (float64(sj)+0.5)/supersample
					ix := (fx - originX) / motif * 48
					iy := (fy - originY) / motif * 48
					cr, cg, cb := coverSample(ix, iy, bgR, bgG, bgB)
					r, g, b = r+cr, g+cg, b+cb
				}
			}
			n := float64(supersample * supersample)
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(r/n + 0.5),
				G: uint8(g/n + 0.5),
				B: uint8(b/n + 0.5),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// coverSample returns the colour of the page motif at a point in 48-unit icon
// space, falling back to the supplied background colour off the page.
func coverSample(x, y float64, bgR, bgG, bgB int) (r, g, b float64) {
	if inPage(x, y) {
		switch {
		case onFold(x, y):
			return 203, 213, 225 // #cbd5e1, the turned corner
		case onTextLine(x, y):
			return 148, 163, 184 // #94a3b8, lines of text
		default:
			return 255, 255, 255 // the page
		}
	}
	return float64(bgR), float64(bgG), float64(bgB)
}

// iconColorAt returns the colour, as 0-255 channels, at a point in the 48-unit
// icon space. Points outside the rounded background are transparent.
func iconColorAt(x, y float64) (r, g, b, a float64) {
	const sz = 48
	if !insideRounded(x, y, sz, 9) {
		return 0, 0, 0, 0
	}
	if inPage(x, y) {
		if onFold(x, y) {
			return 203, 213, 225, 255 // #cbd5e1, the turned corner
		}
		if onTextLine(x, y) {
			return 148, 163, 184, 255 // #94a3b8, lines of text
		}
		return 255, 255, 255, 255 // the page
	}
	return 37, 99, 235, 255 // #2563eb, the background
}

// insideRounded reports whether (x,y) is within a sz-by-sz square with corners
// rounded to the given radius.
func insideRounded(x, y, sz, radius float64) bool {
	if x < 0 || y < 0 || x > sz || y > sz {
		return false
	}
	cx, cy := x, y
	if x < radius {
		cx = radius
	} else if x > sz-radius {
		cx = sz - radius
	}
	if y < radius {
		cy = radius
	} else if y > sz-radius {
		cy = sz - radius
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= radius*radius
}

// The page rectangle and its folded top-right corner, in icon space.
const (
	pageLeft   = 13.0
	pageRight  = 35.0
	pageTop    = 9.0
	pageBottom = 40.0
	foldSize   = 8.0
)

// inPage reports whether (x,y) is on the page, the folded corner notch removed.
func inPage(x, y float64) bool {
	if x < pageLeft || x > pageRight || y < pageTop || y > pageBottom {
		return false
	}
	if x >= pageRight-foldSize && y <= pageTop+foldSize {
		dx := x - (pageRight - foldSize)
		dy := y - pageTop
		if dx > dy { // the cut-away triangle at the top-right
			return false
		}
	}
	return true
}

// onFold reports whether (x,y) is on the small flap of the turned corner.
func onFold(x, y float64) bool {
	if x < pageRight-foldSize || y > pageTop+foldSize {
		return false
	}
	dx := x - (pageRight - foldSize)
	dy := y - pageTop
	return dx <= dy
}

// onTextLine reports whether (x,y) falls on one of the drawn lines of text.
func onTextLine(x, y float64) bool {
	lines := []struct{ cy, x0, x1 float64 }{
		{16, 16, 27}, // shorter, clears the fold
		{20.5, 16, 31},
		{25, 16, 31},
		{29.5, 16, 31},
		{34, 16, 28},
	}
	for _, ln := range lines {
		if x >= ln.x0 && x <= ln.x1 && y >= ln.cy-1 && y <= ln.cy+1 {
			return true
		}
	}
	return false
}
