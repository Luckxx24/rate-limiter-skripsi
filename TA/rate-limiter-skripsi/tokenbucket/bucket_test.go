package tokenbucket

import (
	"testing"
	"time"
)

// Test 1: Bucket baru harus langsung bisa terima request (karena mulai penuh)
func TestBucketBaruPenuh(t *testing.T) {
	bucket := New(10, 5) // kapasitas 10, rate 5/detik

	boleh := bucket.Allow()
	if !boleh {
		t.Error("Bucket baru harusnya penuh dan request pertama harus diizinkan")
	}
}

// Test 2: Kalau request terus-menerus, akhirnya harus diblok
func TestBucketHabisKemudiánBlok(t *testing.T) {
	bucket := New(3, 1) // kapasitas kecil: 3 token saja

	// Kirim 3 request → semuanya harus lolos (menghabiskan token)
	for i := 0; i < 3; i++ {
		if !bucket.Allow() {
			t.Errorf("Request ke-%d harusnya lolos, tapi diblok", i+1)
		}
	}

	// Request ke-4 → harus diblok karena bucket sudah kosong
	if bucket.Allow() {
		t.Error("Request ke-4 harusnya diblok, tapi lolos")
	}
}

// Test 3: Setelah diblok, tunggu sebentar, token harus terisi lagi
func TestRefillSetelahTunggu(t *testing.T) {
	bucket := New(2, 10) // rate 10/detik = 1 token setiap 0.1 detik

	// Habiskan semua token
	bucket.Allow()
	bucket.Allow()

	// Pastikan sekarang diblok
	if bucket.Allow() {
		t.Error("Harusnya diblok setelah token habis")
	}

	// Tunggu 0.2 detik → harusnya ada 2 token baru masuk
	time.Sleep(200 * time.Millisecond)

	// Sekarang harusnya bisa lagi
	if !bucket.Allow() {
		t.Error("Harusnya bisa request lagi setelah tunggu refill")
	}
}

// Test 4: Race condition — ini yang paling penting untuk skripsi
// Jalankan dengan: go test -race ./tokenbucket/
func TestTidakAdaRaceCondition(t *testing.T) {
	bucket := New(100, 50)

	// Jalankan 50 goroutine bersamaan, masing-masing kirim 10 request
	done := make(chan bool, 50)

	for i := 0; i < 50; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				bucket.Allow() // hasilnya tidak penting, yang penting tidak crash
			}
			done <- true
		}()
	}

	// Tunggu semua goroutine selesai
	for i := 0; i < 50; i++ {
		<-done
	}

	// Kalau sampai sini tanpa error → tidak ada race condition
}
