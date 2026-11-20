package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Alexey71/opera-proxy/dialer"
	"github.com/things-go/go-socks5"
	"github.com/things-go/go-socks5/statute"
)

// TUN mode hands every packet of the system to tun2socks, which forwards TCP
// as SOCKS5 CONNECT and UDP as SOCKS5 UDP ASSOCIATE. The upstream Opera proxy
// is an HTTPS CONNECT proxy and can only carry TCP, so general UDP cannot be
// tunnelled (browsers fall back from QUIC to TCP on their own). DNS is the one
// UDP protocol the system cannot live without, so the ASSOCIATE handler below
// answers UDP/53 queries by sending them as DNS-over-HTTPS through the same
// upstream proxy. Every other UDP datagram is dropped.

var dohServers = []string{
	"https://1.1.1.1/dns-query",
	"https://8.8.8.8/dns-query",
	"https://9.9.9.9/dns-query",
}

var (
	dohOnce   sync.Once
	dohClient *http.Client
)

func dohHTTPClient(up dialer.ContextDialer) *http.Client {
	dohOnce.Do(func() {
		dohClient = &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				DialContext:           up.DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          8,
				MaxIdleConnsPerHost:   4,
				IdleConnTimeout:       60 * time.Second,
				TLSHandshakeTimeout:   8 * time.Second,
				ResponseHeaderTimeout: 8 * time.Second,
			},
		}
	})
	return dohClient
}

func handleSocksAssociate(ctx context.Context, writer io.Writer, request *socks5.Request, up dialer.ContextDialer) error {
	ctrl, ok := writer.(net.Conn)
	if !ok {
		return fmt.Errorf("writer is %T, expected net.Conn", writer)
	}
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		_ = socks5.SendReply(writer, statute.RepServerFailure, nil)
		return fmt.Errorf("udp listen failed: %v", err)
	}
	defer pc.Close()
	if err := socks5.SendReply(writer, statute.RepSuccess, pc.LocalAddr()); err != nil {
		return fmt.Errorf("failed to send reply, %v", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The association lives as long as the TCP control connection.
	go func() {
		buf := make([]byte, 64)
		for {
			if _, err := ctrl.Read(buf); err != nil {
				break
			}
		}
		cancel()
		_ = pc.Close()
	}()

	buf := make([]byte, 65535)
	for {
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return nil
		}
		hdrLen, port, ok := parseSocksUDPHeader(buf[:n])
		if !ok || port != 53 {
			continue // only DNS is supported
		}
		hdr := append([]byte(nil), buf[:hdrLen]...)
		query := append([]byte(nil), buf[hdrLen:n]...)
		go func() {
			resp := resolveDoH(ctx, up, query)
			if resp == nil {
				return
			}
			learnDirectAnswers(resp)
			out := make([]byte, 0, len(hdr)+len(resp))
			out = append(out, hdr...)
			out = append(out, resp...)
			_, _ = pc.WriteTo(out, from)
		}()
	}
}

// parseSocksUDPHeader returns the header length and destination port of a
// SOCKS5 UDP datagram (RSV RSV FRAG ATYP ADDR PORT DATA).
func parseSocksUDPHeader(b []byte) (hdrLen int, port int, ok bool) {
	if len(b) < 10 || b[0] != 0 || b[1] != 0 || b[2] != 0 {
		return 0, 0, false // fragmentation is not supported
	}
	switch b[3] {
	case 0x01:
		hdrLen = 4 + 4 + 2
	case 0x04:
		hdrLen = 4 + 16 + 2
	case 0x03:
		hdrLen = 4 + 1 + int(b[4]) + 2
	default:
		return 0, 0, false
	}
	if len(b) < hdrLen {
		return 0, 0, false
	}
	return hdrLen, int(binary.BigEndian.Uint16(b[hdrLen-2 : hdrLen])), true
}

func resolveDoH(ctx context.Context, up dialer.ContextDialer, query []byte) []byte {
	client := dohHTTPClient(up)
	for _, server := range dohServers {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server, bytes.NewReader(query))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 65535))
		resp.Body.Close()
		if err == nil && resp.StatusCode == http.StatusOK && len(body) >= 12 {
			return body
		}
	}
	return nil
}
