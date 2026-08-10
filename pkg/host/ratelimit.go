package host

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateBucket struct {
	tokens   float64
	last     time.Time
	capacity float64
	refill   float64 // tokens per second
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string]*rateBucket)}
}

func (rl *rateLimiter) allow(key string, capacity, refillPerSec float64) (ok bool, retryAfterSec int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists {
		rl.buckets[key] = &rateBucket{tokens: capacity - 1, last: now, capacity: capacity, refill: refillPerSec}
		return true, 0
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.refill
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.last = now
	if b.tokens < 1 {
		need := 1 - b.tokens
		sec := 1
		if b.refill > 0 {
			sec = int(need/b.refill + 0.999)
			if sec < 1 {
				sec = 1
			}
		}
		return false, sec
	}
	b.tokens--
	return true, 0
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *Host) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		method := r.Method
		ip := clientIP(r)
		var capacity, refill float64
		var class string
		var defaultRetry int

		switch {
		case method == http.MethodPost && (path == "/login" || path == "/register" ||
			path == "/api/auth/login" || path == "/api/auth/register" ||
			path == "/login/2fa" || path == "/api/auth/2fa"):
			capacity, refill, class, defaultRetry = 20, 20.0/60.0, "auth", 30 // 20/min
		case method == http.MethodPost && path == "/api/v1/publish":
			// Light secondary IP guard; durable per-user cooldown is enforced in handle_publish.
			capacity, refill, class, defaultRetry = 8, 8.0/3600.0, "publish_ip", 450 // ~8/hour
		case path == "/api/v1/search" || path == "/search":
			capacity, refill, class, defaultRetry = 60, 60.0/60.0, "search", 30 // 60/min
		default:
			next.ServeHTTP(w, r)
			return
		}

		key := class + ":" + ip
		ok, retryAfter := h.limiter.allow(key, capacity, refill)
		if !ok {
			if retryAfter < 1 {
				retryAfter = defaultRetry
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded","retry_after_seconds":` + strconv.Itoa(retryAfter) + `}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
