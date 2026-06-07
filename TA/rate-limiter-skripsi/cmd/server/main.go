// cmd/server/main.go
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/lucky/rate-limiter/middleware"
)

// response adalah struct untuk format JSON yang kita balas ke client
type response struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// writeJSON adalah helper — tulis response JSON dengan status code tertentu
func writeJSON(w http.ResponseWriter, code int, msg string, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response{Message: msg, Status: status})
}

// handlerLogin mensimulasikan endpoint login
// Ini endpoint sensitif → rate limit ketat (konfigurasi A)
func handlerLogin(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "Login berhasil", "ok")
}

// handlerData mensimulasikan endpoint data umum
// Rate limit lebih longgar (konfigurasi B)
func handlerData(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "Data berhasil diambil", "ok")
}

// handlerHealth adalah health check — TIDAK dipasang rate limiter
// Dipakai Prometheus dan Docker untuk cek apakah server hidup
func handlerHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "Server berjalan", "ok")
}

func main() {
	// Konfigurasi A — ketat, untuk /api/login
	// kapasitas 10 token, isi ulang 5 token/detik
	rlLogin := middleware.NewRateLimiter(10, 5)

	// Konfigurasi B — sedang, untuk /api/data
	// kapasitas 50 token, isi ulang 20 token/detik
	rlData := middleware.NewRateLimiter(50, 20)

	// Daftarkan endpoint
	// /health — tanpa rate limiter
	http.HandleFunc("/health", handlerHealth)

	// /api/login — dibungkus rate limiter ketat
	http.Handle("/api/login", rlLogin.Middleware(http.HandlerFunc(handlerLogin)))

	// /api/data — dibungkus rate limiter sedang
	http.Handle("/api/data", rlData.Middleware(http.HandlerFunc(handlerData)))

	// Jalankan server di port 8080
	log.Println("Server berjalan di :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
