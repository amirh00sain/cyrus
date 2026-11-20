// Package servers ships a ready-to-use location catalog together with the
// endpoints that were collected by the engine's discovery step. The GUI does
// not perform a live "search for servers": the discovered list is embedded in
// the source (discovered.csv) and is written to a temp file for the engine.
//
// The embedded data was generated from the real SurfEasy discovery output:
//
//	opera-proxy -list-proxies-all-out discovered.csv
//
// That API currently exposes three geo regions (EU, AM and AS) with the nine
// endpoints listed below. BaseDelay values are real TCP connect timings to the
// endpoint IPs at the time of generation.
package servers

import (
	"embed"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// discoveredCSV is the pre-baked result of the SurfEasy discovery request.
// It uses the exact CSV schema the engine's -discover-csv loader expects.
//
//go:embed discovered.csv
var discoveredFS embed.FS

// Server is one selectable location in the Servers tab.
type Server struct {
	Code      string // engine country code (EU, AM, AS)
	Name      string
	Flag      string
	Region    string
	Host      string
	IP        string
	Port      int
	BaseDelay int // ms, measured at catalog generation time
}

const (
	RecommendedCode = "RECOMMENDED"
	FastestCode     = "FASTEST"
)

// Automatic choices, matching the reference UI. Both use the EU pool.
func Automatic() []Server {
	return []Server{
		{Code: RecommendedCode, Name: "Recommended", Flag: "✨", Region: "Picks the best location for you.", Host: "eu5.sec-tunnel.com", IP: "77.111.247.23", Port: 443, BaseDelay: 173},
		{Code: FastestCode, Name: "Fastest", Flag: "⚡", Region: "Lowest delay at each connection.", Host: "eu8.sec-tunnel.com", IP: "77.111.247.80", Port: 443, BaseDelay: 155},
	}
}

// Catalog returns the discovered endpoints as selectable locations.
// Flags are regional because the SurfEasy API exposes regions, not countries.
func Catalog() []Server {
	return []Server{
		{Code: "EU", Name: "Europe 1", Flag: "🇪🇺", Region: "Europe · eu5", Host: "eu5.sec-tunnel.com", IP: "77.111.247.23", Port: 443, BaseDelay: 173},
		{Code: "EU", Name: "Europe 2", Flag: "🇪🇺", Region: "Europe · eu7", Host: "eu7.sec-tunnel.com", IP: "77.111.247.36", Port: 443, BaseDelay: 166},
		{Code: "EU", Name: "Europe 3", Flag: "🇪🇺", Region: "Europe · eu8", Host: "eu8.sec-tunnel.com", IP: "77.111.247.80", Port: 443, BaseDelay: 155},
		{Code: "EU", Name: "Europe 4", Flag: "🇪🇺", Region: "Europe · eu6", Host: "eu6.sec-tunnel.com", IP: "77.111.247.25", Port: 443, BaseDelay: 168},
		{Code: "AM", Name: "Americas 1", Flag: "🌎", Region: "Americas · am0", Host: "am0.sec-tunnel.com", IP: "77.111.246.21", Port: 443, BaseDelay: 232},
		{Code: "AM", Name: "Americas 2", Flag: "🌎", Region: "Americas · am3", Host: "am3.sec-tunnel.com", IP: "77.111.246.64", Port: 443, BaseDelay: 238},
		{Code: "AM", Name: "Americas 3", Flag: "🌎", Region: "Americas · am2", Host: "am2.sec-tunnel.com", IP: "77.111.246.63", Port: 443, BaseDelay: 244},
		{Code: "AM", Name: "Americas 4", Flag: "🌎", Region: "Americas · am1", Host: "am1.sec-tunnel.com", IP: "77.111.246.27", Port: 443, BaseDelay: 245},
		{Code: "AS", Name: "Asia", Flag: "🌏", Region: "Asia · as4", Host: "as4.sec-tunnel.com", IP: "77.111.245.14", Port: 443, BaseDelay: 384},
	}
}

// Find returns a server by name or code, or nil.
func Find(code string) *Server {
	for _, s := range Catalog() {
		if strings.EqualFold(s.Code, code) || strings.EqualFold(s.Name, code) {
			cp := s
			return &cp
		}
	}
	for _, s := range Automatic() {
		if strings.EqualFold(s.Code, code) || strings.EqualFold(s.Name, code) {
			cp := s
			return &cp
		}
	}
	return nil
}

// EngineCountry maps an automatic selection to a concrete region code that
// the engine understands. Recommended/Fastest use the EU pool.
func EngineCountry(code string) string {
	switch strings.ToUpper(code) {
	case RecommendedCode, FastestCode, "":
		return "EU"
	default:
		return strings.ToUpper(code)
	}
}

// WriteDiscoveredCSV materializes the embedded discovery data to a temp file
// and returns its path. The engine then uses this file instead of issuing a
// live discovery request, so the server list is always ready and stable.
func WriteDiscoveredCSV() (string, error) {
	data, err := discoveredFS.ReadFile("discovered.csv")
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "opera-vpn-discovery-")
	if err != nil {
		// Android apps may not be able to write to the global temp dir.
		if cache, cacheErr := os.UserCacheDir(); cacheErr == nil {
			dir, err = os.MkdirTemp(cache, "opera-vpn-discovery-")
		}
		if err != nil {
			return "", err
		}
	}
	path := filepath.Join(dir, "discovered.csv")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return path, nil
}

// Ping measures a real TCP connect latency to a server endpoint. It dials the
// discovered IP directly (hostnames resolve to fake benchmark addresses on
// some networks) and returns the round-trip time.
func Ping(s Server, timeout time.Duration) (time.Duration, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	addr := net.JoinHostPort(s.IP, strconv.Itoa(s.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return elapsed, nil
}

// SortFastest sorts servers by their current latency value (lower first).
func SortFastest(servers []Server, latency map[string]int) {
	sort.SliceStable(servers, func(i, j int) bool {
		return latencyOf(servers[i].Name, latency) < latencyOf(servers[j].Name, latency)
	})
}

func latencyOf(name string, latency map[string]int) int {
	if v, ok := latency[name]; ok {
		return v
	}
	for _, s := range append(Catalog(), Automatic()...) {
		if s.Name == name {
			return s.BaseDelay
		}
	}
	return 9999
}

// ParseDiscoveredRows returns the embedded discovery rows in a structured
// form, useful for diagnostics and the About screen.
func ParseDiscoveredRows() ([]Server, error) {
	data, err := discoveredFS.ReadFile("discovered.csv")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	rows := make([]Server, 0, len(lines))
	for i, line := range lines[1:] {
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			return nil, fmt.Errorf("discovered.csv line %d: expected 5 columns", i+2)
		}
		port, err := strconv.Atoi(strings.TrimSpace(fields[4]))
		if err != nil {
			return nil, fmt.Errorf("discovered.csv line %d: invalid port: %w", i+2, err)
		}
		rows = append(rows, Server{
			Code:   strings.TrimSpace(fields[0]),
			Name:   strings.TrimSpace(fields[1]),
			Host:   strings.TrimSpace(fields[2]),
			IP:     strings.TrimSpace(fields[3]),
			Port:   port,
		})
	}
	return rows, nil
}
