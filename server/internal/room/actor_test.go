package room

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"pokerroom/internal/protocol"
)

func newTestManager() *Manager {
	return NewManager(Options{ResultDelay: 10 * time.Millisecond, ShowdownDelay: 10 * time.Millisecond}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func recv(t *testing.T, c *Client) protocol.ServerMsg {
	t.Helper()
	select {
	case b := <-c.Send():
		var m protocol.ServerMsg
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	case <-time.After(2 * time.Second):
		t.Fatal("no message")
	}
	return protocol.ServerMsg{}
}

func TestManagerCreateAndGet(t *testing.T) {
	m := newTestManager()
	if _, _, err := m.Create(""); err == nil {
		t.Fatal("empty host name should fail")
	}
	code, token, err := m.Create("Ann")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 || strings.ContainsAny(code, "01OIL") || token == "" {
		t.Fatalf("code %q token %q", code, token)
	}
	a := m.Get(strings.ToLower(code))
	if a == nil || m.Get("NOPE") != nil {
		t.Fatal("lookup broken")
	}
	if s := a.Summary(); s.Players != 1 || s.Phase != protocol.PhaseLobby || s.Max != 8 {
		t.Fatalf("summary %+v", s)
	}
}

func TestActorConnectSubmitDisconnect(t *testing.T) {
	m := newTestManager()
	code, token, _ := m.Create("Ann")
	a := m.Get(code)

	ann := NewClient()
	if err := a.Connect(ann, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatal(err)
	}
	if w := recv(t, ann); w.Type != protocol.MsgWelcome || w.Token != token {
		t.Fatalf("welcome = %+v", w)
	}
	if s := recv(t, ann); s.Type != protocol.MsgState || s.State.You != "p1" {
		t.Fatalf("state = %+v", s)
	}

	// Errors go back to the sender only.
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgStart})
	if e := recv(t, ann); e.Type != protocol.MsgError {
		t.Fatalf("want error, got %+v", e)
	}

	if err := a.Connect(NewClient(), protocol.ClientMsg{Type: "hello"}); err == nil {
		t.Fatal("bad first message should fail")
	}

	a.Disconnect(ann)
	select {
	case <-ann.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("client not closed on disconnect")
	}
	a.Inspect(func(r *Room) {
		if r.players[0].Connections != 0 {
			t.Errorf("connections = %d", r.players[0].Connections)
		}
	})
	// Messages from a disconnected client are ignored.
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgSitOut})
	a.Inspect(func(r *Room) {
		if r.players[0].SittingOut {
			t.Error("disconnected client's message was applied")
		}
	})
}

// A client that stops reading is dropped instead of blocking the room.
func TestActorDropsSlowClient(t *testing.T) {
	m := newTestManager()
	code, token, _ := m.Create("Ann")
	a := m.Get(code)
	slow := NewClient()
	if err := a.Connect(slow, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatal(err)
	}
	for range cap(slow.send) + 5 {
		a.Submit(slow, protocol.ClientMsg{Type: protocol.MsgSitOut})
	}
	select {
	case <-slow.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("slow client was not dropped")
	}
}

// A panic inside the room is contained to that message.
func TestActorRecoversFromPanic(t *testing.T) {
	m := newTestManager()
	code, _, _ := m.Create("Ann")
	a := m.Get(code)
	a.inbox <- func() { panic("boom") }
	if s := a.Summary(); s.Code != code {
		t.Fatal("actor stopped after a panic")
	}
}

func TestActorHistoryGoesOnlyToTheAsker(t *testing.T) {
	m := newTestManager()
	code, token, _ := m.Create("Ann")
	a := m.Get(code)
	ann := NewClient()
	if err := a.Connect(ann, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatal(err)
	}
	recv(t, ann) // welcome
	recv(t, ann) // state
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgHistory})
	if msg := recv(t, ann); msg.Type != protocol.MsgHistoryReply || msg.State != nil {
		t.Fatalf("got %+v", msg)
	}
}
