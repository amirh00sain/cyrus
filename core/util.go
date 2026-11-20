package core

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func funcPtr(fn interface{}) uintptr { return reflect.ValueOf(fn).Pointer() }

// SafeGo runs fn in a goroutine and recovers panics. On Android an unrecovered
// panic in any goroutine kills the whole app process, so every background
// worker (engine monitor, speed sampler, pings) goes through this helper.
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("recovered panic in background task: %v", r)
			}
		}()
		fn()
	}()
}

// splitAddress splits a host:port string, defaulting the port to 18080.
func splitAddress(address string) (string, string) {
	address = strings.TrimSpace(address)
	if i := strings.LastIndex(address, ":"); i > 0 {
		return address[:i], address[i+1:]
	}
	return address, "18080"
}

func parsePort(value string) (int, error) {
	p, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || p <= 0 || p > 65535 {
		return 0, fmt.Errorf("invalid port %q", value)
	}
	return p, nil
}

// formatBytes renders a byte count as a human readable string.
func formatBytes(n float64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%.0f B", n)
	}
	div, exp := float64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", n/div, "KMGTPE"[exp])
}

// formatRate renders bytes-per-second.
func formatRate(n float64) string {
	if n <= 0 {
		return "0 B/s"
	}
	return formatBytes(n) + "/s"
}

// formatDuration renders a duration as a stable string for the UI.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	return d.Round(time.Millisecond).String()
}

func sprint(v any) string { return fmt.Sprint(v) }
