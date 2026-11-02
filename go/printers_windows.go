//go:build windows

// Windows printers through winspool.drv directly (no PowerShell): list them and send RAW bytes to the
// spooler, which is what a thermal printer wants.
package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	winspool         = syscall.NewLazyDLL("winspool.drv")
	procEnumPrinters = winspool.NewProc("EnumPrintersW")
	procGetDefault   = winspool.NewProc("GetDefaultPrinterW")
	procOpenPrinter  = winspool.NewProc("OpenPrinterW")
	procClosePrinter = winspool.NewProc("ClosePrinter")
	procStartDoc     = winspool.NewProc("StartDocPrinterW")
	procEndDoc       = winspool.NewProc("EndDocPrinter")
	procStartPage    = winspool.NewProc("StartPagePrinter")
	procEndPage      = winspool.NewProc("EndPagePrinter")
	procWritePrinter = winspool.NewProc("WritePrinter")
)

const (
	printerEnumLocal       = 0x2
	printerEnumConnections = 0x4
)

// PRINTER_INFO_2W; the pointer-sized fields come first, then the 32-bit ones.
type printerInfo2 struct {
	pServerName, pPrinterName, pShareName, pPortName, pDriverName, pComment, pLocation uintptr
	pDevMode, pSepFile, pPrintProcessor, pDatatype, pParameters, pSecurityDescriptor   uintptr
	attributes, priority, defaultPriority, startTime, untilTime, status, cJobs, avgPPM uint32
}

type docInfo1 struct{ pDocName, pOutputFile, pDatatype *uint16 }

// utf16At reads the string a PRINTER_INFO pointer refers to. Those strings live inside the buffer
// EnumPrinters filled, so the pointer is turned into an offset in that buffer.
func utf16At(buf []byte, p uintptr) *string {
	base := uintptr(unsafe.Pointer(&buf[0]))
	if p < base || p >= base+uintptr(len(buf)) {
		return nil
	}
	var chars []uint16
	for i := int(p - base); i+1 < len(buf); i += 2 {
		c := uint16(buf[i]) | uint16(buf[i+1])<<8
		if c == 0 {
			break
		}
		chars = append(chars, c)
	}
	s := syscall.UTF16ToString(chars)
	return &s
}

func defaultPrinterName() string {
	var n uint32
	procGetDefault.Call(0, uintptr(unsafe.Pointer(&n)))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	if r, _, _ := procGetDefault.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func listPrinters() ([]printerInfo, error) {
	flags := uintptr(printerEnumLocal | printerEnumConnections)
	var needed, returned uint32
	procEnumPrinters.Call(flags, 0, 2, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned))) // asks for the size
	if needed == 0 {
		return []printerInfo{}, nil
	}
	buf := make([]byte, needed)
	r, _, err := procEnumPrinters.Call(flags, 0, 2, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if r == 0 {
		return nil, fmt.Errorf("EnumPrinters: %v", err)
	}
	def := defaultPrinterName()
	size := unsafe.Sizeof(printerInfo2{})
	out := make([]printerInfo, 0, returned)
	for i := uint32(0); i < returned; i++ {
		info := (*printerInfo2)(unsafe.Pointer(&buf[uintptr(i)*size]))
		name := utf16At(buf, info.pPrinterName)
		if name == nil {
			continue
		}
		out = append(out, printerInfo{Name: *name, Port: utf16At(buf, info.pPortName), Driver: utf16At(buf, info.pDriverName), IsDefault: *name == def})
	}
	return out, nil
}

func printRaw(printer string, data []byte) error {
	name, err := syscall.UTF16PtrFromString(printer)
	if err != nil {
		return err
	}
	var h uintptr
	if r, _, e := procOpenPrinter.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&h)), 0); r == 0 {
		return fmt.Errorf("OpenPrinter %q: %v", printer, e)
	}
	defer procClosePrinter.Call(h)

	docName, _ := syscall.UTF16PtrFromString("Mutfak On")
	dataType, _ := syscall.UTF16PtrFromString("RAW")
	di := docInfo1{pDocName: docName, pDatatype: dataType}
	if r, _, e := procStartDoc.Call(h, 1, uintptr(unsafe.Pointer(&di))); r == 0 {
		return fmt.Errorf("StartDocPrinter: %v", e)
	}
	defer procEndDoc.Call(h)
	if r, _, e := procStartPage.Call(h); r == 0 {
		return fmt.Errorf("StartPagePrinter: %v", e)
	}
	defer procEndPage.Call(h)
	var written uint32
	if r, _, e := procWritePrinter.Call(h, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&written))); r == 0 {
		return fmt.Errorf("WritePrinter: %v", e)
	}
	return nil
}
