package store

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type Entry struct {
	Content      []byte
	FilePath     string
	ContentType  string
	ExtraHeaders map[string]string
	Timestamp    time.Time
}

// ReadContent returns the entry body, reading from file if stored on disk.
func (e *Entry) ReadContent() ([]byte, error) {
	if e.FilePath != "" {
		return os.ReadFile(e.FilePath)
	}
	return e.Content, nil
}

type Store struct {
	mu              sync.RWMutex
	entries         map[string]*Entry
	order           []string
	maxRecords      int
	expire          time.Duration
	maxBodySize     int64
	fileStorageSize int64
}

func New(maxRecords int, expire time.Duration, maxBodySize, fileStorageSize int64) *Store {
	return &Store{
		entries:         make(map[string]*Entry),
		maxRecords:      maxRecords,
		expire:          expire,
		maxBodySize:     maxBodySize,
		fileStorageSize: fileStorageSize,
	}
}

func (s *Store) key(client, msgID string) string {
	return client + "/" + msgID
}

// Put stores a message. Returns an error if the body exceeds maxBodySize.
func (s *Store) Put(client, msgID string, content []byte, contentType string, extraHeaders map[string]string) error {
	if s.maxBodySize > 0 && int64(len(content)) > s.maxBodySize {
		return fmt.Errorf("body size %d exceeds limit %d", len(content), s.maxBodySize)
	}

	entry := &Entry{
		ContentType:  contentType,
		ExtraHeaders: extraHeaders,
		Timestamp:    time.Now(),
	}

	if s.fileStorageSize > 0 && int64(len(content)) > s.fileStorageSize {
		f, err := os.CreateTemp("", "cfc-store-*")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		if _, err := f.Write(content); err != nil {
			f.Close()
			os.Remove(f.Name())
			return fmt.Errorf("write temp file: %w", err)
		}
		f.Close()
		entry.FilePath = f.Name()
	} else {
		entry.Content = content
	}

	k := s.key(client, msgID)

	s.mu.Lock()
	filesToDelete := s.removeExpiredLocked()
	var evictedFile string
	if old, exists := s.entries[k]; exists {
		evictedFile = old.FilePath
	} else {
		if len(s.order) >= s.maxRecords {
			evict := s.order[0]
			if e, ok := s.entries[evict]; ok && e.FilePath != "" {
				evictedFile = e.FilePath
			}
			delete(s.entries, evict)
			s.order = s.order[1:]
		}
		s.order = append(s.order, k)
	}
	s.entries[k] = entry
	s.mu.Unlock()

	// Delete files outside the lock to avoid blocking concurrent operations.
	if evictedFile != "" {
		filesToDelete = append(filesToDelete, evictedFile)
	}
	removeFiles(filesToDelete)
	return nil
}

// Get retrieves an entry. Returns nil if not found or expired.
func (s *Store) Get(client, msgID string) (*Entry, bool) {
	k := s.key(client, msgID)
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[k]
	if !ok {
		return nil, false
	}
	if time.Since(e.Timestamp) > s.expire {
		return nil, false
	}
	return e, true
}

// StartCleanup runs a background goroutine that cleans up expired entries
// every minute until ctx is cancelled.
func (s *Store) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				before := len(s.order)
				files := s.removeExpiredLocked()
				removed := before - len(s.order)
				s.mu.Unlock()

				// Delete files outside the lock.
				removeFiles(files)
				if removed > 0 {
					log.Printf("store: cleanup removed %d expired entries", removed)
				}
			}
		}
	}()
}

// Close removes all remaining temporary files. Call on program shutdown.
func (s *Store) Close() {
	s.mu.Lock()
	var files []string
	for _, e := range s.entries {
		if e.FilePath != "" {
			files = append(files, e.FilePath)
		}
	}
	s.entries = make(map[string]*Entry)
	s.order = nil
	s.mu.Unlock()

	removeFiles(files)
	if len(files) > 0 {
		log.Printf("store: shutdown cleanup removed %d temp files", len(files))
	}
}

// removeExpiredLocked removes expired entries from the map and order slice,
// returning the file paths of any disk-backed entries for deletion by the caller.
// Must be called with s.mu held.
func (s *Store) removeExpiredLocked() []string {
	now := time.Now()
	var files []string
	i := 0
	for i < len(s.order) {
		k := s.order[i]
		e, ok := s.entries[k]
		if !ok || now.Sub(e.Timestamp) > s.expire {
			if ok && e.FilePath != "" {
				files = append(files, e.FilePath)
			}
			delete(s.entries, k)
			s.order = append(s.order[:i], s.order[i+1:]...)
			continue
		}
		i++
	}
	return files
}

// removeFiles deletes a list of files, logging errors for unexpected failures.
func removeFiles(files []string) {
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			log.Printf("store: remove temp file %s: %v", f, err)
		}
	}
}
