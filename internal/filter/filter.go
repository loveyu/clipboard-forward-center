package filter

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type entry struct {
	hash      string
	timestamp time.Time
}

type Filter struct {
	mu      sync.RWMutex
	records map[string][]entry
	window  time.Duration
}

func New(window time.Duration) *Filter {
	return &Filter{
		records: make(map[string][]entry),
		window:  window,
	}
}

func ComputeHash(typeStr, content string) string {
	h := sha256.Sum256([]byte(typeStr + ":" + content))
	return hex.EncodeToString(h[:])
}

func (f *Filter) ShouldForward(client, hash string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries := f.records[client]
	now := time.Now()
	for _, e := range entries {
		if e.hash == hash && now.Sub(e.timestamp) < f.window {
			return false
		}
	}
	return true
}

func (f *Filter) Record(client, hash string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	entries := f.records[client]
	cleaned := entries[:0]
	for _, e := range entries {
		if now.Sub(e.timestamp) < f.window {
			cleaned = append(cleaned, e)
		}
	}

	f.records[client] = append(cleaned, entry{hash: hash, timestamp: now})
}
