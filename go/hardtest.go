package main

import (
	"bytes"
	_ "embed"
	"image/png"
)

// A deliberately hard receipt (tiny text, symbols, rules, gray ramp, solid areas, a long tail) for checking a
// printer end to end. The bitmap is cut exactly the way the app does it: luminance below 160 prints black.
//
//go:embed hardtest.png
var hardTestPNG []byte

func hardTestReceipt(copies int) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(hardTestPNG))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	rowBytes := width / 8
	data := make([]byte, rowBytes*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA() // 16-bit channels
			lum := (299*r + 587*g + 114*bl) / 1000
			if lum < 160*257 {
				data[y*rowBytes+x/8] |= 0x80 >> uint(x%8)
			}
		}
	}
	return rasterReceipt(width, height, data, copies)
}
