package store

import (
	"sync"
	"time"
)

type Entry struct {
	Content     []byte
	ContentType string
	Timestamp   time.Time
}

type Store struct {
	mu         sync.RWMutex
	entries    map[string]*Entry
	order      []string
	maxRecords int
	expire     time.Duration
}

func New(maxRecords int, expire time.Duration) *Store {
	return &Store{
		entries:    make(map[string]*Entry),
		maxRecords: maxRecords,
		expire:     expire,
	}
}

func (s *Store) key(client, msgID string) string {
	return client + "/" + msgID
}

func (s *Store) Put(client, msgID string, content []byte, contentType string) {
	k := s.key(client, msgID)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked()

	if _, exists := s.entries[k]; !exists {
		if len(s.order) >= s.maxRecords {
			delete(s.entries, s.order[0])
			s.order = s.order[1:]
		}
		s.order = append(s.order, k)
	}

	s.entries[k] = &Entry{
		Content:     content,
		ContentType: contentType,
		Timestamp:   time.Now(),
	}
}

func (s *Store) Get(client, msgID string) ([]byte, string, bool) {
	k := s.key(client, msgID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[k]
	if !ok {
		return nil, "", false
	}
	if time.Since(e.Timestamp) > s.expire {
		return nil, "", false
	}
	return e.Content, e.ContentType, true
}

func (s *Store) cleanupLocked() {
	now := time.Now()
	i := 0
	for i < len(s.order) {
		k := s.order[i]
		e, ok := s.entries[k]
		if !ok || now.Sub(e.Timestamp) > s.expire {
			delete(s.entries, k)
			s.order = append(s.order[:i], s.order[i+1:]...)
			continue
		}
		i++
	}
}
