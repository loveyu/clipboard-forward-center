package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func newTestStore(maxRecords int, expire time.Duration) *Store {
	return New(maxRecords, expire, 100*1024*1024, 20*1024*1024)
}

func TestPutAndGet(t *testing.T) {
	s := newTestStore(100, 10*time.Minute)

	if err := s.Put("client1", "msg1", []byte("hello"), "", nil); err != nil {
		t.Fatal(err)
	}
	e, ok := s.Get("client1", "msg1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	data, err := e.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("data = %q, want hello", data)
	}
}

func TestPutAndGetWithContentType(t *testing.T) {
	s := newTestStore(100, 10*time.Minute)

	if err := s.Put("client1", "msg1", []byte("hello"), "text/plain", nil); err != nil {
		t.Fatal(err)
	}
	e, ok := s.Get("client1", "msg1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	data, err := e.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("data = %q, want hello", data)
	}
	if e.ContentType != "text/plain" {
		t.Errorf("contentType = %q, want text/plain", e.ContentType)
	}
}

func TestGetExpired(t *testing.T) {
	s := newTestStore(100, 50*time.Millisecond)

	s.Put("client1", "msg1", []byte("hello"), "", nil)
	time.Sleep(100 * time.Millisecond)

	_, ok := s.Get("client1", "msg1")
	if ok {
		t.Error("expected expired entry to be missing")
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(100, 10*time.Minute)

	_, ok := s.Get("nonexistent", "msg")
	if ok {
		t.Error("expected not found")
	}
}

func TestMaxRecords(t *testing.T) {
	s := newTestStore(3, 10*time.Minute)

	s.Put("c", "1", []byte("a"), "", nil)
	s.Put("c", "2", []byte("b"), "", nil)
	s.Put("c", "3", []byte("c"), "", nil)
	s.Put("c", "4", []byte("d"), "", nil)

	_, ok := s.Get("c", "1")
	if ok {
		t.Error("oldest entry should be evicted")
	}
	_, ok = s.Get("c", "4")
	if !ok {
		t.Error("newest entry should exist")
	}
}

func TestUpdateExisting(t *testing.T) {
	s := newTestStore(100, 10*time.Minute)

	s.Put("c", "1", []byte("old"), "text/plain", nil)
	s.Put("c", "1", []byte("new"), "image/png", nil)

	e, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry")
	}
	data, err := e.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("data = %q, want new", data)
	}
	if e.ContentType != "image/png" {
		t.Errorf("contentType = %q, want image/png", e.ContentType)
	}
}

func TestDifferentClients(t *testing.T) {
	s := newTestStore(100, 10*time.Minute)

	s.Put("c1", "msg", []byte("from-c1"), "", nil)
	s.Put("c2", "msg", []byte("from-c2"), "", nil)

	e1, ok := s.Get("c1", "msg")
	if !ok {
		t.Fatal("c1/msg not found")
	}
	data1, _ := e1.ReadContent()
	if string(data1) != "from-c1" {
		t.Error("c1/msg mismatch")
	}

	e2, ok := s.Get("c2", "msg")
	if !ok {
		t.Fatal("c2/msg not found")
	}
	data2, _ := e2.ReadContent()
	if string(data2) != "from-c2" {
		t.Error("c2/msg mismatch")
	}
}

func TestMaxBodySize(t *testing.T) {
	s := New(100, 10*time.Minute, 10, 5)

	err := s.Put("c", "1", []byte("12345678901"), "", nil)
	if err == nil {
		t.Error("expected error for oversized body")
	}
}

func TestFileStorage(t *testing.T) {
	s := New(100, 10*time.Minute, 100, 5)

	data := []byte("1234567890") // 10 bytes > fileStorageSize(5)
	if err := s.Put("c", "1", data, "text/plain", nil); err != nil {
		t.Fatal(err)
	}

	e, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry")
	}
	if e.FilePath == "" {
		t.Error("expected file storage, got in-memory")
	}
	got, err := e.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("data = %q, want %q", got, data)
	}
}

