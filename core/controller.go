package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Alexey71/opera-proxy/engine"
	"github.com/Alexey71/opera-proxy/servers"
)

// Controller owns the proxy engine, the connection state and the routing
// modes. All public methods are safe to call from the UI thread.
type Controller struct {
	model    *Model
	mu       sync.Mutex
	session  *engine.Session
	csvDir   string
	stats    *SpeedMonitor
	sysProxy *SystemProxy
	tun      *TunAdapter
	bind     string
	gen      uint64
	applied  Mode // routing mode currently applied to the OS ("" = none)
	bypassIR bool // keep .ir sites / Iranian IPs out of the VPN
}

// NewController creates a controller bound to the given model.
func NewController(model *Model) *Controller {
	sysProxyLog = func(line string) { _, _ = model.Log().Write([]byte(line + "\n")) }
	// A previous run may have been killed while the system proxy was set.
	RecoverSystemProxy()
	return &Controller{
		bypassIR: true,
		model:    model,
		bind:     engine.DefaultBindAddress,
		sysProxy: NewSystemProxy(engine.DefaultBindAddress),
		tun:      NewTunAdapter(engine.DefaultBindAddress),
	}
}

// SetBypassIR chooses whether .ir sites bypass the VPN. It takes effect on the
// next connect.
func (c *Controller) SetBypassIR(on bool) {
	c.mu.Lock()
	c.bypassIR = on
	c.mu.Unlock()
}

// BypassIR reports the current setting.
func (c *Controller) BypassIR() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bypassIR
}

// Model returns the shared state model.
func (c *Controller) Model() *Model { return c.model }

// AvailableModes returns the modes exposed by the UI for this platform.
// Android only offers TUN and Proxy; desktop adds System proxy.
func AvailableModes() []Mode {
	if isAndroid() {
		return []Mode{ModeProxy, ModeTun}
	}
	return []Mode{ModeProxy, ModeSystemProxy, ModeTun}
}

// SupportsSystemProxy reports whether the OS proxy can be changed here.
func (c *Controller) SupportsSystemProxy() bool {
	return !isAndroid() && NewSystemProxy(c.bind).Enabled()
}

// SupportsTun reports whether TUN mode can be enabled here.
func (c *Controller) SupportsTun() bool { return c.tun.Supported() }

// SetMode changes the routing mode. While connected the engine is restarted:
// TUN needs it to speak SOCKS, the other modes need the HTTP proxy.
func (c *Controller) SetMode(mode Mode) {
	prev := c.model.Snapshot()
	if prev.Mode == mode {
		return
	}
	c.model.Update(func(s *Snapshot) { s.Mode = mode })
	if prev.Status == StatusConnected || prev.Status == StatusConnecting {
		SafeGo(func() {
			_ = c.Disconnect()
			_ = c.Connect()
		})
	}
}

// proxyURL is the local endpoint the stats and IP probes go through.
func (c *Controller) proxyURL(mode Mode) string {
	if mode == ModeTun {
		return "socks5://" + c.bind
	}
	return "http://" + c.bind
}

// SetServer selects a location. If connected, the engine is restarted so the
// new location takes effect.
func (c *Controller) SetServer(s servers.Server) {
	c.model.Update(func(snap *Snapshot) { snap.Server = s })
	if c.model.Snapshot().Status == StatusConnected {
		SafeGo(func() {
			if err := c.Disconnect(); err == nil {
				_ = c.Connect()
			}
		})
	}
}

