package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"pokerroom/internal/protocol"
)

func TestBucket(t *testing.T) {
	var b bucket
	now := time.Unix(0, 0)
	for i := range 3 {
		if !b.allow(now, 1, 3) {
			t.Fatalf("burst event %d refused", i)
		}
	}
	if b.allow(now, 1, 3) {
		t.Fatal("over the burst should be refused")
	}
	if !b.allow(now.Add(time.Second), 1, 3) {
		t.Fatal("a token should refill after a second")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/rooms", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("Fly-Client-IP", "203.0.113.9")
	if got := (&Server{}).clientIP(r); got != "10.0.0.5" {
		t.Fatalf("untrusted proxy header used: %s", got)
	}
	if got := (&Server{TrustProxy: true}).clientIP(r); got != "203.0.113.9" {
		t.Fatalf("trusted header ignored: %s", got)
	}
	r.RemoteAddr = "weird"
	if got := (&Server{}).clientIP(r); got != "weird" {
		t.Fatalf("got %s", got)
	}
}

func TestRoomCreationIsRateLimitedPerIP(t *testing.T) {
	srv := newTestServer(t)
	status := func() int {
		resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"Ann"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := range int(defaultRoomsPerMin) {
		if code := status(); code != http.StatusCreated {
			t.Fatalf("room %d: status %d", i, code)
		}
	}
	if code := status(); code != http.StatusTooManyRequests {
		t.Fatalf("over the limit: status %d", code)
	}
}

// A connection that floods messages is cut off; normal play never gets
// near the limit.
func TestFloodingConnectionIsClosed(t *testing.T) {
	srv := newTestServer(t)
	created := createRoom(t, srv, "Ann")
	c := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgResume, Token: created.Token})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	msg := []byte(`{"type":"sitOut"}`)
	var err error
	for range int(msgBurst) + maxDropped + 50 {
		if err = c.conn.Write(ctx, websocket.MessageText, msg); err != nil {
			break
		}
	}
	// Drain until the server closes the connection.
	for range c.msgs {
	}
	if code := websocket.CloseStatus(c.conn.Ping(ctx)); code != websocket.StatusPolicyViolation && code != -1 {
		t.Fatalf("close status %v", code)
	}
}