func TestTempFileDeletedOnOverwrite(t *testing.T) {
	// fileStorageSize=5, so 10-byte body goes to disk
	s := New(100, 10*time.Minute, 100, 5)

	large := []byte("1234567890") // 10 bytes → stored on disk
	if err := s.Put("c", "1", large, "", nil); err != nil {
		t.Fatal(err)
	}
	e, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry")
	}
	tmpPath := e.FilePath
	if tmpPath == "" {
		t.Fatal("expected file storage")
	}
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatalf("temp file should exist: %v", err)
	}

	// Overwrite with a small body that fits in memory
	small := []byte("hi") // 2 bytes → stored in memory
	if err := s.Put("c", "1", small, "", nil); err != nil {
		t.Fatal(err)
	}

	// Temp file must be deleted
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file should have been deleted after overwrite")
	}

	// New entry should be in memory
	e2, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry after overwrite")
	}
	if e2.FilePath != "" {
		t.Error("expected in-memory storage after overwrite with small body")
	}
	data, _ := e2.ReadContent()
	if string(data) != "hi" {
		t.Errorf("data = %q, want hi", data)
	}
}

func TestPeriodicCleanupDeletesTempFiles(t *testing.T) {
	// expire=50ms so entries expire quickly
	s := New(100, 50*time.Millisecond, 100, 5)

	large := []byte("1234567890") // 10 bytes → stored on disk
	if err := s.Put("c", "1", large, "", nil); err != nil {
		t.Fatal(err)
	}
	e, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry before expiry")
	}
	tmpPath := e.FilePath
	if tmpPath == "" {
		t.Fatal("expected file storage")
	}

	// Wait for entry to expire, then trigger cleanup manually
	time.Sleep(100 * time.Millisecond)
	s.mu.Lock()
	files := s.removeExpiredLocked()
	s.mu.Unlock()
	removeFiles(files)

	// Temp file must be deleted by cleanup
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file should have been deleted by cleanup")
	}

	// Entry must no longer be accessible
	_, ok = s.Get("c", "1")
	if ok {
		t.Error("expired entry should not be found")
	}
}

func TestCloseDeletesAllTempFiles(t *testing.T) {
	s := New(100, 10*time.Minute, 100, 5)

	paths := make([]string, 3)
	for i := range paths {
		large := []byte("1234567890") // 10 bytes → stored on disk
		key := fmt.Sprintf("%d", i)
		if err := s.Put("c", key, large, "", nil); err != nil {
			t.Fatal(err)
		}
		e, ok := s.Get("c", key)
		if !ok {
			t.Fatal("expected entry")
		}
		paths[i] = e.FilePath
		if paths[i] == "" {
			t.Fatalf("entry %d: expected file storage", i)
		}
	}

	s.Close()

	for _, p := range paths {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("temp file %s should have been deleted by Close()", p)
		}
	}
}


func TestStartCleanupStopsOnContextCancel(t *testing.T) {
	s := New(100, 50*time.Millisecond, 100, 5)
	ctx, cancel := context.WithCancel(context.Background())
	s.StartCleanup(ctx)
	cancel() // should not block or panic
	time.Sleep(10 * time.Millisecond)
}

func TestExtraHeaders(t *testing.T) {
	s := newTestStore(100, 10*time.Minute)

	extra := map[string]string{"X-Extra-Foo": "bar", "X-Extra-Baz": "qux"}
	s.Put("c", "1", []byte("data"), "", extra)

	e, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry")
	}
	if e.ExtraHeaders["X-Extra-Foo"] != "bar" {
		t.Errorf("X-Extra-Foo = %q, want bar", e.ExtraHeaders["X-Extra-Foo"])
	}
	if e.ExtraHeaders["X-Extra-Baz"] != "qux" {
		t.Errorf("X-Extra-Baz = %q, want qux", e.ExtraHeaders["X-Extra-Baz"])
	}
}