// Connect starts the proxy engine and tracks it until it stops.
// If the selected server is "Fastest", it first pings all servers and picks
// the one with the lowest measured latency.
func (c *Controller) Connect() error {
	c.mu.Lock()
	if c.session != nil {
		c.mu.Unlock()
		return errors.New("already connected")
	}
	c.gen++
	gen := c.gen

	snap := c.model.Snapshot()
	bypassIR := c.bypassIR
	c.mu.Unlock()

	// The engine connects .ir hosts directly. On desktop TUN that is not
	// possible (its own direct connections would re-enter the tunnel), so
	// there Iranian IP ranges are excluded at the routing level instead.
	var bypassPatterns []string
	if bypassIR && (snap.Mode != ModeTun || isAndroid()) {
		bypassPatterns = []string{"*.ir"}
	}

	// Resolve the fastest server before starting the engine.
	targetServer := snap.Server
	if snap.Server.Code == servers.FastestCode {
		if best := c.resolveFastest(); best != nil {
			targetServer = *best
			c.model.Update(func(s *Snapshot) {
				s.Server = targetServer
			})
			c.model.Log().Write([]byte(fmt.Sprintf(
				"[fastest] selected %s (%d ms)\n", targetServer.Name, targetServer.BaseDelay)))
		}
	}

	csvPath, err := servers.WriteDiscoveredCSV()
	if err != nil {
		c.model.Update(func(s *Snapshot) {
			s.Status = StatusError
			s.ErrorMessage = err.Error()
		})
		return err
	}
	c.model.Update(func(s *Snapshot) {
		s.Status = StatusConnecting
		s.ErrorMessage = ""
		s.ConnectedIP = ""
		s.PingMS = targetServer.BaseDelay
	})

	c.mu.Lock()
	session, err := engine.Start(engine.Options{
		Country:         servers.EngineCountry(targetServer.Code),
		BindAddress:     c.bind,
		SocksMode:       snap.Mode == ModeTun,
		BypassPatterns:  bypassPatterns,
		DiscoverCSV:     csvPath,
		// Connect straight to the chosen server: skips the discovery and
		// server-selection steps of the engine.
		ProxyAddress:    net.JoinHostPort(targetServer.IP, strconv.Itoa(targetServer.Port)),
		ProxyHost:       targetServer.Host,
		ServerSelection: "fastest",
		LogWriter:       c.model.Log(),
		Verbosity:       20,
	})
	if err != nil {
		c.mu.Unlock()
		_ = os.RemoveAll(csvPathDir(csvPath))
		c.model.Update(func(s *Snapshot) {
			s.Status = StatusError
			s.ErrorMessage = err.Error()
		})
		return err
	}
	c.session = session
	c.csvDir = csvPathDir(csvPath)
	c.mu.Unlock()

	SafeGo(func() { c.monitor(gen, session, snap.Mode, targetServer) })
	return nil
}

// resolveFastest pings all known servers concurrently and returns the one
// with the lowest latency. Returns nil if all pings fail.
func (c *Controller) resolveFastest() *servers.Server {
	c.model.Log().Write([]byte("[fastest] pinging all servers to find lowest latency...\n"))
	all := append(servers.Automatic()[:0:0], servers.Automatic()...)
	all = append(all, servers.Catalog()...)
	// Filter out the meta-entries themselves.
	var candidates []servers.Server
	for _, s := range all {
		if s.Code != servers.FastestCode && s.Code != servers.RecommendedCode {
			candidates = append(candidates, s)
		}
	}

	type result struct {
		s  servers.Server
		ms int
	}
	ch := make(chan result, len(candidates))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, s := range candidates {
		srv := s
		wg.Add(1)
		SafeGo(func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			d, err := servers.Ping(srv, 3*time.Second)
			if err == nil {
				ms := int(d.Milliseconds())
				if ms < 1 {
					ms = 1
				}
				ch <- result{srv, ms}
			}
		})
	}
	wg.Wait()
	close(ch)

	var best *servers.Server
	bestMS := int(^uint(0) >> 1)
	for r := range ch {
		if r.ms < bestMS {
			bestMS = r.ms
			s := r.s
			s.BaseDelay = r.ms
			best = &s
		}
	}
	return best
}

// Disconnect stops the engine, restores routing and resets the state.
func (c *Controller) Disconnect() error {
	c.mu.Lock()
	c.gen++
	session := c.session
	c.session = nil
	stats := c.stats
	c.stats = nil
	csvDir := c.csvDir
	c.csvDir = ""
	mode := c.applied
	c.applied = ""
	c.mu.Unlock()

	if stats != nil {
		stats.Stop()
	}
	c.leaveMode(mode)
	if session != nil {
		session.Stop()
		// Let the engine release its port so an immediate Connect() (mode or
		// server change) can bind it again.
		waitPortFree(c.bind, 5*time.Second)
	}
	if csvDir != "" {
		_ = os.RemoveAll(csvDir)
	}
	c.model.Update(func(s *Snapshot) {
		s.Status = StatusDisconnected
		s.ConnectedTo = "Not connected"
		s.ConnectedIP = ""
		s.ErrorMessage = ""
		s.Download = "—"
		s.Upload = "—"
		s.TotalDown = "0 B"
		s.TotalUp = "0 B"
	})
	return nil
}

// Toggle connects or disconnects depending on the current state.
func (c *Controller) Toggle() error {
	snap := c.model.Snapshot()
	if snap.Status == StatusDisconnected || snap.Status == StatusError {
		return c.Connect()
	}
	return c.Disconnect()
}

// Ping measures a server's real TCP latency for the Servers tab.
func (c *Controller) Ping(s servers.Server) (time.Duration, error) {
	return servers.Ping(s, 3*time.Second)
}

// TestPing measures the selected server and stores the result in the model.
func (c *Controller) TestPing() {
	srv := c.model.Snapshot().Server
	SafeGo(func() {
		d, err := servers.Ping(srv, 3*time.Second)
		ms := -1
		if err == nil {
			ms = int(d.Milliseconds())
			if ms < 1 {
				ms = 1
			}
		}
		c.model.Update(func(s *Snapshot) {
			s.PingMS = ms
			s.PingAt = time.Now()
		})
	})
}

