package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/lucky/rate-limiter/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ── Definisi metrics Prometheus ────────────────────────────────────────

var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP request masuk",
	},
	[]string{"endpoint", "status"},
)

var rateLimitBlockedTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rate_limit_blocked_total",
		Help: "Total request yang diblok rate limiter",
	},
	[]string{"endpoint"},
)

var tokenBucketLevel = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "token_bucket_level",
		Help: "Jumlah token tersisa di bucket",
	},
	[]string{"endpoint"},
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(rateLimitBlockedTotal)
	prometheus.MustRegister(tokenBucketLevel)
}

// ── Interface limiter ───────────────────────────────────────────────────

// Limiter adalah interface yang bisa dipenuhi oleh RateLimiter maupun
// SlidingWindowLimiter — supaya withMetrics tidak perlu tahu algoritma mana.
// Analoginya: dua satpam berbeda tapi keduanya punya fungsi yang sama: "boleh masuk?"
type Limiter interface {
	Middleware(next http.Handler) http.Handler
}

// ── Struct response JSON ────────────────────────────────────────────────

type response struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

func writeJSON(w http.ResponseWriter, code int, msg string, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{Message: msg, Status: status})
}

// ── Handler ─────────────────────────────────────────────────────────────

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	httpRequestsTotal.WithLabelValues("/api/login", "200").Inc()
	writeJSON(w, http.StatusOK, "Login berhasil", "ok")
}

func handlerData(w http.ResponseWriter, r *http.Request) {
	httpRequestsTotal.WithLabelValues("/api/data", "200").Inc()
	writeJSON(w, http.StatusOK, "Data berhasil diambil", "ok")
}

func handlerHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "Server berjalan", "ok")
}

// ── statusRecorder ──────────────────────────────────────────────────────

// statusRecorder menangkap status code yang ditulis ke ResponseWriter
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// ── Middleware wrapper dengan metrics ───────────────────────────────────

// withMetrics sekarang terima interface Limiter, bukan *RateLimiter langsung.
// Jadi bisa dipakai untuk Token Bucket maupun Sliding Window.
func withMetrics(endpoint string, limiter Limiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: 200}

		limiter.Middleware(next).ServeHTTP(rw, r)

		if rw.status == http.StatusTooManyRequests {
			rateLimitBlockedTotal.WithLabelValues(endpoint).Inc()
			httpRequestsTotal.WithLabelValues(endpoint, "429").Inc()
		}
	})
}

// ── Main ────────────────────────────────────────────────────────────────

func main() {
	// ── Token Bucket limiters ──
	// Konfigurasi A — ketat, untuk /api/login
	rlLogin := middleware.NewRateLimiter(10, 5)
	// Konfigurasi B — sedang, untuk /api/data
	rlData := middleware.NewRateLimiter(50, 20)

	// ── Sliding Window limiters ──
	// Konfigurasi setara dengan Token Bucket supaya perbandingan adil:
	// limit 10 per detik untuk login, limit 50 per detik untuk data
	swLogin := middleware.NewSlidingWindowLimiter(10, time.Second)
	swData := middleware.NewSlidingWindowLimiter(50, time.Second)

	// ── Endpoint Token Bucket ──
	http.HandleFunc("/health", handlerHealth)
	http.Handle("/api/login",
		withMetrics("/api/login", rlLogin, http.HandlerFunc(handlerLogin)))
	http.Handle("/api/data",
		withMetrics("/api/data", rlData, http.HandlerFunc(handlerData)))

	// ── Endpoint Sliding Window ──
	// Endpoint terpisah supaya attacker bisa uji keduanya secara independen
	http.Handle("/api/login-sw",
		withMetrics("/api/login-sw", swLogin, http.HandlerFunc(handlerLogin)))
	http.Handle("/api/data-sw",
		withMetrics("/api/data-sw", swData, http.HandlerFunc(handlerData)))

	// ── Prometheus metrics ──
	http.Handle("/metrics", promhttp.Handler())

	log.Println("Server berjalan di :8080")
	log.Println("Endpoint Token Bucket : /api/login  /api/data")
	log.Println("Endpoint Sliding Window: /api/login-sw  /api/data-sw")
	log.Println("Metrics tersedia di   : /metrics")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
