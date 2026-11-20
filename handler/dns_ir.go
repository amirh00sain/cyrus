package handler

import (
	"encoding/binary"
	"net"
	"strings"
	"time"

	"github.com/Alexey71/opera-proxy/dialer"
)

// learnDirectAnswers inspects a DNS response and, when the question was for a
// .ir name, remembers the returned addresses so that the TUN SOCKS server can
// connect to them directly (it only sees IPs, never host names).
func learnDirectAnswers(msg []byte) {
	if len(msg) < 12 {
		return
	}
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))
	off := 12
	qname := ""
	for i := 0; i < qd; i++ {
		name, next, ok := readDNSName(msg, off)
		if !ok {
			return
		}
		if i == 0 {
			qname = name
		}
		off = next + 4 // QTYPE + QCLASS
		if off > len(msg) {
			return
		}
	}
	if qname != "ir" && !strings.HasSuffix(qname, ".ir") {
		return
	}
	for i := 0; i < an; i++ {
		_, next, ok := readDNSName(msg, off)
		if !ok || next+10 > len(msg) {
			return
		}
		typ := binary.BigEndian.Uint16(msg[next:])
		ttl := time.Duration(binary.BigEndian.Uint32(msg[next+4:])) * time.Second
		rdlen := int(binary.BigEndian.Uint16(msg[next+8:]))
		off = next + 10
		if off+rdlen > len(msg) {
			return
		}
		if (typ == 1 && rdlen == 4) || (typ == 28 && rdlen == 16) {
			if ttl < time.Minute {
				ttl = time.Minute
			}
			if ttl > time.Hour {
				ttl = time.Hour
			}
			ip := net.IP(append([]byte(nil), msg[off:off+rdlen]...))
			dialer.MarkDirectIP(ip, ttl)
		}
		off += rdlen
	}
}

// readDNSName decodes a (possibly compressed) name at off and returns it in
// lower case together with the offset just after it.
func readDNSName(msg []byte, off int) (string, int, bool) {
	var labels []string
	next := -1
	for steps := 0; steps < 255; steps++ {
		if off >= len(msg) {
			return "", 0, false
		}
		l := int(msg[off])
		switch {
		case l == 0:
			if next < 0 {
				next = off + 1
			}
			return strings.ToLower(strings.Join(labels, ".")), next, true
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) {
				return "", 0, false
			}
			if next < 0 {
				next = off + 2
			}
			off = int(binary.BigEndian.Uint16(msg[off:]) & 0x3FFF)
		case l&0xC0 != 0:
			return "", 0, false
		default:
			if off+1+l > len(msg) {
				return "", 0, false
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
	return "", 0, false
}
