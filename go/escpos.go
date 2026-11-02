package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// The few ESC/POS commands the agent needs itself. Receipt content comes from the app as a bitmap later.
const (
	esc = 0x1b
	gs  = 0x1d
)

var turkishToASCII = strings.NewReplacer("ç", "c", "Ç", "C", "ğ", "g", "Ğ", "G", "ı", "i", "İ", "I", "ö", "o", "Ö", "O", "ş", "s", "Ş", "S", "ü", "u", "Ü", "U")

// asciiOnly keeps the test text printable whatever code page the printer is on.
func asciiOnly(s string) string {
	s = turkishToASCII.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func testReceipt(printerName string) []byte {
	var b bytes.Buffer
	b.Write([]byte{esc, 0x40})    // initialize
	b.Write([]byte{esc, 0x61, 1}) // center
	for _, l := range []string{"MUTFAK ON", "Test fisi / Test print", printerName, time.Now().Format("02.01.2006 15:04")} {
		b.WriteString(asciiOnly(l) + "\n")
	}
	b.WriteString("\n\n\n\n")
	b.Write([]byte{gs, 0x56, 66, 0}) // feed and partial cut
	return b.Bytes()
}

// rasterReceipt builds the print job for a receipt that arrives as a 1-bit bitmap (rows of width/8 bytes,
// most significant bit = leftmost dot, 1 = black). The bitmap goes out in bands (GS v 0) because some printers
// have a small receive buffer; every copy ends with a feed and a cut.
func rasterReceipt(width, height int, data []byte, copies int) ([]byte, error) {
	if width <= 0 || width%8 != 0 || width > 1024 || height <= 0 || height > 10000 {
		return nil, fmt.Errorf("bad bitmap size %dx%d", width, height)
	}
	rowBytes := width / 8
	if len(data) != rowBytes*height {
		return nil, fmt.Errorf("bitmap has %d bytes, expected %d", len(data), rowBytes*height)
	}
	if copies < 1 || copies > 7 {
		return nil, fmt.Errorf("copies must be 1-7")
	}
	const band = 128
	var b bytes.Buffer
	for c := 0; c < copies; c++ {
		b.Write([]byte{esc, 0x40}) // initialize
		for y := 0; y < height; y += band {
			n := band
			if y+n > height {
				n = height - y
			}
			b.Write([]byte{gs, 0x76, 0x30, 0, byte(rowBytes), byte(rowBytes >> 8), byte(n), byte(n >> 8)})
			b.Write(data[y*rowBytes : (y+n)*rowBytes])
		}
		b.WriteString("\n\n\n")
		b.Write([]byte{gs, 0x56, 66, 0}) // feed and partial cut
	}
	return b.Bytes(), nil
}
