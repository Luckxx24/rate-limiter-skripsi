// middleware/ratelimit_test.go
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// handler dummy — selalu balas 200 OK
// ini adalah "handler asli" yang dibungkus middleware
var handlerDummy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// Test 1: Request pertama harus lolos (bucket baru = penuh)
func TestRequestPertamaLolos(t *testing.T) {
	rl := NewRateLimiter(5, 1)
	handler := rl.Middleware(handlerDummy)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345" // simulasi IP client
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Request pertama harusnya 200, dapat %d", rec.Code)
	}
}

// Test 2: Setelah token habis, harus dapat 429
func TestDiblokSetelahTokenHabis(t *testing.T) {
	rl := NewRateLimiter(3, 1) // hanya 3 token
	handler := rl.Middleware(handlerDummy)

	ip := "10.0.0.1:9999"

	// Habiskan semua token
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Request ke-%d harusnya 200, dapat %d", i+1, rec.Code)
		}
	}

	// Request ke-4 harus 429
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = ip
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("Request ke-4 harusnya 429, dapat %d", rec.Code)
	}
}

// Test 3: IP berbeda punya bucket masing-masing, tidak saling ganggu
func TestIPBerbedaBucketTerpisah(t *testing.T) {
	rl := NewRateLimiter(2, 1) // hanya 2 token per IP
	handler := rl.Middleware(handlerDummy)

	// Habiskan token IP A
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.1.1.1:1111"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP A sekarang habis — pastikan 429
	reqA := httptest.NewRequest("GET", "/", nil)
	reqA.RemoteAddr = "1.1.1.1:1111"
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusTooManyRequests {
		t.Errorf("IP A harusnya 429, dapat %d", recA.Code)
	}

	// IP B belum pernah request — harusnya masih 200
	reqB := httptest.NewRequest("GET", "/", nil)
	reqB.RemoteAddr = "2.2.2.2:2222"
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Errorf("IP B harusnya 200, dapat %d", recB.Code)
	}
}

// Test 4: Header X-RateLimit-Remaining harus ada saat diblok
func TestHeaderRateLimitSaatDiblok(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 token saja
	handler := rl.Middleware(handlerDummy)

	ip := "5.5.5.5:5555"

	// Habiskan 1 token
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = ip
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// Request kedua harus diblok + ada header
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = ip
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("Harusnya 429, dapat %d", rec2.Code)
	}

	header := rec2.Header().Get("X-RateLimit-Remaining")
	if header != "0" {
		t.Errorf("Header X-RateLimit-Remaining harusnya '0', dapat '%s'", header)
	}
}

// Test 5: Race condition — banyak goroutine, IP berbeda-beda
func TestTidakAdaRaceCondition(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	handler := rl.Middleware(handlerDummy)

	done := make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func(id int) {
			// Setiap goroutine pakai IP berbeda
			ip := fmt.Sprintf("10.0.%d.1:8080", id)
			for j := 0; j < 10; j++ {
				req := httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = ip
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}
}
