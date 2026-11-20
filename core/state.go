package core

import (
	"strings"
	"sync"
	"time"

	"github.com/Alexey71/opera-proxy/servers"
)

// Mode is how traffic is routed once connected.
type Mode string

const (
	ModeProxy       Mode = "proxy"  // local HTTP/SOCKS proxy only
	ModeSystemProxy Mode = "system" // set the OS-wide proxy
	ModeTun         Mode = "tun"    // TUN/VPN interface
)

// Status is the connection state shown by the power circle.
type Status string

const (
	StatusDisconnected Status = "disconnected"
	StatusConnecting   Status = "connecting"
	StatusConnected    Status = "connected"
	StatusError        Status = "error"
)

// Snapshot is an immutable view of the app state delivered to the UI.
type Snapshot struct {
	Status       Status
	Mode         Mode
	Server       servers.Server
	PingMS       int
	PingAt       time.Time // when PingMS was last measured
	Download     string
	Upload       string
	TotalDown    string
	TotalUp      string
	ConnectedTo  string
	ConnectedIP  string // external IP seen through the VPN, populated after connect
	ErrorMessage string
}

// Model holds state and notifies UI subscribers on every change.
type Model struct {
	mu        sync.RWMutex
	snapshot  Snapshot
	listeners []func(Snapshot)
	log       *LogBuffer
}

// NewModel creates a model initialized to the disconnected state.
func NewModel() *Model {
	m := &Model{log: NewLogBuffer(500)}
	m.snapshot = Snapshot{
		Status:      StatusDisconnected,
		Mode:        ModeProxy,
		Server:      *servers.Find("EU"),
		PingMS:      -1,
		Download:    "—",
		Upload:      "—",
		TotalDown:   "0 B",
		TotalUp:     "0 B",
		ConnectedTo: "Not connected",
		ConnectedIP: "",
	}
	if m.snapshot.Server.Code == "" {
		m.snapshot.Server = servers.Catalog()[0]
	}
	return m
}

// Subscribe registers a listener and returns an unsubscribe function.
func (m *Model) Subscribe(fn func(Snapshot)) func() {
	m.mu.Lock()
	m.listeners = append(m.listeners, fn)
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		for i, l := range m.listeners {
			if funcPtr(l) == funcPtr(fn) {
				m.listeners = append(m.listeners[:i], m.listeners[i+1:]...)
				return
			}
		}
	}
}

// Snapshot returns the current state.
func (m *Model) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot
}

// Update applies a mutation and notifies listeners.
func (m *Model) Update(mutate func(*Snapshot)) {
	m.mu.Lock()
	mutate(&m.snapshot)
	snapshot := m.snapshot
	listeners := append([]func(Snapshot){}, m.listeners...)
	m.mu.Unlock()
	for _, l := range listeners {
		l(snapshot)
	}
}

// Log returns the shared log buffer used by the Logs tab.
func (m *Model) Log() *LogBuffer { return m.log }

// LogBuffer is a bounded, thread-safe line buffer implementing io.Writer.
type LogBuffer struct {
	mu      sync.RWMutex
	lines   []string
	limit   int
	changed []func()
}

// NewLogBuffer creates a buffer that keeps at most limit lines.
func NewLogBuffer(limit int) *LogBuffer {
	if limit <= 0 {
		limit = 500
	}
	return &LogBuffer{limit: limit, lines: make([]string, 0, limit)}
}

// Write implements io.Writer.
func (b *LogBuffer) Write(p []byte) (int, error) {
	text := strings.TrimRight(string(p), "\r\n")
	if text != "" {
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.mu.Lock()
			b.lines = append(b.lines, line)
			if len(b.lines) > b.limit {
				b.lines = append([]string(nil), b.lines[len(b.lines)-b.limit:]...)
			}
			callbacks := append([]func(){}, b.changed...)
			b.mu.Unlock()
			for _, cb := range callbacks {
				cb()
			}
		}
	}
	return len(p), nil
}

// Lines returns a copy of the buffered log lines.
func (b *LogBuffer) Lines() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.lines...)
}

// OnChange registers a callback fired when a new line is appended.
func (b *LogBuffer) OnChange(fn func()) func() {
	b.mu.Lock()
	b.changed = append(b.changed, fn)
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, cb := range b.changed {
			if funcPtr(cb) == funcPtr(fn) {
				b.changed = append(b.changed[:i], b.changed[i+1:]...)
				return
			}
		}
	}
}

// Clear empties the buffer.
func (b *LogBuffer) Clear() {
	b.mu.Lock()
	b.lines = nil
	b.mu.Unlock()
}
