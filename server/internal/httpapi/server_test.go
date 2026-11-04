package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestServer(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(NewHandler(cfg, log))
	t.Cleanup(srv.Close)
	return srv
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t, Config{Version: "test"})

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["version"] != "test" {
		t.Fatalf("unexpected body: %v", body)
	}
}

func TestStaticServingWithSPAFallback(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<html>app</html>")
	mustWrite(t, filepath.Join(dir, "app.js"), "console.log(1)")
	srv := newTestServer(t, Config{StaticDir: dir})

	tests := []struct {
		path, want string
	}{
		{"/", "<html>app</html>"},
		{"/app.js", "console.log(1)"},
		{"/r/AB12CD", "<html>app</html>"},         // client-side route
		{"/../../etc/passwd", "<html>app</html>"}, // no escaping the dir
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tt.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			got, _ := io.ReadAll(resp.Body)
			if string(got) != tt.want {
				t.Fatalf("GET %s = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCacheHeaders(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<html>app</html>")
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "assets", "index-abc123.js"), "console.log(1)")
	srv := newTestServer(t, Config{StaticDir: dir})

	for path, want := range map[string]string{
		"/":                       "no-cache",
		"/r/AB12CD":               "no-cache",
		"/assets/index-abc123.js": "public, max-age=31536000, immutable",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if got := resp.Header.Get("Cache-Control"); got != want {
			t.Errorf("GET %s: Cache-Control = %q, want %q", path, got, want)
		}
	}
}

func TestNoStaticDirMeans404(t *testing.T) {
	srv := newTestServer(t, Config{})
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
