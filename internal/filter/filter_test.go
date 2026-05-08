package filter

import (
	"testing"
	"time"
)

func TestComputeHash(t *testing.T) {
	h1 := ComputeHash("text", "hello")
	h2 := ComputeHash("text", "hello")
	h3 := ComputeHash("text", "world")
	h4 := ComputeHash("image", "hello")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if h1 == h4 {
		t.Error("different type should produce different hash")
	}
}

func TestShouldForward(t *testing.T) {
	f := New(5 * time.Second)

	hash := ComputeHash("text", "hello")

	if !f.ShouldForward("clientA", hash) {
		t.Error("should forward when no history")
	}

	f.Record("clientA", hash)

	if f.ShouldForward("clientA", hash) {
		t.Error("should not forward duplicate")
	}

	if !f.ShouldForward("clientB", hash) {
		t.Error("different client should forward")
	}
}

func TestFilterExpiry(t *testing.T) {
	f := New(50 * time.Millisecond)

	hash := ComputeHash("text", "hello")
	f.Record("clientA", hash)

	if f.ShouldForward("clientA", hash) {
		t.Error("should be filtered")
	}

	time.Sleep(100 * time.Millisecond)

	if !f.ShouldForward("clientA", hash) {
		t.Error("should not be filtered after expiry")
	}
}
