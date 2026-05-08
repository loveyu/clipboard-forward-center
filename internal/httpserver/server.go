package httpserver

import (
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

func (s *Server) handleClient(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/client/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	client, msgID := parts[0], parts[1]

	token := extractBearerToken(r)
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
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}
		contentType := r.Header.Get("Content-Type")
		s.store.Put(client, msgID, body, contentType)
		w.WriteHeader(http.StatusNoContent)

	case http.MethodGet:
		data, ct, ok := s.store.Get(client, msgID)
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		w.Write(data)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
