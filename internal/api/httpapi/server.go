package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"model-category-document-generator/internal/config"
	"model-category-document-generator/internal/domain"
	"model-category-document-generator/internal/usecase"
)

type Server struct {
	cfg       *config.Config
	generator *usecase.Generator
	logger    *slog.Logger
}

func NewServer(cfg *config.Config, generator *usecase.Generator, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, generator: generator, logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /api/bootstrap", s.bootstrap)
	mux.HandleFunc("GET /api/templates/", s.template)
	mux.HandleFunc("POST /api/generate", s.generate)
	mux.HandleFunc("GET /generated/", s.generated)
	mux.HandleFunc("/", s.static)
	return logMiddleware(s.logger, mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) bootstrap(w http.ResponseWriter, _ *http.Request) {
	response, err := s.generator.Bootstrap()
	if err != nil {
		s.logger.Error("bootstrap failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) template(w http.ResponseWriter, r *http.Request) {
	templateID := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	template, err := s.generator.Template(templateID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (s *Server) generate(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) generated(w http.ResponseWriter, r *http.Request) {
	fileName := filepath.Base(r.URL.Path)
	http.ServeFile(w, r, filepath.Join(s.cfg.OutputDir, fileName))
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
