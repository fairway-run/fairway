package dashboard

import (
	"sync"
	"time"
)

const dashboardSnapshotCacheTTL = 2 * time.Second

type dashboardSnapshotCache struct {
	ttl      time.Duration
	mu       sync.Mutex
	entries  map[string]dashboardSnapshotEntry
	inflight map[string]*dashboardSnapshotCall
}

type dashboardSnapshotEntry struct {
	value     any
	expiresAt time.Time
}

type dashboardSnapshotCall struct {
	done  chan struct{}
	value any
	err   error
}

func newDashboardSnapshotCache(ttl time.Duration) *dashboardSnapshotCache {
	return &dashboardSnapshotCache{
		ttl:      ttl,
		entries:  map[string]dashboardSnapshotEntry{},
		inflight: map[string]*dashboardSnapshotCall{},
	}
}

func dashboardSnapshotGet[T any](cache *dashboardSnapshotCache, key string, build func() (T, error)) (T, string, error) {
	var zero T
	if cache == nil || cache.ttl <= 0 || key == "" {
		value, err := build()
		return value, "bypass", err
	}
	now := time.Now()
	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok {
		if now.Before(entry.expiresAt) {
			value, ok := entry.value.(T)
			cache.mu.Unlock()
			if ok {
				return value, "hit", nil
			}
			return zero, "hit_type_mismatch", nil
		}
		delete(cache.entries, key)
	}
	if call, ok := cache.inflight[key]; ok {
		cache.mu.Unlock()
		<-call.done
		if call.err != nil {
			return zero, "wait_error", call.err
		}
		value, ok := call.value.(T)
		if !ok {
			return zero, "wait_type_mismatch", nil
		}
		return value, "wait", nil
	}
	call := &dashboardSnapshotCall{done: make(chan struct{})}
	cache.inflight[key] = call
	cache.mu.Unlock()

	value, err := build()

	cache.mu.Lock()
	call.value = value
	call.err = err
	if err == nil {
		cache.entries[key] = dashboardSnapshotEntry{value: value, expiresAt: time.Now().Add(cache.ttl)}
	}
	delete(cache.inflight, key)
	close(call.done)
	cache.mu.Unlock()

	if err != nil {
		return zero, "miss_error", err
	}
	return value, "miss", nil
}

func (cache *dashboardSnapshotCache) clear() {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	cache.entries = map[string]dashboardSnapshotEntry{}
	cache.mu.Unlock()
}
