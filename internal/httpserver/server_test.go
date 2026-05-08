package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/store"
)

func testConfig() *config.Config {
	return &config.Config{
		Clients: []config.Client{
			{Name: "mobile-k50", Token: "TOKEN-A"},
			{Name: "work-debian", Token: "TOKEN-B"},
		},
	}
}

func TestPutAndGet(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("hello")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("PUT status = %d, want %d", w.Code, http.StatusNoContent)
	}

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-B")
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "hello" {
		t.Errorf("GET body = %q, want hello", w.Body.String())
	}
}

func TestContentTypePreserved(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/img1", bytes.NewReader([]byte("imagedata")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	req.Header.Set("Content-Type", "image/png")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/img1", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}

func TestDefaultContentType(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("data")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if ct := w.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}
}

func TestPutClientMismatch(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/work-debian/msg1", bytes.NewReader([]byte("data")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestNoAuth(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInvalidToken(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	req.Header.Set("Authorization", "Bearer INVALID")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestGetNotFound(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodGet, "/client/mobile-k50/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetPublicAuth(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("data")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-B")
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET with different token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestInvalidPath(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodGet, "/client/onlyonepart", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestPostMethod(t *testing.T) {
	s := store.New(100, 10*time.Minute)
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPost, "/client/mobile-k50/msg1", bytes.NewReader([]byte("post-data")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("POST status = %d, want %d", w.Code, http.StatusNoContent)
	}

	data, _, ok := s.Get("mobile-k50", "msg1")
	if !ok || string(data) != "post-data" {
		t.Error("POST data mismatch")
	}
}
