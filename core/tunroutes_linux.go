//go:build linux && !android

package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

const tunAddr = "198.18.0.1/15"

func prepareHelper() error {
	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("the `ip` command (iproute2) is required for TUN mode")
	}
	return nil
}

func ipRun(args ...string) error {
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ipBatch feeds many commands to a single `ip` process. -force keeps going
// when one line fails (for example a range that overlaps another).
func ipBatch(script string) error {
	cmd := exec.Command("ip", "-force", "-batch", "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil && len(bytes.TrimSpace(out)) > 0 {
		// Individual failures are expected (e.g. duplicates); only report
		// when the batch could not run at all.
		if _, isExit := err.(*exec.ExitError); !isExit {
			return fmt.Errorf("ip -batch: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// linuxDefaultRoute returns the gateway and interface of the current IPv4
// default route (gateway is empty for point-to-point links).
func linuxDefaultRoute() (gw, dev string) {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || f[0] != "default" {
			continue
		}
		for i := 0; i+1 < len(f); i++ {
			switch f[i] {
			case "via":
				gw = f[i+1]
			case "dev":
				dev = f[i+1]
			}
		}
		if dev != "" {
			return gw, dev
		}
	}
	return "", ""
}

func setupRoutes(dev string, bypass []string) (func(), error) {
	gw, gwDev := linuxDefaultRoute()
	if gwDev == "" {
		return nil, fmt.Errorf("no default route found - are you online?")
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

	if err := ipRun("addr", "replace", tunAddr, "dev", dev); err != nil {
		return fail(err)
	}
	if err := ipRun("link", "set", "dev", dev, "up"); err != nil {
		return fail(err)
	}

	// Upstream proxies and Iranian ranges keep using the real network. There
	// can be a thousand of them, so they go through one `ip -batch` call.
	if len(bypass) > 0 {
		via := ""
		if gw != "" {
			via = " via " + gw
		}
		var add, del strings.Builder
		for _, cidr := range bypass {
			fmt.Fprintf(&add, "route replace %s%s dev %s\n", cidr, via, gwDev)
			fmt.Fprintf(&del, "route del %s dev %s\n", cidr, gwDev)
		}
		if err := ipBatch(add.String()); err != nil {
			return fail(err)
		}
		undo = append(undo, func() { _ = ipBatch(del.String()) })
	}

	// Two /1 routes beat the default route without replacing it, so nothing
	// has to be restored on teardown but the routes themselves.
	for _, cidr := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := ipRun("route", "replace", cidr, "dev", dev); err != nil {
			return fail(err)
		}
		c := cidr
		undo = append(undo, func() { _ = ipRun("route", "del", c, "dev", dev) })
	}
	// IPv6 would bypass the tunnel entirely; steer it into the device (which
	// has no v6 address, so it simply fails fast and apps fall back to IPv4).
	for _, cidr := range []string{"::/1", "8000::/1"} {
		if err := ipRun("-6", "route", "replace", cidr, "dev", dev); err == nil {
			c := cidr
			undo = append(undo, func() { _ = ipRun("-6", "route", "del", c, "dev", dev) })
		}
	}
	return teardown, nil
}
