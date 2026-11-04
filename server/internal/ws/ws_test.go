package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"pokerroom/internal/engine"
	"pokerroom/internal/httpapi"
	"pokerroom/internal/protocol"
	"pokerroom/internal/room"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Rooms: room.NewManager(room.Options{
		Variants: map[string]engine.Variant{"holdem": engine.Holdem},
	}, log), Log: log}
	srv := httptest.NewServer(httpapi.NewHandler(httpapi.Config{Routes: s.Register}, log))
	t.Cleanup(srv.Close)
	return srv
}

func createRoom(t *testing.T, srv *httptest.Server, name string) createResp {
	t.Helper()
	resp, err := http.Post(srv.URL+"/api/rooms", "application/json", strings.NewReader(`{"name":"`+name+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create room: status %d", resp.StatusCode)
	}
	var out createResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

// client is a test player with a background reader.
type client struct {
	t      *testing.T
	conn   *websocket.Conn
	msgs   chan protocol.ServerMsg
	id     string
	token  string
	name   string
	states []protocol.State // every state received, for leak checks
}

func wsURL(srv *httptest.Server, code string) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/" + code
}

func dial(t *testing.T, srv *httptest.Server, code string, hello protocol.ClientMsg) *client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL(srv, code), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{t: t, conn: conn, msgs: make(chan protocol.ServerMsg, 256), name: hello.Name}
	t.Cleanup(func() { conn.CloseNow() })
	go func() {
		defer close(c.msgs)
		for {
			var m protocol.ServerMsg
			if err := wsjson.Read(context.Background(), conn, &m); err != nil {
				return
			}
			c.msgs <- m
		}
	}()
	if err := wsjson.Write(ctx, conn, hello); err != nil {
		t.Fatal(err)
	}
	w := c.next()
	if w.Type != protocol.MsgWelcome {
		t.Fatalf("expected welcome, got %+v", w)
	}
	c.id, c.token = w.PlayerID, w.Token
	return c
}

// next returns the next message, failing after a timeout.
func (c *client) next() protocol.ServerMsg {
	c.t.Helper()
	select {
	case m, ok := <-c.msgs:
		if !ok {
			c.t.Fatal("connection closed")
		}
		if m.State != nil {
			c.states = append(c.states, *m.State)
		}
		return m
	case <-time.After(5 * time.Second):
		c.t.Fatal("timed out waiting for a message")
	}
	return protocol.ServerMsg{}
}

// waitState reads until a state satisfies pred.
func (c *client) waitState(pred func(*protocol.State) bool) *protocol.State {
	c.t.Helper()
	for {
		m := c.next()
		if m.Type == protocol.MsgError {
			c.t.Fatalf("%s got error: %s", c.name, m.Error)
		}
		if m.State != nil && pred(m.State) {
			return m.State
		}
	}
}

// latest drains queued messages and returns the newest state, if any.
func (c *client) latest() *protocol.State {
	var s *protocol.State
	for {
		select {
		case m, ok := <-c.msgs:
			if !ok {
				return s
			}
			if m.Type == protocol.MsgError {
				c.t.Fatalf("%s got error: %s", c.name, m.Error)
			}
			if m.State != nil {
				c.states = append(c.states, *m.State)
				s = m.State
			}
		default:
			return s
		}
	}
}

func (c *client) send(m protocol.ClientMsg) {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, c.conn, m); err != nil {
		c.t.Fatal(err)
	}
}

// assertNoLeaks checks every state a client received: other players' hole
// cards appear only once shown at showdown, and the log never mentions
// another player's deal.
func assertNoLeaks(t *testing.T, c *client) {
	t.Helper()
	for _, s := range c.states {
		for _, p := range s.Players {
			if p.ID != c.id && p.Hole != nil && (p.HandDesc == "" || s.Hand == nil || !s.Hand.Done) {
				t.Fatalf("%s saw %s's hole cards %v (stage %s)", c.name, p.Name, p.Hole, s.Stage)
			}
		}
		for _, line := range s.Log {
			if strings.Contains(line, " is dealt ") && !strings.HasPrefix(line, c.name+" is dealt") {
				t.Fatalf("%s's log leaks %q", c.name, line)
			}
		}
	}
}

func TestPlayAHandOverWebSockets(t *testing.T) {
	srv := newTestServer(t)
	created := createRoom(t, srv, "Ann")

	ann := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgResume, Token: created.Token})
	ann.name = "Ann"
	bob := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Bob"})
	cy := dial(t, srv, strings.ToLower(created.Code), protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Cy"})
	clients := []*client{ann, bob, cy}

	ann.waitState(func(s *protocol.State) bool { return len(s.Players) == 3 })
	if s := ann.latest(); s != nil && s.HostID != ann.id {
		t.Fatal("creator should be host")
	}
	ann.send(protocol.ClientMsg{Type: protocol.MsgStart})

	// Everyone checks or calls until the first hand is over.
	deadline := time.Now().Add(10 * time.Second)
	var final *protocol.State
	for final == nil {
		if time.Now().After(deadline) {
			t.Fatal("hand did not finish")
		}
		for _, c := range clients {
			s := c.latest()
			if s == nil || s.Hand == nil {
				continue
			}
			if s.Hand.Done {
				final = s
				break
			}
			if l := s.Hand.Legal; l != nil {
				a := &protocol.Action{Type: "call"}
				if l.Check {
					a.Type = "check"
				}
				c.send(protocol.ClientMsg{Type: protocol.MsgAction, Action: a})
			}
		}
		time.Sleep(5 * time.Millisecond)
	}

	var total int64
	for _, p := range final.Players {
		total += p.Stack
	}
	if total != 6000 {
		t.Fatalf("chips after hand = %d, want 6000", total)
	}
	if final.Hand.Number != 1 || len(final.Hand.Boards[0]) != 5 {
		t.Fatalf("unexpected final hand %+v", final.Hand)
	}

	// Nothing moves on by itself: any player starts the next hand.
	bob.send(protocol.ClientMsg{Type: protocol.MsgNextHand})
	bob.waitState(func(s *protocol.State) bool { return s.Hand != nil && s.Hand.Number == 2 })

	for _, c := range clients {
		c.latest()
		if len(c.states) == 0 {
			t.Fatalf("%s received no states", c.name)
		}
		assertNoLeaks(t, c)
	}
}

