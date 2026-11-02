// Package httpapi wires the HTTP routes: health checks, the REST API and,
// optionally, the built web app.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Config holds what the HTTP layer needs from the process environment.
type Config struct {
	// StaticDir is the directory of the built web app. Empty disables static serving.
	StaticDir string
	Version   string
}

// NewHandler returns the root handler for the server.
func NewHandler(cfg Config, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	started := time.Now()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"version": cfg.Version,
			"uptimeS": int(time.Since(started).Seconds()),
		})
	})

	if cfg.StaticDir != "" {
		mux.Handle("GET /", spaHandler(cfg.StaticDir))
	}

	return logRequests(log, mux)
}

// spaHandler serves files from dir and falls back to index.html so client-side
// routes like /r/AB12CD load the app.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur", time.Since(start))
	})
}
