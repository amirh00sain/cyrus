//go:build !android

package core

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	tunMu  sync.Mutex
	tunDir string
)

func tunSupported() bool { return elevationAvailable() }

func startTun(proxyAddress string, directIR bool) error {
	tunMu.Lock()
	defer tunMu.Unlock()
	if tunDir != "" {
		return nil
	}
	exe, err := helperExecutable()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "opera-tun-")
	if err != nil {
		return err
	}
	// Destinations that must not enter the tunnel: the upstream proxies and,
	// optionally, every Iranian IP range. Passed as a file (the list is long).
	cidrs := tunBypassCIDRs()
	if directIR {
		if ir := iranCIDRs(); len(ir) > 0 {
			cidrs = append(cidrs, ir...)
			sysProxyLog(fmt.Sprintf("tun: %d Iranian IP ranges bypass the tunnel", len(ir)))
		} else {
			sysProxyLog("tun: Iran IP list is not embedded in this build, Iranian sites will use the VPN (see scripts/fetch-ir.sh)")
		}
	}
	bypassFile := filepath.Join(dir, "bypass.txt")
	if err := os.WriteFile(bypassFile, []byte(strings.Join(cidrs, "\n")), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	args := []string{
		"--tun-helper",
		"--device=" + tunDeviceName,
		"--proxy=socks5://" + proxyAddress,
		"--bypass-file=" + bypassFile,
		"--dir=" + dir,
	}
	cmd, err := elevatedCommand(exe, args)
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("could not start the TUN helper: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	fail := func(msg string) error {
		_ = os.WriteFile(filepath.Join(dir, "stop"), nil, 0o600)
		go func() { time.Sleep(5 * time.Second); _ = os.RemoveAll(dir) }()
		return errors.New(msg)
	}

	// The user may need a while to answer the polkit / UAC prompt.
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "ready")); err == nil {
			tunDir = dir
			return nil
		}
		if b, err := os.ReadFile(filepath.Join(dir, "error")); err == nil {
			_ = os.RemoveAll(dir)
			return errors.New(strings.TrimSpace(string(b)))
		}
		select {
		case err := <-exited:
			if err != nil || !launcherDetaches {
				detail := strings.TrimSpace(stderr.String())
				if detail != "" {
					detail = ": " + detail
				}
				return fail("administrator permission was not granted or the TUN helper stopped early" + detail)
			}
		default:
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fail("timed out while waiting for the TUN helper")
}

func stopTun() error {
	tunMu.Lock()
	dir := tunDir
	tunDir = ""
	tunMu.Unlock()
	if dir == "" {
		return nil
	}
	_ = os.WriteFile(filepath.Join(dir, "stop"), nil, 0o600)
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "done")); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = os.RemoveAll(dir)
	return nil
}
