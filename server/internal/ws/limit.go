package ws

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// bucket is a token bucket: it holds up to burst tokens and refills at
// rate per second; each allowed event takes one.
type bucket struct {
	tokens float64
	last   time.Time
}

func (b *bucket) allow(now time.Time, rate, burst float64) bool {
	if b.last.IsZero() {
		b.tokens = burst
	} else {
		b.tokens = min(burst, b.tokens+now.Sub(b.last).Seconds()*rate)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// ipLimiter rate-limits by client IP address.
type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   float64
}

func newIPLimiter(perMinute, burst float64) *ipLimiter {
	return &ipLimiter{buckets: map[string]*bucket{}, rate: perMinute / 60, burst: burst}
}

func (l *ipLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buckets) > 10_000 {
		l.buckets = map[string]*bucket{} // bound memory; a reset only forgives
	}
	b := l.buckets[ip]
	if b == nil {
		b = &bucket{}
		l.buckets[ip] = b
	}
	return b.allow(now, l.rate, l.burst)
}

// clientIP is the caller's address. Behind Fly.io's proxy the real client
// is in Fly-Client-IP; that header is only trusted when configured, since
// anyone can send it directly otherwise.
func (s *Server) clientIP(r *http.Request) string {
	if s.TrustProxy {
		if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
