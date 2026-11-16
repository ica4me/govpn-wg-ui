package console

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	loginAttempts    = 5
	loginWindow      = time.Minute
	loginMaxClients  = 10000
	loginConcurrency = 4
)

type loginBucket struct {
	attempts int
	expires  time.Time
}
type loginLimiter struct {
	mu      sync.Mutex
	clients map[string]loginBucket
	active  chan struct{}
}

// admit never evicts live buckets: saturation fails closed, bounding memory
// without allowing rotating source addresses to erase another client's limit.
func (l *loginLimiter) admit(ip string, now time.Time) (func(), int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, exists := l.clients[ip]
	if !exists || !now.Before(b.expires) {
		// Sweep only when at capacity; normal admission stays O(1).
		if len(l.clients) >= loginMaxClients {
			for key, bucket := range l.clients {
				if !now.Before(bucket.expires) {
					delete(l.clients, key)
				}
			}
		}
		if len(l.clients) >= loginMaxClients {
			return nil, 60
		}
		b = loginBucket{expires: now.Add(loginWindow)}
	}
	if b.attempts >= loginAttempts {
		return nil, max(1, int(b.expires.Sub(now).Seconds()+0.999))
	}
	b.attempts++
	l.clients[ip] = b
	select {
	case l.active <- struct{}{}:
		return func() { <-l.active }, 0
	default:
		return nil, 1
	}
}

// SetTrustedProxies configures the explicit proxy allowlist before serving.
// Empty means direct peers only. Catch-all networks are rejected.
func (s *Server) SetTrustedProxies(cidrs string) error {
	var prefixes []netip.Prefix
	if strings.TrimSpace(cidrs) != "" {
		for _, raw := range strings.Split(cidrs, ",") {
			p, err := netip.ParsePrefix(strings.TrimSpace(raw))
			if err != nil || p.Bits() == 0 {
				return fmt.Errorf("invalid trusted proxy CIDR %q", raw)
			}
			prefixes = append(prefixes, p.Masked())
		}
	}
	s.trustedProxies = prefixes
	return nil
}

func (s *Server) trusted(ip netip.Addr) bool {
	for _, p := range s.trustedProxies {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return "unknown"
	}
	peer = peer.Unmap()
	if !s.trusted(peer) {
		return peer.String()
	}
	// Walk from the trusted immediate peer toward the client. Never trust the
	// leftmost address blindly: a client may have supplied that part of XFF.
	raw := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if len(raw) > 4096 {
		return peer.String()
	}
	chain := strings.Split(raw, ",")
	if len(chain) > 32 {
		return peer.String()
	}
	candidate := peer
	for i := len(chain) - 1; i >= 0; i-- {
		addr, e := netip.ParseAddr(strings.TrimSpace(chain[i]))
		if e != nil {
			return peer.String()
		}
		candidate = addr.Unmap()
		if !s.trusted(candidate) {
			break
		}
	}
	return candidate.String()
}

func (s *Server) loginAdmission(w http.ResponseWriter, r *http.Request, v Session) func() {
	release, retry := s.logins.admit(s.clientIP(r), time.Now())
	if release == nil {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = Login(v.CSRF, "Too many sign-in attempts. Please wait and try again.").Render(r.Context(), w)
	}
	return release
}
