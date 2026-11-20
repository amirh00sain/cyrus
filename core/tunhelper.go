//go:build !android

package core

import (
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaybeRunHelper must be the first call in main(). When the executable was
// started as "<exe> --tun-helper ...", it runs the privileged TUN helper and
// exits; otherwise it returns immediately and the GUI starts as usual.
func MaybeRunHelper() {
	if len(os.Args) < 2 || os.Args[1] != "--tun-helper" {
		return
	}
	os.Exit(runTunHelper(os.Args[2:]))
}

// The helper talks to the GUI through files in --dir:
//
//	ready  written once the device and routes are up
//	error  written (with a message) if setup failed
//	stop   created by the GUI to ask the helper to shut down
//	done   written by the helper after it cleaned up
func runTunHelper(args []string) (code int) {
	fs := flag.NewFlagSet("tun-helper", flag.ContinueOnError)
	device := fs.String("device", tunDeviceName, "TUN device name")
	proxy := fs.String("proxy", "", "socks5://host:port of the local engine")
	bypassFile := fs.String("bypass-file", "", "file with one CIDR per line that skips the tunnel")
	dir := fs.String("dir", "", "control directory")
	if err := fs.Parse(args); err != nil || *dir == "" || *proxy == "" {
		return 2
	}
	status := func(name, msg string) {
		_ = os.WriteFile(filepath.Join(*dir, name), []byte(msg), 0o600)
	}
	defer func() {
		if r := recover(); r != nil {
			status("error", fmt.Sprintf("TUN helper crashed: %v", r))
			code = 1
		}
	}()

	if err := prepareHelper(); err != nil {
		status("error", err.Error())
		return 1
	}
	if err := startT2S("tun://"+*device, *proxy); err != nil {
		status("error", err.Error())
		return 1
	}
	var cidrs []string
	if data, err := os.ReadFile(*bypassFile); err == nil {
		for _, c := range strings.Split(string(data), "\n") {
			if c = strings.TrimSpace(c); c != "" {
				cidrs = append(cidrs, c)
			}
		}
	}
	teardown, err := setupRoutes(*device, cidrs)
	if err != nil {
		stopT2S()
		status("error", err.Error())
		return 1
	}
	status("ready", "ok")

	waitForStop(*dir, *proxy)

	teardown()
	stopT2S()
	status("done", "ok")
	return 0
}

// waitForStop returns when the GUI asks for it or when the GUI (and with it
// the local engine) has disappeared, so a crashed GUI never leaves the
// machine without a working default route.
func waitForStop(dir, proxy string) {
	addr := proxy
	if u, err := url.Parse(proxy); err == nil && u.Host != "" {
		addr = u.Host
	}
	misses := 0
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	n := 0
	for range tick.C {
		if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil {
			return
		}
		n++
		if n%10 != 0 { // probe the engine every ~3 s
			continue
		}
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			misses++
			if misses >= 3 {
				return
			}
			continue
		}
		_ = conn.Close()
		misses = 0
	}
}
