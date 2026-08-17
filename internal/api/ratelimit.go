package api

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// clientIP returns the request's remote address without the port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// loginLimiter tracks failed login attempts per client IP. Once an IP
// exceeds the configured threshold its attempts are refused for the lock
// window; a successful login clears the entry. Thresholds come from config
// (0 disables the limiter).
type loginLimiter struct {
	mu    sync.Mutex
	fails map[string]int
	until map[string]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{fails: make(map[string]int), until: make(map[string]time.Time)}
}

// locked reports whether ip is inside its lock window, clearing expired
// entries along the way.
func (l *loginLimiter) locked(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	until, ok := l.until[ip]
	if !ok {
		return false
	}
	if now.After(until) {
		// Lock window over: start over from a clean slate.
		delete(l.until, ip)
		delete(l.fails, ip)
		return false
	}
	return true
}

// recordFailure counts one failed attempt for ip. When the attempt crosses
// the threshold the entry is locked for the window and the remaining lock
// duration is returned (0 when not (yet) locked).
func (l *loginLimiter) recordFailure(ip string, maxAttempts, lockSeconds int, now time.Time) time.Duration {
	if maxAttempts <= 0 || lockSeconds <= 0 {
		return 0 // limiter disabled
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip]++
	if l.fails[ip] >= maxAttempts {
		until := now.Add(time.Duration(lockSeconds) * time.Second)
		l.until[ip] = until
		return until.Sub(now)
	}
	return 0
}

// clear drops all state for ip after a successful login.
func (l *loginLimiter) clear(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
	delete(l.until, ip)
}

// allowLink admits a short-link redirect against the shared per-second
// budget (link_rate_per_second, 0 = unlimited). The limiter is rebuilt only
// when the configured rate changes, so hot config updates take effect.
func (s *Server) allowLink() bool {
	n := s.cfg.Get().LinkRatePerSecond
	if n <= 0 {
		return true
	}
	s.linkRateMu.Lock()
	if s.linkRate == nil || s.linkRateN != n {
		s.linkRate = rate.NewLimiter(rate.Limit(n), n) // burst = one second's budget
		s.linkRateN = n
	}
	lim := s.linkRate
	s.linkRateMu.Unlock()
	return lim.Allow()
}
