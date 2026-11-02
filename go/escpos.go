package main

import (
	"bytes"
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
