package core

import (
	"net"
	"sort"

	"github.com/Alexey71/opera-proxy/servers"
	"github.com/xjasonlyu/tun2socks/v2/engine"
)

// tunMTU is used for the TUN device and the Android VPN interface.
const tunMTU = 1500

// tunBypassCIDRs lists the destinations that must keep using the physical
// network while TUN is up: the upstream Opera proxies themselves. Without this
// the engine's own connections would be routed into the tunnel they feed.
func tunBypassCIDRs() []string {
	set := map[string]bool{"77.111.244.0/22": true}
	if rows, err := servers.ParseDiscoveredRows(); err == nil {
		for _, r := range rows {
			if ip := net.ParseIP(r.IP); ip != nil && ip.To4() != nil {
				set[ip.String()+"/32"] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// startT2S starts the in-process tun2socks engine on device
// ("tun://name" or "fd://N") forwarding to proxy ("socks5://host:port").
func startT2S(device, proxy string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &tunError{msg: "tun2socks failed to start", cause: r}
		}
	}()
	engine.Insert(&engine.Key{
		Device:   device,
		Proxy:    proxy,
		MTU:      tunMTU,
		LogLevel: "warn",
	})
	engine.Start()
	return nil
}

func stopT2S() {
	defer func() { _ = recover() }()
	engine.Stop()
}

type tunError struct {
	msg   string
	cause any
}

func (e *tunError) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return e.msg + ": " + sprint(e.cause)
}
