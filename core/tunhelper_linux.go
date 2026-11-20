//go:build linux && !android

package core

import (
	"errors"
	"os"
	"os/exec"
)

const (
	tunDeviceName    = "opera0"
	launcherDetaches = false // pkexec stays alive as long as the helper runs
)

func helperExecutable() (string, error) {
	// Inside an AppImage os.Executable() points into a FUSE mount that root
	// cannot read; the AppImage file itself can be started by root.
	if p := os.Getenv("APPIMAGE"); p != "" {
		return p, nil
	}
	return os.Executable()
}

func elevationAvailable() bool {
	if os.Geteuid() == 0 {
		return true
	}
	_, err := exec.LookPath("pkexec")
	return err == nil
}

func elevatedCommand(exe string, args []string) (*exec.Cmd, error) {
	if os.Geteuid() == 0 {
		return exec.Command(exe, args...), nil
	}
	pk, err := exec.LookPath("pkexec")
	if err != nil {
		return nil, errors.New("TUN needs root: install polkit (pkexec) with an authentication agent " +
			"(on Hyprland e.g. hyprpolkitagent), or start the app with sudo")
	}
	return exec.Command(pk, append([]string{exe}, args...)...), nil
}
