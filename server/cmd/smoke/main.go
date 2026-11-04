// Command smoke checks that a running server works end to end: health, the
// web app, creating a room, two players joining over WebSockets, and the
// game starting. It runs after every deploy and against a local build.
//
//	go run ./cmd/smoke -url https://poker-room-smruti.fly.dev -version abc123
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"pokerroom/internal/protocol"
)

func main() {
	base := flag.String("url", "http://localhost:8080", "server to check")
	version := flag.String("version", "", "if set, /healthz must report this version")
	wait := flag.Duration("wait", 2*time.Minute, "how long to keep retrying /healthz (a new deploy may still be starting)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *wait+time.Minute)
	defer cancel()
	if err := run(ctx, strings.TrimRight(*base, "/"), *version, *wait); err != nil {
		fmt.Fprintln(os.Stderr, "smoke test FAILED:", err)
		os.Exit(1)
	}
	fmt.Println("smoke test passed")
}

func run(ctx context.Context, base, version string, wait time.Duration) error {
	if err := waitHealthy(ctx, base, version, wait); err != nil {
		return err
	}
	step("health ok")

	page, err := get(ctx, base+"/")
	if err != nil {
		return fmt.Errorf("web app: %w", err)
	}
	if !strings.Contains(page, `id="root"`) {
		return errors.New("web app: / is not the app's index.html")
	}
	step("web app served")

	code, token, err := createRoom(ctx, base)
	if err != nil {
		return err
	}
	step("room " + code + " created")

	host, err := connect(ctx, base, code, protocol.ClientMsg{Type: protocol.MsgResume, Token: token})
	if err != nil {
		return fmt.Errorf("host: %w", err)
	}
	defer host.CloseNow()
	guest, err := connect(ctx, base, code, protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Smoke guest"})
	if err != nil {
		return fmt.Errorf("guest: %w", err)
	}
	defer guest.CloseNow()
	step("two players connected")

	if err := wsjson.Write(ctx, host, protocol.ClientMsg{Type: protocol.MsgStart}); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	if err := waitFor(ctx, guest, func(s *protocol.State) bool { return s.Phase == protocol.PhasePlaying }); err != nil {
		return fmt.Errorf("game did not start: %w", err)
	}
	step("game started")

	// Leave nothing half-played: the room is swept once it's been idle.
	_ = wsjson.Write(ctx, host, protocol.ClientMsg{Type: protocol.MsgEndGame})
	return nil
}

func step(s string) { fmt.Println("  ✓", s) }

func waitHealthy(ctx context.Context, base, version string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		var health struct {
			Status  string `json:"status"`
			Version string `json:"version"`
		}
		body, err := get(ctx, base+"/healthz")
		if err == nil {
			err = json.Unmarshal([]byte(body), &health)
		}
		switch {
		case err != nil:
		case health.Status != "ok":
			err = fmt.Errorf("status %q", health.Status)
		case version != "" && health.Version != version:
			err = fmt.Errorf("version %q, waiting for %q", health.Version, version)
		default:
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health: %w", err)
		}
		time.Sleep(3 * time.Second)
	}
}

func get(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %d", u, resp.StatusCode)
	}
	return string(b), nil
}

func createRoom(ctx context.Context, base string) (code, token string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/rooms", strings.NewReader(`{"name":"Smoke host"}`))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("create room: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("create room: status %d", resp.StatusCode)
	}
	var out struct{ Code, Token string }
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("create room: %w", err)
	}
	return out.Code, out.Token, nil
}

// connect opens a player's WebSocket, says hello, and waits for the welcome.
func connect(ctx context.Context, base, code string, hello protocol.ClientMsg) (*websocket.Conn, error) {
	u, err := url.Parse(base + "/ws/" + code)
	if err != nil {
		return nil, err
	}
	u.Scheme = map[string]string{"http": "ws", "https": "wss"}[u.Scheme]
	conn, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{
		// Browsers send an Origin; the server only accepts its own.
		HTTPHeader: http.Header{"Origin": {base}},
	})
	if err != nil {
		return nil, err
	}
	if err := wsjson.Write(ctx, conn, hello); err != nil {
		conn.CloseNow()
		return nil, err
	}
	for {
		m, err := read(ctx, conn)
		if err != nil {
			conn.CloseNow()
			return nil, err
		}
		if m.Type == protocol.MsgWelcome {
			return conn, nil
		}
	}
}

func waitFor(ctx context.Context, conn *websocket.Conn, ok func(*protocol.State) bool) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		m, err := read(ctx, conn)
		if err != nil {
			return err
		}
		if m.State != nil && ok(m.State) {
			return nil
		}
	}
}

func read(ctx context.Context, conn *websocket.Conn) (protocol.ServerMsg, error) {
	var m protocol.ServerMsg
	if err := wsjson.Read(ctx, conn, &m); err != nil {
		return m, err
	}
	if m.Type == protocol.MsgError {
		return m, errors.New(m.Error)
	}
	return m, nil
}
