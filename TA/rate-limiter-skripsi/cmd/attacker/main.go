// cmd/attacker/main.go
package main

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	// ── Konfigurasi via command line flag ──────────────────────────
	url := flag.String("url", "http://localhost:8080/api/login", "Target URL")
	rps := flag.Int("rps", 100, "Request per second")
	durasi := flag.Int("durasi", 10, "Durasi serangan dalam detik")
	goroutines := flag.Int("goroutines", 10, "Jumlah goroutine paralel")
	flag.Parse()

	fmt.Printf("Target   : %s\n", *url)
	fmt.Printf("RPS      : %d\n", *rps)
	fmt.Printf("Durasi   : %d detik\n", *durasi)
	fmt.Printf("Goroutine: %d\n", *goroutines)
	fmt.Println("Memulai serangan...")
	fmt.Println("─────────────────────────────────")

	// ── Counter hasil — pakai atomic agar aman dari race condition ──
	var total200 int64 // jumlah response 200 OK
	var total429 int64 // jumlah response 429 Too Many Requests
	var totalErr int64 // jumlah error jaringan

	// ── Ticker = metronom pengatur RPS ─────────────────────────────
	// Interval antar request = 1 detik / RPS
	interval := time.Second / time.Duration(*rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// ── Timer = batas durasi serangan ──────────────────────────────
	selesai := time.After(time.Duration(*durasi) * time.Second)

	// ── Channel sebagai antrian request ────────────────────────────
	// Setiap tick → kirim sinyal ke channel → goroutine ambil dan eksekusi
	jobs := make(chan struct{}, *rps)

	// ── WaitGroup untuk tunggu semua goroutine selesai ─────────────
	var wg sync.WaitGroup

	// ── Spawn goroutine worker ─────────────────────────────────────
	for i := 0; i < *goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 5 * time.Second}

			for range jobs {
				// Kirim request ke target
				resp, err := client.Get(*url)
				if err != nil {
					// Error jaringan (server mati, timeout, dll)
					atomic.AddInt64(&totalErr, 1)
					continue
				}
				resp.Body.Close()

				// Catat status code
				switch resp.StatusCode {
				case http.StatusOK:
					atomic.AddInt64(&total200, 1)
				case http.StatusTooManyRequests:
					atomic.AddInt64(&total429, 1)
				}
			}
		}()
	}

	// ── Loop utama: isi jobs setiap tick sampai durasi habis ────────
loop:
	for {
		select {
		case <-selesai:
			// Waktu habis → stop
			break loop
		case <-ticker.C:
			// Setiap tick → kirim 1 job ke channel
			// Non-blocking: kalau channel penuh, skip (hindari deadlock)
			select {
			case jobs <- struct{}{}:
			default:
			}
		}
	}

	// Tutup channel → goroutine worker akan selesai setelah jobs habis
	close(jobs)
	wg.Wait()

	// ── Print hasil ─────────────────────────────────────────────────
	totalRequest := total200 + total429 + totalErr
	fmt.Println("─────────────────────────────────")
	fmt.Printf("Total request  : %d\n", totalRequest)
	fmt.Printf("200 OK         : %d (%.1f%%)\n", total200, persen(total200, totalRequest))
	fmt.Printf("429 Diblok     : %d (%.1f%%)\n", total429, persen(total429, totalRequest))
	fmt.Printf("Error jaringan : %d (%.1f%%)\n", totalErr, persen(totalErr, totalRequest))
	fmt.Printf("Efektivitas blok: %.1f%%\n", persen(total429, total200+total429))
}

// persen menghitung persentase a dari total
func persen(a, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(a) / float64(total) * 100
}
