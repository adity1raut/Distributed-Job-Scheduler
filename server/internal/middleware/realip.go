package middleware

import (
	"net"
	"net/http"
	"strings"
)

// RealIP rewrites r.RemoteAddr to the originating client address when the
// request arrived through a trusted reverse proxy.
//
// This matters for more than logging: the rate limiter buckets
// unauthenticated requests (register, login) by RemoteAddr. Behind nginx or
// an ingress controller every such request carries the proxy's address, so
// without this the entire internet would share a single login bucket — one
// noisy client would lock everyone out, and per-IP brute-force protection
// would be gone.
//
// X-Forwarded-For is trivially spoofable by the client, so it is honoured
// only when the immediate peer is inside trustedProxies. With an empty
// trust list the header is ignored entirely, which is the correct default
// for an API exposed directly to the internet.
func RealIP(trustedProxies []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := clientIP(r, trustedProxies); ip != "" {
				r.RemoteAddr = ip
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request, trustedProxies []*net.IPNet) string {
	if len(trustedProxies) == 0 {
		return ""
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil || !isTrusted(peer, trustedProxies) {
		return ""
	}

	// Walk X-Forwarded-For right to left and stop at the first address that
	// isn't one of our own proxies. Anything further left was appended by an
	// untrusted hop and may be forged.
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return ""
	}
	parts := strings.Split(forwarded, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			return ""
		}
		if !isTrusted(ip, trustedProxies) {
			return ip.String()
		}
	}
	return ""
}

func isTrusted(ip net.IP, trustedProxies []*net.IPNet) bool {
	for _, n := range trustedProxies {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseTrustedProxies turns the TRUSTED_PROXIES config value into CIDR
// blocks, ignoring blank entries. A bare IP is accepted and treated as a
// single-host block, since "the one nginx in front of me" is the common
// case and writing /32 by hand is easy to get wrong.
func ParseTrustedProxies(values []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, block, err := net.ParseCIDR(v); err == nil {
			nets = append(nets, block)
			continue
		}
		if ip := net.ParseIP(v); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		}
	}
	return nets
}
