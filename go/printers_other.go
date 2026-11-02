//go:build !windows

// macOS/Linux exist only so the agent can be developed away from the kasa.
package main

import (
	"os"
	"os/exec"
	"strings"
)

func listPrinters() ([]printerInfo, error) {
	out, _ := exec.Command("lpstat", "-e").Output()
	def := ""
	if d, err := exec.Command("lpstat", "-d").Output(); err == nil {
		if parts := strings.SplitN(string(d), ":", 2); len(parts) == 2 {
			def = strings.TrimSpace(parts[1])
		}
	}
	printers := []printerInfo{}
	for _, name := range strings.Fields(string(out)) {
		printers = append(printers, printerInfo{Name: name, IsDefault: name == def})
	}
	return printers, nil
}

func printRaw(printer string, data []byte) error {
	f, err := os.CreateTemp("", "mutfakon-print-*.bin")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		return err
	}
	f.Close()
	return exec.Command("lp", "-d", printer, "-o", "raw", f.Name()).Run()
}
