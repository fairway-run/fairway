package dashboard

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDashboardSnapshotCacheHitExpireAndClear(t *testing.T) {
	cache := newDashboardSnapshotCache(25 * time.Millisecond)
	var builds int32
	build := func() (string, error) {
		n := atomic.AddInt32(&builds, 1)
		return "value-" + string(rune('0'+n)), nil
	}
	got, status, err := dashboardSnapshotGet(cache, "k", build)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value-1" || status != "miss" {
		t.Fatalf("first get = %q %s, want value-1 miss", got, status)
	}
	got, status, err = dashboardSnapshotGet(cache, "k", build)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value-1" || status != "hit" {
		t.Fatalf("second get = %q %s, want value-1 hit", got, status)
	}
	cache.clear()
	got, status, err = dashboardSnapshotGet(cache, "k", build)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value-2" || status != "miss" {
		t.Fatalf("after clear = %q %s, want value-2 miss", got, status)
	}
	time.Sleep(30 * time.Millisecond)
	got, status, err = dashboardSnapshotGet(cache, "k", build)
	if err != nil {
		t.Fatal(err)
	}
	if got != "value-3" || status != "miss" {
		t.Fatalf("after expiry = %q %s, want value-3 miss", got, status)
	}
}

func TestDashboardSnapshotCacheCoalescesConcurrentBuilds(t *testing.T) {
	cache := newDashboardSnapshotCache(time.Second)
	var builds int32
	start := make(chan struct{})
	var wg sync.WaitGroup
	statuses := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, status, err := dashboardSnapshotGet(cache, "k", func() (int, error) {
				atomic.AddInt32(&builds, 1)
				time.Sleep(20 * time.Millisecond)
				return 42, nil
			})
			if err != nil {
				t.Errorf("get: %v", err)
				return
			}
			if got != 42 {
				t.Errorf("got %d, want 42", got)
			}
			statuses <- status
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)
	if got := atomic.LoadInt32(&builds); got != 1 {
		t.Fatalf("builds=%d, want 1", got)
	}
	var miss, wait int
	for status := range statuses {
		switch status {
		case "miss":
			miss++
		case "wait":
			wait++
		default:
			t.Fatalf("unexpected status %q", status)
		}
	}
	if miss != 1 || wait != 7 {
		t.Fatalf("miss=%d wait=%d, want 1/7", miss, wait)
	}
}
