package httpapi

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"model-category-document-generator/internal/config"
	"model-category-document-generator/internal/domain"
	"model-category-document-generator/internal/usecase"
)

type Server struct {
	cfg       *config.Config
	generator *usecase.Generator
	logger    *slog.Logger
	limiter   *rateLimiter
}

func NewServer(cfg *config.Config, generator *usecase.Generator, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, generator: generator, logger: logger, limiter: newRateLimiter(30, time.Minute)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/document-templates", s.documentTemplates)
	mux.HandleFunc("GET /api/document-templates/", s.documentTemplate)
	mux.HandleFunc("POST /api/documents", s.createDocument)
	mux.HandleFunc("GET /api/documents/", s.documentAction)
	mux.HandleFunc("/", s.static)
	return logMiddleware(s.logger, mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) documentTemplates(w http.ResponseWriter, _ *http.Request) {
	response, err := s.generator.Bootstrap()
	if err != nil {
		s.logger.Error("document templates failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) documentTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := strings.TrimPrefix(r.URL.Path, "/api/document-templates/")
	template, err := s.generator.Template(templateID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (s *Server) createDocument(w http.ResponseWriter, r *http.Request) {
	if !s.limiter.allow(clientIP(r)) {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{"ok": false, "error": "too many requests"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	var request domain.GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "invalid json"})
		return
	}

	response, status, err := s.generator.Generate(request)
	if err != nil {
		s.logger.Error("generate failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, status, response)
}

func (s *Server) documentAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/documents/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	documentID := filepath.Base(parts[0])
	action := parts[1]
	switch action {
	case "status":
		s.documentStatus(w, documentID)
	case "download":
		s.documentDownload(w, r, documentID, false)
	case "preview":
		s.documentDownload(w, r, documentID, true)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) documentStatus(w http.ResponseWriter, documentID string) {
	if _, ok := s.findDocumentFile(documentID, false); ok {
		writeJSON(w, http.StatusOK, map[string]string{"document_id": documentID, "status": "ready"})
		return
	}
	if _, ok := s.findDocumentFile(documentID, true); ok {
		writeJSON(w, http.StatusOK, map[string]string{"document_id": documentID, "status": "processing"})
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"document_id": documentID, "status": "expired"})
}

func (s *Server) documentDownload(w http.ResponseWriter, r *http.Request, documentID string, htmlPreview bool) {
	filePath, ok := s.findDocumentFile(documentID, htmlPreview)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"ok": false, "error": "document not found"})
		return
	}
	http.ServeFile(w, r, filePath)
}

func (s *Server) findDocumentFile(documentID string, htmlPreview bool) (string, bool) {
	if documentID != filepath.Base(documentID) {
		return "", false
	}
	if htmlPreview {
		filePath := filepath.Join(s.cfg.OutputDir, documentID+".html")
		if _, err := os.Stat(filePath); err == nil {
			return filePath, true
		}
		return "", false
	}
	for _, extension := range []string{".pdf", ".docx", ".txt"} {
		filePath := filepath.Join(s.cfg.OutputDir, documentID+extension)
		if _, err := os.Stat(filePath); err == nil {
			return filePath, true
		}
	}
	return "", false
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		http.ServeFile(w, r, filepath.Join(s.cfg.PublicDir, "index.html"))
		return
	}

	filePath := filepath.Join(s.cfg.PublicDir, filepath.Clean(strings.TrimPrefix(path, "/")))
	if _, err := os.Stat(filePath); err == nil {
		http.ServeFile(w, r, filePath)
		return
	}

	http.NotFound(w, r)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func logMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
	})
}

type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, requests: map[string][]time.Time{}}
}

func (l *rateLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	entries := l.requests[key]
	kept := entries[:0]
	for _, entry := range entries {
		if entry.After(cutoff) {
			kept = append(kept, entry)
		}
	}
	if len(kept) >= l.limit {
		l.requests[key] = kept
		return false
	}
	l.requests[key] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if comma := strings.Index(value, ","); comma >= 0 {
			value = strings.TrimSpace(value[:comma])
		}
		if parsed := net.ParseIP(value); parsed != nil {
			return parsed.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
