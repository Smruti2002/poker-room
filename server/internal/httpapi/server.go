// Package httpapi wires the HTTP routes: health checks, the REST API and,
// optionally, the built web app.
package httpapi

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds what the HTTP layer needs from the process environment.
type Config struct {
	// StaticDir is the directory of the built web app. Empty disables static serving.
	StaticDir string
	Version   string
	// Routes registers extra handlers, such as the room API.
	Routes func(*http.ServeMux)
	// Stats, if set, adds load figures (rooms, connections) to /healthz.
	Stats func() any
}

// NewHandler returns the root handler for the server.
func NewHandler(cfg Config, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	started := time.Now()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		body := map[string]any{
			"status":  "ok",
			"version": cfg.Version,
			"uptimeS": int(time.Since(started).Seconds()),
		}
		if cfg.Stats != nil {
			body["stats"] = cfg.Stats()
		}
		writeJSON(w, http.StatusOK, body)
	})

	if cfg.Routes != nil {
		cfg.Routes(mux)
	}

	if cfg.StaticDir != "" {
		mux.Handle("GET /", spaHandler(cfg.StaticDir))
	}

	return logRequests(log, mux)
}

// spaHandler serves files from dir and falls back to index.html so client-side
// routes like /r/AB12CD load the app.
//
// Files under /assets/ have content hashes in their names, so browsers may
// keep them forever. Everything else, index.html above all, is revalidated
// on every load, so a redeploy never leaves a phone with a page that points
// at assets that no longer exist.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
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

// Hijack lets WebSocket upgrades through the logging wrapper.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	s.status = http.StatusSwitchingProtocols
	return http.NewResponseController(s.ResponseWriter).Hijack()
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", rec.status, "dur", time.Since(start))
	})
}
