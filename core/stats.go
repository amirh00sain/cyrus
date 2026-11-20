package core

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	speedTestURL = "https://ajax.googleapis.com/ajax/libs/angularjs/1.8.2/angular.min.js"
	uploadTestURL = "https://httpbin.org/post"
	// One probe cycle downloads a small file and uploads a tiny one through the
	// tunnel. Every 5 s was needlessly heavy on mobile data and battery.
	sampleInterval = 20 * time.Second
	sampleTimeout  = 6 * time.Second
)

// SpeedMonitor periodically measures latency, download and upload throughput
// through the running local proxy and pushes the results into the model.
type SpeedMonitor struct {
	model    *Model
	client   *http.Client
	transport *http.Transport
	stop     chan struct{}
	once     sync.Once
	wg       sync.WaitGroup
	upTotal  int64
	downTotal int64
}

// NewSpeedMonitor creates a monitor for the local proxy, given as an
// http://host:port or socks5://host:port URL.
func NewSpeedMonitor(model *Model, proxy string) *SpeedMonitor {
	proxyURL, _ := url.Parse(proxy)
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		MaxIdleConns:          8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	return &SpeedMonitor{
		model:     model,
		client:    &http.Client{Transport: transport, Timeout: sampleTimeout},
		transport: transport,
		stop:      make(chan struct{}),
	}
}

// Start begins sampling in the background.
func (s *SpeedMonitor) Start() {
	s.wg.Add(1)
	SafeGo(func() {
		defer s.wg.Done()
		// Wait for the engine to finish registration/discovery first.
		select {
		case <-time.After(2 * time.Second):
		case <-s.stop:
			return
		}
		ticker := time.NewTicker(sampleInterval)
		defer ticker.Stop()
		s.sample()
		for {
			select {
			case <-ticker.C:
				s.sample()
			case <-s.stop:
				return
			}
		}
	})
}

// Stop ends sampling and waits for the worker to exit.
func (s *SpeedMonitor) Stop() {
	s.once.Do(func() { close(s.stop) })
	s.wg.Wait()
	s.transport.CloseIdleConnections()
}

func (s *SpeedMonitor) sample() {
	ping, downRate, downBytes, ok := s.measureDownload()
	if ok {
		upRate, upBytes := s.measureUpload()
		s.downTotal += downBytes
		s.upTotal += upBytes
		s.model.Update(func(snap *Snapshot) {
			snap.PingMS = int(ping.Milliseconds())
			snap.PingAt = time.Now()
			snap.Download = formatRate(downRate)
			snap.Upload = formatRate(upRate)
			snap.TotalDown = formatBytes(float64(s.downTotal))
			snap.TotalUp = formatBytes(float64(s.upTotal))
		})
	}
}

func (s *SpeedMonitor) measureDownload() (time.Duration, float64, int64, bool) {
	start := time.Now()
	req, err := http.NewRequest(http.MethodGet, speedTestURL, nil)
	if err != nil {
		return 0, 0, 0, false
	}
	req.Header.Set("User-Agent", "OperaVPN/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, 0, 0, false
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, 0, 0, false
	}
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0, 0, 0, false
	}
	n := int64(len(buf))
	return elapsed, float64(n) / elapsed.Seconds(), n, true
}

func (s *SpeedMonitor) measureUpload() (float64, int64) {
	payload := bytes.Repeat([]byte("opera-vpn-speed-test"), 256) // ~5 KB
	start := time.Now()
	req, err := http.NewRequest(http.MethodPost, uploadTestURL, bytes.NewReader(payload))
	if err != nil {
		return 0, int64(len(payload))
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, int64(len(payload))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	resp.Body.Close()
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return 0, int64(len(payload))
	}
	return float64(len(payload)) / elapsed.Seconds(), int64(len(payload))
}