func TestReconnectKeepsSeatAndCards(t *testing.T) {
	srv := newTestServer(t)
	created := createRoom(t, srv, "Ann")
	ann := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgResume, Token: created.Token})
	bob := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Bob"})
	ann.send(protocol.ClientMsg{Type: protocol.MsgStart})

	before := bob.waitState(func(s *protocol.State) bool { return s.Hand != nil })
	hole := func(s *protocol.State, id string) []string {
		for _, p := range s.Players {
			if p.ID == id {
				return p.Hole
			}
		}
		return nil
	}
	cards := hole(before, bob.id)
	if len(cards) != 2 {
		t.Fatalf("bob's cards = %v", cards)
	}

	bob.conn.Close(websocket.StatusNormalClosure, "bye")
	ann.waitState(func(s *protocol.State) bool {
		return slices.ContainsFunc(s.Players, func(p protocol.PlayerView) bool { return p.ID == bob.id && !p.Connected })
	})

	again := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgResume, Token: bob.token})
	if again.id != bob.id {
		t.Fatalf("resumed as %s, want %s", again.id, bob.id)
	}
	s := again.waitState(func(*protocol.State) bool { return true })
	if !slices.Equal(hole(s, bob.id), cards) {
		t.Fatalf("cards after reconnect %v, want %v", hole(s, bob.id), cards)
	}
}

func TestBadHandshakes(t *testing.T) {
	srv := newTestServer(t)
	created := createRoom(t, srv, "Ann")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Unknown room: plain 404, no upgrade.
	if _, resp, err := websocket.Dial(ctx, wsURL(srv, "NOPE42"), nil); err == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown room: err %v", err)
	}

	// Foreign origin is refused.
	_, resp, err := websocket.Dial(ctx, wsURL(srv, created.Code), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"https://evil.example"}},
	})
	if err == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin: err %v", err)
	}

	expectError := func(hello any) {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, wsURL(srv, created.Code), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.CloseNow()
		if err := wsjson.Write(ctx, conn, hello); err != nil {
			t.Fatal(err)
		}
		var m protocol.ServerMsg
		if err := wsjson.Read(ctx, conn, &m); err == nil && m.Type != protocol.MsgError {
			t.Fatalf("hello %v: got %+v, want error", hello, m)
		}
	}
	expectError(protocol.ClientMsg{Type: protocol.MsgResume, Token: "not-a-token"})
	expectError(protocol.ClientMsg{Type: protocol.MsgStart})
	expectError(protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Ann"}) // name taken
	expectError(protocol.ClientMsg{Type: protocol.MsgJoin, Name: ""})
	expectError("not even an object")

	// Garbage after joining closes the connection.
	c := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Bob"})
	if err := c.conn.Write(ctx, websocket.MessageText, []byte("{{{")); err != nil {
		t.Fatal(err)
	}
	for range c.msgs {
	}
	if code := websocket.CloseStatus(c.conn.Ping(ctx)); code != -1 && code != websocket.StatusUnsupportedData {
		t.Fatalf("close status %v", code)
	}
}

func TestErrorsAreReportedToTheSender(t *testing.T) {
	srv := newTestServer(t)
	created := createRoom(t, srv, "Ann")
	ann := dial(t, srv, created.Code, protocol.ClientMsg{Type: protocol.MsgResume, Token: created.Token})
	ann.send(protocol.ClientMsg{Type: protocol.MsgStart}) // only one player
	for {
		m := ann.next()
		if m.Type == protocol.MsgError {
			if !strings.Contains(m.Error, "at least 2") {
				t.Fatalf("error = %q", m.Error)
			}
			return
		}
	}
}

func TestREST(t *testing.T) {
	srv := newTestServer(t)
	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/api/rooms", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := post(`not json`); code != http.StatusBadRequest {
		t.Fatalf("bad json: %d", code)
	}
	if code := post(`{"name":""}`); code != http.StatusBadRequest {
		t.Fatalf("empty name: %d", code)
	}
	if code := post(`{"name":"` + strings.Repeat("x", 2000) + `"}`); code != http.StatusBadRequest {
		t.Fatalf("huge body: %d", code)
	}

	created := createRoom(t, srv, "Ann")
	if len(created.Code) != 6 || len(created.Token) != 32 {
		t.Fatalf("created = %+v", created)
	}
	resp, err := http.Get(srv.URL + "/api/rooms/" + strings.ToLower(created.Code))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sum room.Summary
	if err := json.NewDecoder(resp.Body).Decode(&sum); err != nil || sum.Code != created.Code || sum.Players != 1 || sum.Phase != protocol.PhaseLobby {
		t.Fatalf("summary = %+v, %v", sum, err)
	}
	resp2, err := http.Get(srv.URL + "/api/rooms/ZZZZZZ")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("missing room: %d", resp2.StatusCode)
	}
}
