package httpserver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"clipboard-forward-center/internal/config"
	"clipboard-forward-center/internal/store"
)

type Server struct {
	cfg   *config.Config
	store *store.Store
}

func New(cfg *config.Config, s *store.Store) *Server {
	return &Server{cfg: cfg, store: s}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/client/", s.handleClient)
	return mux
}

func (s *Server) Start(addr string) error {
	log.Printf("http: listening on %s", addr)
	return http.ListenAndServe(addr, s.Handler())
}

type putResponse struct {
	Length int64  `json:"length"`
	SHA256 string `json:"sha256"`
}

func (s *Server) handleClient(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/client/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	client, msgID := parts[0], parts[1]

	token := extractToken(r)
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c := s.cfg.FindClientByToken(token)
	if c == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodPut, http.MethodPost:
		if c.Name != client {
			http.Error(w, "forbidden: client mismatch", http.StatusForbidden)
			return
		}

		maxBody := s.cfg.Storage.MaxBodySize
		if maxBody <= 0 {
			maxBody = 100 * 1024 * 1024
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		if int64(len(body)) > maxBody {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		contentType := r.Header.Get("Content-Type")
		extraHeaders := extractExtraHeaders(r)

		if err := s.store.Put(client, msgID, body, contentType, extraHeaders); err != nil {
			http.Error(w, "store failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		sum := sha256.Sum256(body)
		resp := putResponse{
			Length: int64(len(body)),
			SHA256: fmt.Sprintf("%x", sum),
		}
		respData, _ := json.Marshal(resp)

		w.Header().Set("Content-Type", "application/json")
		for k, v := range extraHeaders {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respData)))
		w.WriteHeader(http.StatusOK)
		w.Write(respData)

	case http.MethodGet:
		entry, ok := s.store.Get(client, msgID)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		data, err := entry.ReadContent()
		if err != nil {
			http.Error(w, "read content failed", http.StatusInternalServerError)
			return
		}

		ct := entry.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		for k, v := range entry.ExtraHeaders {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.WriteHeader(http.StatusOK)
		w.Write(data)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// extractToken returns the Bearer token from Authorization header or the "token" query param.
func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// extractExtraHeaders collects all request headers with the X-Extra- prefix (case-insensitive).
func extractExtraHeaders(r *http.Request) map[string]string {
	extra := make(map[string]string)
	for k, vs := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-extra-") && len(vs) > 0 {
			extra[k] = vs[0]
		}
	}
	return extra
}
