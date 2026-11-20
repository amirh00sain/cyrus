package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Alexey71/opera-proxy/dialer"
	se "github.com/Alexey71/opera-proxy/seclient"
)

// Options configures a proxy engine session started from the GUI.
// The GUI never exposes account, registration or "free plan" concepts;
// anonymous device registration stays internal to the engine.
type Options struct {
	// Country is the SurfEasy geo code used by discovery (EU, US, DE, ...).
	Country string
	// BindAddress is the local listen address of the HTTP proxy.
	BindAddress string
	// SocksMode listens for SOCKS5 instead of HTTP requests.
	SocksMode bool
	// DiscoverCSV, when set, replaces the live SurfEasy discovery request
	// with a static CSV file that is shipped/selected from the UI.
	DiscoverCSV string
	// ProxyAddress ("ip:port"), when set, connects straight to that endpoint:
	// no discovery and no server selection run at all.
	ProxyAddress string
	// ProxyHost is the TLS server name of ProxyAddress (e.g. eu5.sec-tunnel.com).
	ProxyHost string
	// ServerSelection is first, random or fastest.
	ServerSelection string
	// BypassPatterns are destination host patterns (e.g. "*.ir") that connect
	// directly instead of through the upstream proxy.
	BypassPatterns []string
	// LogWriter receives engine log lines (the UI Logs tab).
	LogWriter io.Writer
	// Verbosity: 10 debug, 20 info, 30 warning, 40 error, 50 critical, 60 silent.
	Verbosity int
}

// Session is a running proxy engine. Call Stop to shut it down.
type Session struct {
	cancel context.CancelFunc
	done   chan int
}

// Stop asks the engine to shut down. It is safe to call more than once.
func (s *Session) Stop() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

// Done returns a channel that receives the engine exit code when it stops.
func (s *Session) Done() <-chan int { return s.done }

// Wait blocks until the engine stops and returns its exit code.
func (s *Session) Wait() int { return <-s.done }

// DefaultBindAddress is the local address the GUI binds the proxy to.
const DefaultBindAddress = "127.0.0.1:18080"

// Start launches the proxy engine in-process. The returned session owns a
// cancellable context; the engine stops when Session.Stop is called.
func Start(opts Options) (*Session, error) {
	country := strings.TrimSpace(opts.Country)
	if country == "" {
		country = "EU"
	}
	bind := strings.TrimSpace(opts.BindAddress)
	if bind == "" {
		bind = DefaultBindAddress
	}

	args, err := defaultArgs()
	if err != nil {
		return nil, err
	}
	args.country = country
	args.countryExplicit = true
	args.bindAddress = bind
	args.socksMode = opts.SocksMode
	args.discoverCSV = opts.DiscoverCSV
	args.overrideProxyAddress = strings.TrimSpace(opts.ProxyAddress)
	args.overrideProxyHost = strings.TrimSpace(opts.ProxyHost)
	if len(opts.BypassPatterns) > 0 {
		args.proxyBypass = &CSVArg{values: append([]string(nil), opts.BypassPatterns...)}
	}
	args.verbosity = opts.Verbosity
	if args.verbosity == 0 {
		args.verbosity = 20
	}

	sel := strings.ToLower(strings.TrimSpace(opts.ServerSelection))
	switch sel {
	case "first":
		args.serverSelection.value = dialer.ServerSelectionFirst
	case "random":
		args.serverSelection.value = dialer.ServerSelectionRandom
	case "fastest", "":
		args.serverSelection.value = dialer.ServerSelectionFastest
	default:
		return nil, fmt.Errorf("unknown server selection %q", opts.ServerSelection)
	}

	logDst := opts.LogWriter
	if logDst == nil {
		logDst = io.Discard
	}

	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{cancel: cancel, done: make(chan int, 1)}
	go func() {
		code := runWithArgs(ctx, args, logDst)
		session.done <- code
	}()
	return session, nil
}

// defaultArgs builds the same argument defaults as the CLI parser without
// touching the global flag set, so Start can be called repeatedly.
func defaultArgs() (*CLIArgs, error) {
	args := &CLIArgs{
		bootstrapDNS: &CSVArg{values: []string{
			"https://1.1.1.3/dns-query",
			"https://8.8.8.8/dns-query",
			"https://dns.google/dns-query",
			"https://security.cloudflare-dns.com/dns-query",
			"https://fidelity.vm-0.com/q",
			"https://wikimedia-dns.org/dns-query",
			"https://dns.adguard-dns.com/dns-query",
			"https://dns.quad9.net/dns-query",
			"https://doh.cleanbrowsing.org/doh/adult-filter/",
		}},
		proxyBypass:        &CSVArg{},
		serverSelection:    serverSelectionArg{dialer.ServerSelectionFastest},
		discoverRepeat:     1,
		verbosity:          20,
		timeout:            DEFAULT_TIMEOUT,
		apiLogin:           "se0316",
		apiPassword:        "SILrMEPBmJuhomxWkfm3JalqHX2Eheg1YhlEZiMh8II",
		apiClientType:      se.DefaultSESettings.ClientType,
		apiClientVersion:   se.DefaultSESettings.ClientVersion,
		apiUserAgent:       se.DefaultSESettings.UserAgent,
		apiProxyParallel:   15,
		refresh:            4 * time.Hour,
		refreshRetry:       5 * time.Second,
		initRetryInterval:  5 * time.Second,
		proxySpeedTestURL:  "https://ajax.googleapis.com/ajax/libs/angularjs/1.8.2/angular.min.js",
		proxySpeedTimeout:  15 * time.Second,
		proxySpeedDLLimit:  262144,
		serverSelectionTimeout: DEFAULT_SERVER_SELECTION_TIMEOUT,
		serverSelectionTestURL: "https://ajax.googleapis.com/ajax/libs/angularjs/1.8.2/angular.min.js",
		sortProxiesBy:      "speed",
	}
	return args, nil
}
