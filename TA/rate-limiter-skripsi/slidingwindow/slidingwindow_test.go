package slidingwindow

import (
	"sync"
	"testing"
	"time"
)

func TestAllow_DalamBatas(t *testing.T) {
	sw := New(5, time.Second)
	for i := 0; i < 5; i++ {
		if !sw.Allow() {
			t.Fatalf("request ke-%d seharusnya diizinkan", i+1)
		}
	}
}

func TestAllow_MelebihiLimit(t *testing.T) {
	sw := New(3, time.Second)
	for i := 0; i < 3; i++ {
		if !sw.Allow() {
			t.Fatalf("request ke-%d seharusnya diizinkan", i+1)
		}
	}
	if sw.Allow() {
		t.Fatal("request ke-4 seharusnya ditolak")
	}
}

func TestAllow_WindowExpiry(t *testing.T) {
	sw := New(2, 100*time.Millisecond)
	sw.Allow()
	sw.Allow()
	if sw.Allow() {
		t.Fatal("request ke-3 seharusnya ditolak sebelum window expired")
	}
	time.Sleep(150 * time.Millisecond)
	if !sw.Allow() {
		t.Fatal("setelah window expired, request seharusnya diizinkan lagi")
	}
}

func TestAllow_PartialExpiry(t *testing.T) {
	sw := New(3, 200*time.Millisecond)
	sw.Allow()
	time.Sleep(150 * time.Millisecond)
	sw.Allow()
	sw.Allow()
	if sw.Allow() {
		t.Fatal("request ke-4 seharusnya ditolak")
	}
	time.Sleep(60 * time.Millisecond)
	if !sw.Allow() {
		t.Fatal("setelah timestamp pertama expired, seharusnya ada slot tersisa")
	}
}

func TestRemaining(t *testing.T) {
	sw := New(5, time.Second)
	if sw.Remaining() != 5 {
		t.Fatalf("expected remaining 5, got %d", sw.Remaining())
	}
	sw.Allow()
	sw.Allow()
	if sw.Remaining() != 3 {
		t.Fatalf("expected remaining 3, got %d", sw.Remaining())
	}
}

func TestAllow_RaceCondition(t *testing.T) {
	sw := New(100, time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sw.Allow()
		}()
	}
	wg.Wait()
}
