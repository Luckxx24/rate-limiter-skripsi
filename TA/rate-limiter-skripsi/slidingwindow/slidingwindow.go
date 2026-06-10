package slidingwindow

import (
	"sync"
	"time"
)

// SlidingWindow menyimpan timestamps request dalam satu window waktu.
// Berbeda dengan Token Bucket yang pakai "token", Sliding Window
// menyimpan catatan kapan tiap request masuk, lalu menghitung
// berapa request yang terjadi dalam rentang waktu terakhir.
type SlidingWindow struct {
	mu         sync.Mutex
	timestamps []time.Time   // catatan waktu tiap request yang lolos
	limit      int           // maksimal request per window
	window     time.Duration // durasi window (misal 1 detik)
}

// New membuat SlidingWindow baru.
// limit  = maksimal request yang diizinkan dalam satu window
// window = durasi window waktu (misal time.Second)
func New(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		timestamps: make([]time.Time, 0, limit),
		limit:      limit,
		window:     window,
	}
}

// Allow memeriksa apakah request boleh diproses.
// Mengembalikan true jika masih dalam batas, false jika sudah melebihi limit.
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	batasWaktu := now.Add(-sw.window) // waktu paling lama yang masih dihitung

	// buang timestamps yang sudah di luar window
	// analoginya: buang tiket yang sudah kadaluarsa
	sw.evict(batasWaktu)

	// cek apakah jumlah request dalam window sudah mencapai limit
	if len(sw.timestamps) >= sw.limit {
		return false // tolak request
	}

	// catat timestamp request ini
	sw.timestamps = append(sw.timestamps, now)
	return true
}

// evict membuang timestamps yang sudah lebih tua dari batasWaktu.
// Dipanggil setiap kali Allow() dieksekusi supaya slice tidak membengkak.
// Karena timestamps selalu ditambah secara urut (append),
// yang kadaluarsa pasti ada di awal slice.
func (sw *SlidingWindow) evict(batasWaktu time.Time) {
	// cari index pertama yang masih valid (belum kadaluarsa)
	i := 0
	for i < len(sw.timestamps) && sw.timestamps[i].Before(batasWaktu) {
		i++
	}
	// buang semua sebelum index i
	sw.timestamps = sw.timestamps[i:]
}

// Remaining mengembalikan sisa kapasitas request dalam window saat ini.
// Berguna untuk header X-RateLimit-Remaining.
func (sw *SlidingWindow) Remaining() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	batasWaktu := now.Add(-sw.window)
	sw.evict(batasWaktu)

	sisa := sw.limit - len(sw.timestamps)
	if sisa < 0 {
		return 0
	}
	return sisa
}
