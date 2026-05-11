package httpserver

import (
	"bytes"
	"encoding/json"
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
		Storage: config.StorageConfig{
			MaxBodySize:     100 * 1024 * 1024,
			FileStorageSize: 20 * 1024 * 1024,
		},
	}
}

func newTestStore() *store.Store {
	return store.New(100, 10*time.Minute, 100*1024*1024, 20*1024*1024)
}

func TestPutAndGet(t *testing.T) {
	s := newTestStore()
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("hello")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT status = %d, want %d", w.Code, http.StatusOK)
	}

	// Verify JSON response
	var resp putResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if resp.Length != 5 {
		t.Errorf("length = %d, want 5", resp.Length)
	}
	if resp.SHA256 == "" {
		t.Error("sha256 empty")
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

func TestQueryTokenAuth(t *testing.T) {
	s := newTestStore()
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1?token=TOKEN-A", bytes.NewReader([]byte("hello")))
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("PUT with query token: status = %d, want %d", w.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1?token=TOKEN-B", nil)
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("GET with query token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestContentTypePreserved(t *testing.T) {
	s := newTestStore()
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
	s := newTestStore()
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

func TestExtraHeadersRoundTrip(t *testing.T) {
	s := newTestStore()
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("data")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	req.Header.Set("X-Extra-Source", "phone")
	req.Header.Set("X-Extra-Tag", "clipboard")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Header().Get("X-Extra-Source") != "phone" {
		t.Errorf("X-Extra-Source = %q, want phone", w.Header().Get("X-Extra-Source"))
	}
	if w.Header().Get("X-Extra-Tag") != "clipboard" {
		t.Errorf("X-Extra-Tag = %q, want clipboard", w.Header().Get("X-Extra-Tag"))
	}
}

func TestContentLengthHeader(t *testing.T) {
	s := newTestStore()
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("hello")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)

	req = httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w = httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Header().Get("Content-Length") != "5" {
		t.Errorf("Content-Length = %q, want 5", w.Header().Get("Content-Length"))
	}
}

func TestPutClientMismatch(t *testing.T) {
	s := newTestStore()
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
	s := newTestStore()
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodGet, "/client/mobile-k50/msg1", nil)
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestInvalidToken(t *testing.T) {
	s := newTestStore()
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
	s := newTestStore()
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
	s := newTestStore()
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
	s := newTestStore()
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
	s := newTestStore()
	srv := New(testConfig(), s)

	req := httptest.NewRequest(http.MethodPost, "/client/mobile-k50/msg1", bytes.NewReader([]byte("post-data")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("POST status = %d, want %d", w.Code, http.StatusOK)
	}

	e, ok := s.Get("mobile-k50", "msg1")
	if !ok {
		t.Fatal("entry not found")
	}
	data, _ := e.ReadContent()
	if string(data) != "post-data" {
		t.Error("POST data mismatch")
	}
}

func TestBodyTooLarge(t *testing.T) {
	cfg := testConfig()
	cfg.Storage.MaxBodySize = 5
	s := store.New(100, 10*time.Minute, 5, 5)
	srv := New(cfg, s)

	req := httptest.NewRequest(http.MethodPut, "/client/mobile-k50/msg1", bytes.NewReader([]byte("toolarge")))
	req.Header.Set("Authorization", "Bearer TOKEN-A")
	w := httptest.NewRecorder()
	srv.handleClient(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}
