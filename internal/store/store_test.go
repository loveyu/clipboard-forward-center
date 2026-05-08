package store

import (
	"testing"
	"time"
)

func TestPutAndGet(t *testing.T) {
	s := New(100, 10*time.Minute)

	s.Put("client1", "msg1", []byte("hello"), "")
	data, _, ok := s.Get("client1", "msg1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if string(data) != "hello" {
		t.Errorf("data = %q, want hello", data)
	}
}

func TestPutAndGetWithContentType(t *testing.T) {
	s := New(100, 10*time.Minute)

	s.Put("client1", "msg1", []byte("hello"), "text/plain")
	data, ct, ok := s.Get("client1", "msg1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if string(data) != "hello" {
		t.Errorf("data = %q, want hello", data)
	}
	if ct != "text/plain" {
		t.Errorf("contentType = %q, want text/plain", ct)
	}
}

func TestGetExpired(t *testing.T) {
	s := New(100, 50*time.Millisecond)

	s.Put("client1", "msg1", []byte("hello"), "")
	time.Sleep(100 * time.Millisecond)

	_, _, ok := s.Get("client1", "msg1")
	if ok {
		t.Error("expected expired entry to be missing")
	}
}

func TestGetNotFound(t *testing.T) {
	s := New(100, 10*time.Minute)

	_, _, ok := s.Get("nonexistent", "msg")
	if ok {
		t.Error("expected not found")
	}
}

func TestMaxRecords(t *testing.T) {
	s := New(3, 10*time.Minute)

	s.Put("c", "1", []byte("a"), "")
	s.Put("c", "2", []byte("b"), "")
	s.Put("c", "3", []byte("c"), "")
	s.Put("c", "4", []byte("d"), "")

	_, _, ok := s.Get("c", "1")
	if ok {
		t.Error("oldest entry should be evicted")
	}
	_, _, ok = s.Get("c", "4")
	if !ok {
		t.Error("newest entry should exist")
	}
}

func TestUpdateExisting(t *testing.T) {
	s := New(100, 10*time.Minute)

	s.Put("c", "1", []byte("old"), "text/plain")
	s.Put("c", "1", []byte("new"), "image/png")

	data, ct, ok := s.Get("c", "1")
	if !ok {
		t.Fatal("expected entry")
	}
	if string(data) != "new" {
		t.Errorf("data = %q, want new", data)
	}
	if ct != "image/png" {
		t.Errorf("contentType = %q, want image/png", ct)
	}
}

func TestDifferentClients(t *testing.T) {
	s := New(100, 10*time.Minute)

	s.Put("c1", "msg", []byte("from-c1"), "")
	s.Put("c2", "msg", []byte("from-c2"), "")

	data1, _, ok := s.Get("c1", "msg")
	if !ok || string(data1) != "from-c1" {
		t.Error("c1/msg mismatch")
	}

	data2, _, ok := s.Get("c2", "msg")
	if !ok || string(data2) != "from-c2" {
		t.Error("c2/msg mismatch")
	}
}
