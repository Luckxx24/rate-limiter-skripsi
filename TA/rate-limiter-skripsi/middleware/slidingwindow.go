// middleware/slidingwindow.go
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/lucky/rate-limiter/slidingwindow"
)

// SlidingWindowLimiter menyimpan satu SlidingWindow per IP address.
// Strukturnya sengaja dibuat mirip RateLimiter supaya mudah dibandingkan.
type SlidingWindowLimiter struct {
	windows map[string]*slidingwindow.SlidingWindow // key = IP address
	mu      sync.RWMutex
	limit   int           // maksimal request per window untuk setiap IP baru
	window  time.Duration // durasi window untuk setiap IP baru
}

// NewSlidingWindowLimiter membuat SlidingWindowLimiter baru.
func NewSlidingWindowLimiter(limit int, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		windows: make(map[string]*slidingwindow.SlidingWindow),
		limit:   limit,
		window:  window,
	}
}

// getWindow mengambil SlidingWindow untuk IP tertentu.
// Polanya sama persis dengan getBucket() di ratelimit.go.
func (sl *SlidingWindowLimiter) getWindow(ip string) *slidingwindow.SlidingWindow {
	sl.mu.RLock()
	sw, ada := sl.windows[ip]
	sl.mu.RUnlock()

	if ada {
		return sw
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()

	// double-check setelah dapat lock tulis
	sw, ada = sl.windows[ip]
	if ada {
		return sw
	}

	sw = slidingwindow.New(sl.limit, sl.window)
	sl.windows[ip] = sw
	return sw
}

// Middleware membungkus handler dengan sliding window rate limiting.
func (sl *SlidingWindowLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r.RemoteAddr)

		sw := sl.getWindow(ip)

		if !sw.Allow() {
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
