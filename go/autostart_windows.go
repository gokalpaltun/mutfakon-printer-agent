//go:build windows

// Starts with Windows: on every launch the agent writes its own path under the current user's Run key
// (no admin rights needed), so moving or replacing the exe keeps working after the next start.
package main

import (
	"os"
	"os/exec"
	"syscall"
)

func registerAutostart() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "MutfakOnPrinterAgent", "/t", "REG_SZ", "/d", `"`+exe+`"`, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}
