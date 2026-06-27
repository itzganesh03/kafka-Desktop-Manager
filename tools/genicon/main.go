// Command genicon renders the application logo (a modern "one-way" forward
// arrow on a teal gradient rounded square) and writes:
//   - internal/ui/icon.png  (256x256, for the window / splash / taskbar)
//   - icon.ico              (256x256 PNG-in-ICO, for the .exe file icon)
//
// Pure standard library: it supersamples at 4x and box-downscales for smooth
// edges, so it needs no third-party imaging dependencies.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
)

const (
	out    = 256
	ss     = 4 // supersample factor
	big    = out * ss
	margin = 18 * ss
	radius = 52 * ss
)

func main() {
	src := image.NewNRGBA(image.Rect(0, 0, big, big))

	// Gradient palette (top -> bottom).
	top := color.NRGBA{R: 0x33, G: 0xc9, B: 0xb6, A: 0xff}
	bot := color.NRGBA{R: 0x18, G: 0x77, B: 0x8c, A: 0xff}

	rect := image.Rect(margin, margin, big-margin, big-margin)

	// Arrow (one-way / forward) geometry.
	bf := float64(big)
	ax1, ay1 := bf*0.40, bf*0.30
	ax2, ay2 := bf*0.40, bf*0.70
	ax3, ay3 := bf*0.72, bf*0.50
	// A small "stem" bar to the left for a cleaner forward-symbol look.
	stem := image.Rect(int(bf*0.30), int(bf*0.42), int(bf*0.40), int(bf*0.58))

	for y := 0; y < big; y++ {
		t := float64(y) / float64(big)
		bg := lerp(top, bot, t)
		for x := 0; x < big; x++ {
			if !inRoundRect(x, y, rect, radius) {
				continue
			}
			c := bg
			if pointInTriangle(float64(x), float64(y), ax1, ay1, ax2, ay2, ax3, ay3) ||
				(x >= stem.Min.X && x < stem.Max.X && y >= stem.Min.Y && y < stem.Max.Y) {
				c = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
			}
			src.SetNRGBA(x, y, c)
		}
	}

	dst := downscale(src, out)

	// Write PNG for the UI.
	if err := os.MkdirAll("internal/ui", 0o755); err != nil {
		panic(err)
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, dst); err != nil {
		panic(err)
	}
	if err := os.WriteFile("internal/ui/icon.png", pngBuf.Bytes(), 0o644); err != nil {
		panic(err)
	}

	// Write ICO (PNG embedded — supported by Windows Vista+).
	if err := os.WriteFile("icon.ico", makeICO(pngBuf.Bytes()), 0o644); err != nil {
		panic(err)
	}
}

func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 0xff,
	}
}

func inRoundRect(x, y int, r image.Rectangle, rad int) bool {
	if x < r.Min.X || x >= r.Max.X || y < r.Min.Y || y >= r.Max.Y {
		return false
	}
	// corner centers
	corners := [4][2]int{
		{r.Min.X + rad, r.Min.Y + rad},
		{r.Max.X - rad, r.Min.Y + rad},
		{r.Min.X + rad, r.Max.Y - rad},
		{r.Max.X - rad, r.Max.Y - rad},
	}
	inCornerBox := func(cx, cy int) bool { return abs(x-cx) <= rad && abs(y-cy) <= rad }
	for i, c := range corners {
		// only test the matching corner quadrant
		isLeft := i%2 == 0
		isTop := i < 2
		if (isLeft && x < c[0]) || (!isLeft && x >= c[0]) {
			if (isTop && y < c[1]) || (!isTop && y >= c[1]) {
				if inCornerBox(c[0], c[1]) {
					dx, dy := x-c[0], y-c[1]
					return dx*dx+dy*dy <= rad*rad
				}
				return false
			}
		}
	}
	return true
}

func pointInTriangle(px, py, x1, y1, x2, y2, x3, y3 float64) bool {
	d1 := sign(px, py, x1, y1, x2, y2)
	d2 := sign(px, py, x2, y2, x3, y3)
	d3 := sign(px, py, x3, y3, x1, y1)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func sign(px, py, x1, y1, x2, y2 float64) float64 {
	return (px-x2)*(y1-y2) - (x1-x2)*(py-y2)
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// downscale box-averages src down to size x size.
func downscale(src *image.NRGBA, size int) *image.NRGBA {
	factor := src.Bounds().Dx() / size
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a int
			for dy := 0; dy < factor; dy++ {
				for dx := 0; dx < factor; dx++ {
					c := src.NRGBAAt(x*factor+dx, y*factor+dy)
					r += int(c.R)
					g += int(c.G)
					b += int(c.B)
					a += int(c.A)
				}
			}
			n := factor * factor
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: uint8(a / n),
			})
		}
	}
	return dst
}

// makeICO wraps a PNG in a single-image ICO container.
func makeICO(pngData []byte) []byte {
	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // count
	// ICONDIRENTRY
	buf.WriteByte(0)                                              // width 0 => 256
	buf.WriteByte(0)                                              // height 0 => 256
	buf.WriteByte(0)                                              // color count
	buf.WriteByte(0)                                              // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))            // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))           // bpp
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData))) // size
	binary.Write(&buf, binary.LittleEndian, uint32(22))           // offset
	buf.Write(pngData)
	return buf.Bytes()
}
