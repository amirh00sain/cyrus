package core

import (
	"bufio"
	"embed"
	"net"
	"strings"
)

// irdata/ir-cidrs.txt holds the IPv4 ranges registered in Iran, one CIDR per
// line. It is not committed: scripts/fetch-ir.sh (run by the release workflow)
// downloads it before building. Without it TUN mode on desktop cannot route
// Iranian IP ranges directly (the .ir domain bypass still works in Proxy,
// System and Android TUN mode).
//
//go:embed all:irdata
var irFS embed.FS

// iranCIDRs returns the valid IPv4 CIDRs of the embedded list.
func iranCIDRs() []string {
	f, err := irFS.Open("irdata/ir-cidrs.txt")
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if ip, n, err := net.ParseCIDR(line); err == nil && ip.To4() != nil {
			out = append(out, n.String())
		}
	}
	return out
}
