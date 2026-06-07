package tokenbucket

import (
	"sync"
	"time"
)

// TokenBucket adalah struct yang menyimpan state satu "ember token".
// Satu IP address = satu TokenBucket.
type TokenBucket struct {
	capacity   float64    // batas maksimal token yang bisa ditampung
	tokens     float64    // jumlah token yang tersisa SAAT INI
	rate       float64    // berapa token ditambah PER DETIK
	lastRefill time.Time  // kapan terakhir kali token dihitung ulang
	mu         sync.Mutex // kunci agar tidak terjadi race condition
}

// New membuat bucket baru dengan kapasitas dan rate tertentu.
// Bucket dimulai dalam kondisi PENUH.
func New(capacity float64, rate float64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // mulai penuh
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// refill menghitung berapa token yang harus ditambah
// berdasarkan waktu yang sudah berlalu sejak terakhir dicek.
// Fungsi ini dipanggil di dalam Allow(), bukan di background.
func (tb *TokenBucket) refill() {
	now := time.Now()

	// Hitung berapa detik yang sudah berlalu sejak terakhir refill
	elapsed := now.Sub(tb.lastRefill).Seconds()

	// Tambahkan token: waktu berlalu × rate
	// Contoh: 2 detik berlalu, rate 5/detik → tambah 10 token
	tb.tokens += elapsed * tb.rate

	// Jangan melebihi kapasitas maksimal
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}

	// Catat waktu sekarang sebagai waktu refill terakhir
	tb.lastRefill = now
}

// Allow menjawab satu pertanyaan: "bolehkah request ini lewat?"
// Mengembalikan true jika boleh, false jika harus diblok (429).
func (tb *TokenBucket) Allow() bool {
	// Kunci mutex agar hanya satu goroutine yang bisa masuk sekaligus
	tb.mu.Lock()
	defer tb.mu.Unlock() // otomatis dibuka saat fungsi selesai

	// Hitung dulu token yang masuk sejak terakhir ada request
	tb.refill()

	// Cek apakah masih ada token
	if tb.tokens >= 1 {
		tb.tokens -= 1 // konsumsi 1 token untuk request ini
		return true    // request diizinkan
	}

	return false // bucket kosong, request ditolak
}

// Tokens mengembalikan jumlah token saat ini.
// Dipakai untuk expose ke Prometheus metrics nanti.
func (tb *TokenBucket) Tokens() float64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	return tb.tokens
}
