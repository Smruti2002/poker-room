// Command server runs the Poker Room HTTP + WebSocket server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"pokerroom/internal/httpapi"
	"pokerroom/internal/room"
	"pokerroom/internal/ws"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	addr := ":" + envOr("PORT", "8080")
	maxAge := envDuration(log, "ROOM_MAX_AGE", 12*time.Hour)
	manager := room.NewManager(room.Options{MaxAge: maxAge}, log)
	rooms := &ws.Server{
		Rooms:          manager,
		OriginPatterns: splitList(os.Getenv("ALLOWED_ORIGINS")),
		Log:            log,
		TrustProxy:     os.Getenv("TRUST_PROXY") == "1",
		RoomsPerMinute: envFloat(log, "ROOMS_PER_MINUTE", 10),
	}
	srv := &http.Server{
		Addr: addr,
		Handler: httpapi.NewHandler(httpapi.Config{
			StaticDir: os.Getenv("STATIC_DIR"),
			Version:   version,
			Routes:    rooms.Register,
			Stats:     func() any { return manager.Stats() },
		}, log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Rooms survive restarts and redeploys when SNAPSHOT_PATH is set: they
	// are loaded at start, saved every 30s, and saved again on shutdown.
	snapshot := os.Getenv("SNAPSHOT_PATH")
	if snapshot != "" {
		n, err := manager.LoadAll(snapshot)
		if err != nil {
			return fmt.Errorf("loading rooms: %w", err)
		}
		log.Info("rooms restored", "count", n, "path", snapshot)
	}
	idle := envDuration(log, "ROOM_IDLE", 12*time.Hour)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go housekeeping(ctx, log, manager, snapshot, idle)

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr, "version", version)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	if snapshot != "" {
		if err := manager.SaveAll(snapshot); err != nil {
			return fmt.Errorf("saving rooms: %w", err)
		}
		log.Info("rooms saved", "path", snapshot)
	}
	return nil
}

// housekeeping saves rooms regularly (so a crash loses at most a few
// seconds), closes rooms nobody has used for a long time, and closes every
// room once it reaches its maximum age.
func housekeeping(ctx context.Context, log *slog.Logger, m *room.Manager, snapshot string, idle time.Duration) {
	save := time.NewTicker(30 * time.Second)
	defer save.Stop()
	sweep := time.NewTicker(5 * time.Minute)
	defer sweep.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-save.C:
			if snapshot != "" {
				if err := m.SaveAll(snapshot); err != nil {
					log.Error("saving rooms", "err", err)
				}
			}
		case <-sweep.C:
			if n := m.Sweep(idle); n > 0 {
				log.Info("rooms closed", "count", n)
			}
		}
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// splitList parses a comma-separated env var.
func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envFloat(log *slog.Logger, key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		log.Warn("ignoring invalid setting", "key", key, "value", v)
		return fallback
	}
	return f
}

func envDuration(log *slog.Logger, key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Warn("ignoring invalid setting", "key", key, "value", v)
		return fallback
	}
	return d
}
