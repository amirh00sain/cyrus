//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

const (
	tunDeviceName    = "OperaVPN"
	launcherDetaches = true // Start-Process returns as soon as the helper started
)

func helperExecutable() (string, error) { return os.Executable() }

func elevationAvailable() bool { return true }

func isElevated() bool { return windows.GetCurrentProcessToken().IsElevated() }

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

func winQuote(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

func elevatedCommand(exe string, args []string) (*exec.Cmd, error) {
	if isElevated() {
		return hidden(exec.Command(exe, args...)), nil
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = winQuote(a)
	}
	script := fmt.Sprintf("Start-Process -FilePath '%s' -ArgumentList '%s' -Verb RunAs -WindowStyle Hidden",
		psQuote(exe), psQuote(strings.Join(quoted, " ")))
	return hidden(exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)), nil
}