func (c *Controller) monitor(gen uint64, session *engine.Session, mode Mode, server servers.Server) {
	if !waitForPort(c.bind, 45*time.Second, session.Done()) {
		select {
		case code := <-session.Done():
			if c.isCurrent(gen) {
				msg := fmt.Sprintf("Proxy engine exited before connecting (code %d)", code)
				c.model.Update(func(s *Snapshot) {
					s.Status = StatusError
					s.ErrorMessage = msg
				})
			}
		default:
			if c.isCurrent(gen) {
				c.model.Update(func(s *Snapshot) {
					s.Status = StatusError
					s.ErrorMessage = "Timed out while connecting. Check your network and try again."
				})
			}
		}
		c.cleanupFailed(gen, session)
		return
	}
	if !c.isCurrent(gen) {
		return
	}

	c.mu.Lock()
	c.applied = mode
	c.mu.Unlock()
	if err := c.applyMode(mode); err != nil {
		c.model.Log().Write([]byte("routing: " + err.Error() + "\n"))
		if c.isCurrent(gen) {
			// The user asked for this routing mode; running without it would
			// silently leave their traffic unprotected.
			msg := err.Error()
			_ = c.Disconnect()
			c.model.Update(func(s *Snapshot) {
				s.Status = StatusError
				s.ErrorMessage = msg
			})
		}
		return
	}
	if !c.isCurrent(gen) {
		// Disconnected while the permission prompt was open.
		c.leaveMode(mode)
		return
	}

	c.model.Update(func(s *Snapshot) {
		s.Status = StatusConnected
		s.ErrorMessage = ""
		s.ConnectedTo = server.Name
		s.PingMS = server.BaseDelay
	})

	// Fetch the external IP asynchronously so the UI isn't blocked.
	SafeGo(func() {
		ip := fetchExternalIP(c.proxyURL(mode))
		if ip != "" && c.isCurrent(gen) {
			c.model.Update(func(s *Snapshot) { s.ConnectedIP = ip })
		}
	})

	stats := NewSpeedMonitor(c.model, c.proxyURL(mode))
	c.mu.Lock()
	if c.gen != gen {
		c.mu.Unlock()
		stats.Stop()
		return
	}
	c.stats = stats
	c.mu.Unlock()
	stats.Start()

	code := <-session.Done()
	if c.isCurrent(gen) {
		// Not a user-requested disconnect: record it so the Logs tab shows
		// exactly when and why the connection dropped.
		c.model.Log().Write([]byte(fmt.Sprintf("[monitor] proxy engine stopped on its own (exit code %d)\n", code)))
		msg := ""
		if code != 0 {
			msg = fmt.Sprintf("Proxy engine stopped unexpectedly (code %d)", code)
		}
		_ = c.Disconnect()
		if msg != "" {
			c.model.Update(func(s *Snapshot) {
				s.Status = StatusError
				s.ErrorMessage = msg
			})
		}
	}
}

// fetchExternalIP queries the external IP address through the running proxy
// (proxy is an http:// or socks5:// URL).
func fetchExternalIP(proxy string) string {
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return ""
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			TLSHandshakeTimeout: 5 * time.Second,
			DisableKeepAlives:   true,
		},
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get("https://api.ipify.org/?format=json")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ""
	}
	var data struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	return data.IP
}

func (c *Controller) cleanupFailed(gen uint64, session *engine.Session) {
	c.mu.Lock()
	if c.gen != gen {
		c.mu.Unlock()
		return
	}
	csvDir := c.csvDir
	c.session = nil
	c.csvDir = ""
	c.mu.Unlock()
	session.Stop()
	if csvDir != "" {
		_ = os.RemoveAll(csvDir)
	}
}

func (c *Controller) applyMode(mode Mode) error {
	bypassIR := c.BypassIR()
	switch mode {
	case ModeSystemProxy:
		c.sysProxy.BypassIR = bypassIR
		return c.sysProxy.Enable()
	case ModeTun:
		c.tun.DirectIR = bypassIR
		return c.tun.Start()
	default:
		return nil
	}
}

func (c *Controller) leaveMode(mode Mode) {
	switch mode {
	case ModeSystemProxy:
		_ = c.sysProxy.Disable()
	case ModeTun:
		_ = c.tun.Stop()
	}
}

func (c *Controller) isCurrent(gen uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen == gen
}

func waitForPort(address string, timeout time.Duration, done <-chan int) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-done:
			return false
		default:
		}
		conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func waitPortFree(address string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
}

func csvPathDir(path string) string {
	if i := strings.LastIndex(path, string(os.PathSeparator)); i > 0 {
		return path[:i]
	}
	return ""
}

func isAndroid() bool { return runtime.GOOS == "android" }
