// middleware/ratelimit.go
package middleware

import (
	"net/http"
	"sync"

	"github.com/lucky/rate-limiter/tokenbucket"
)

// RateLimiter menyimpan satu bucket per IP address.
// Analoginya: ini adalah "buku daftar tamu" di pintu restoran.
type RateLimiter struct {
	buckets  map[string]*tokenbucket.TokenBucket // key = IP address
	mu       sync.RWMutex                        // RWMutex karena baca lebih sering dari tulis
	capacity float64                             // kapasitas bucket untuk setiap IP baru
	rate     float64                             // rate isi ulang untuk setiap IP baru
}

// NewRateLimiter membuat RateLimiter baru.
// capacity dan rate akan dipakai untuk SETIAP IP yang baru pertama kali muncul.
func NewRateLimiter(capacity float64, rate float64) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*tokenbucket.TokenBucket),
		capacity: capacity,
		rate:     rate,
	}
}

// getBucket mengambil bucket untuk IP tertentu.
// Kalau IP belum pernah masuk, buat bucket baru untuk dia.
func (rl *RateLimiter) getBucket(ip string) *tokenbucket.TokenBucket {
	// Coba baca dulu (RLock — banyak goroutine boleh baca bersamaan)
	rl.mu.RLock()
	bucket, ada := rl.buckets[ip]
	rl.mu.RUnlock()

	// Kalau sudah ada, langsung return — tidak perlu lock tulis
	if ada {
		return bucket
	}

	// IP baru → perlu tulis ke map → pakai Lock eksklusif
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Cek lagi setelah dapat lock tulis.
	// Kenapa? Mungkin goroutine lain sudah insert duluan
	// di antara RUnlock() dan Lock() tadi.
	bucket, ada = rl.buckets[ip]
	if ada {
		return bucket
	}

	// Benar-benar baru → buat bucket dan simpan ke map
	bucket = tokenbucket.New(rl.capacity, rl.rate)
	rl.buckets[ip] = bucket
	return bucket
}

// Middleware adalah fungsi utama yang membungkus handler.
// Penggunaannya: http.Handle("/path", rl.Middleware(handlerAsli))
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	// http.HandlerFunc adalah cara Go untuk konversi func biasa jadi http.Handler
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ambil IP dari request
		// r.RemoteAddr formatnya "ip:port", jadi kita perlu pisahkan
		ip := extractIP(r.RemoteAddr)

		// Ambil bucket untuk IP ini (buat baru kalau belum ada)
		bucket := rl.getBucket(ip)

		// Tanya bucket: boleh lewat tidak?
		if !bucket.Allow() {
			// Ditolak → tambahkan header standar rate limit
			w.Header().Set("X-RateLimit-Remaining", "0")
			// Balas dengan HTTP 429 Too Many Requests
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return // STOP — jangan panggil next
		}

		// Boleh lewat → teruskan ke handler asli
		next.ServeHTTP(w, r)
	})
}

// extractIP memisahkan IP dari string "ip:port".
// Contoh: "192.168.1.1:54321" → "192.168.1.1"
func extractIP(remoteAddr string) string {
	// Cari posisi tanda ":" terakhir (untuk handle IPv6 juga)
	for i := len(remoteAddr) - 1; i >= 0; i-- {
		if remoteAddr[i] == ':' {
			return remoteAddr[:i] // ambil bagian sebelum ":"
		}
	}
	// Kalau tidak ada ":", return apa adanya
	return remoteAddr
}
