//go:build windows

package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	tunIP      = "198.18.0.2"
	tunMask    = "255.254.0.0"
	tunGateway = "198.18.0.1"
)

func winRun(name string, args ...string) (string, error) {
	out, err := hidden(exec.Command(name, args...)).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// prepareHelper makes sure wintun.dll sits next to the executable. The DLL is
// embedded at build time (see .github/workflows/build.yml).
func prepareHelper() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	target := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	data, err := wintunFS.ReadFile("wintun/wintun_" + runtime.GOARCH + ".dll")
	if err != nil {
		return fmt.Errorf("wintun.dll is missing: put wintun.dll (from wintun.net) next to %s", filepath.Base(exe))
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("could not write wintun.dll next to the executable: %w", err)
	}
	return nil
}

// windowsDefaultGateway parses `route print -4 0.0.0.0` (numbers are not
// localised) and returns the gateway of the lowest-metric default route.
func windowsDefaultGateway() (string, error) {
	out, err := winRun("route", "print", "-4", "0.0.0.0")
	if err != nil {
		return "", err
	}
	best, bestMetric := "", int(^uint(0)>>1)
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || f[0] != "0.0.0.0" || f[1] != "0.0.0.0" {
			continue
		}
		if net.ParseIP(f[2]) == nil {
			continue // "On-link"
		}
		if m, err := strconv.Atoi(f[4]); err == nil && m < bestMetric {
			best, bestMetric = f[2], m
		}
	}
	if best == "" {
		return "", fmt.Errorf("no default route found - are you online?")
	}
	return best, nil
}

func maskString(cidr string) (network, mask string, err error) {
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", err
	}
	_ = ip
	return n.IP.String(), net.IP(n.Mask).String(), nil
}

func setupRoutes(dev string, bypass []string) (func(), error) {
	gw, err := windowsDefaultGateway()
	if err != nil {
		return nil, err
	}

	var undo []func()
	teardown := func() {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
	}
	fail := func(err error) (func(), error) {
		teardown()
		return nil, err
	}

	// The adapter appears asynchronously; retry for a few seconds.
	var lastErr error
	for i := 0; i < 20; i++ {
		var out string
		out, lastErr = winRun("netsh", "interface", "ipv4", "set", "address",
			"name="+dev, "source=static", "address="+tunIP, "mask="+tunMask, "gateway=none")
		if lastErr == nil {
			break
		}
		lastErr = fmt.Errorf("netsh set address: %w: %s", lastErr, out)
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return fail(lastErr)
	}
	// Resolve names through the tunnel (UDP/53 is answered by the engine).
	_, _ = winRun("netsh", "interface", "ipv4", "set", "dnsservers", "name="+dev, "source=static", "address=1.1.1.1", "validate=no")
	_, _ = winRun("netsh", "interface", "ipv4", "add", "dnsservers", "name="+dev, "address=8.8.8.8", "index=2", "validate=no")
	_, _ = winRun("netsh", "interface", "ipv4", "set", "interface", "name="+dev, "metric=1")

	add := func(network, mask, via string) error {
		if out, err := winRun("route", "add", network, "mask", mask, via, "metric", "1"); err != nil {
			return fmt.Errorf("route add %s: %w: %s", network, err, out)
		}
		undo = append(undo, func() { _, _ = winRun("route", "delete", network, "mask", mask) })
		return nil
	}

	// Upstream proxies and Iranian ranges keep using the real network. A
	// thousand route.exe calls would take minutes one by one, so they run in
	// parallel.
	type netMask struct{ network, mask string }
	var direct []netMask
	for _, cidr := range bypass {
		if network, mask, err := maskString(cidr); err == nil {
			direct = append(direct, netMask{network, mask})
		}
	}
	parallel(direct, func(n netMask) {
		_, _ = winRun("route", "add", n.network, "mask", n.mask, gw, "metric", "1")
	})
	undo = append(undo, func() {
		parallel(direct, func(n netMask) { _, _ = winRun("route", "delete", n.network, "mask", n.mask) })
	})
	for _, network := range []string{"0.0.0.0", "128.0.0.0"} {
		if err := add(network, "128.0.0.0", tunGateway); err != nil {
			return fail(err)
		}
	}
	return teardown, nil
}

// parallel runs fn for every item with a handful of workers.
func parallel[T any](items []T, fn func(T)) {
	const workers = 12
	ch := make(chan T)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				fn(it)
			}
		}()
	}
	for _, it := range items {
		ch <- it
	}
	close(ch)
	wg.Wait()
}
