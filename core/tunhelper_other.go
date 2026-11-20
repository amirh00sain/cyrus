//go:build !linux && !windows

package core

import (
	"errors"
	"os"
	"os/exec"
)

const (
	tunDeviceName    = "opera0"
	launcherDetaches = false
)

func helperExecutable() (string, error) { return os.Executable() }

func elevationAvailable() bool { return false }

func elevatedCommand(string, []string) (*exec.Cmd, error) {
	return nil, errors.New("TUN mode is not supported on this platform")
}
