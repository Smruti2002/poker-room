package room

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pokerroom/internal/protocol"
)

func newTestManager() *Manager {
	return NewManager(Options{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
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

func TestClaimHostOnlyWhenHostIsOffline(t *testing.T) {
	m := newTestManager()
	code, annToken, _ := m.Create("Ann")
	a := m.Get(code)
	ann, bob := NewClient(), NewClient()
	if err := a.Connect(ann, protocol.ClientMsg{Type: protocol.MsgResume, Token: annToken}); err != nil {
		t.Fatal(err)
	}
	if err := a.Connect(bob, protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Bob"}); err != nil {
		t.Fatal(err)
	}
	claim := func(c *Client) error {
		var err error
		a.Inspect(func(r *Room) { err = r.Handle(c.playerID, protocol.ClientMsg{Type: protocol.MsgClaimHost}) })
		return err
	}
	if err := claim(bob); err == nil {
		t.Fatal("can't take over while the host is connected")
	}
	if err := claim(ann); err == nil {
		t.Fatal("the host can't claim again")
	}
	a.Disconnect(ann)
	if err := claim(bob); err != nil {
		t.Fatal(err)
	}
	a.Inspect(func(r *Room) {
		if r.hostID != bob.playerID {
			t.Fatalf("host is %s", r.hostID)
		}
	})
}

func TestSweepClosesIdleEmptyRooms(t *testing.T) {
	// The room goroutine reads the clock while the test moves it.
	var ms atomic.Int64
	ms.Store(1_000_000)
	m := NewManager(Options{Now: func() time.Time { return time.UnixMilli(ms.Load()) }}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	busy, _, _ := m.Create("Ann")
	idle, _, _ := m.Create("Bob")
	c := NewClient()
	var token string
	m.Get(busy).Inspect(func(r *Room) { token = r.players[0].Token })
	if err := m.Get(busy).Connect(c, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatal(err)
	}

	ms.Add(time.Hour.Milliseconds())
	if n := m.Sweep(2 * time.Hour); n != 0 {
		t.Fatalf("swept %d rooms too early", n)
	}
	ms.Add(5 * time.Hour.Milliseconds())
	if n := m.Sweep(2 * time.Hour); n != 1 {
		t.Fatalf("swept %d rooms, want 1", n)
	}
	if m.Get(idle) != nil || m.Get(busy) == nil {
		t.Fatal("the idle room should go, the busy one stay")
	}
	if s := m.Stats(); s.Rooms != 1 || s.Connections != 1 {
		t.Fatalf("stats %+v", s)
	}
	// A closed room refuses new work instead of hanging.
	gone := NewClient()
	closed := newActor(New("ZZZZZZ", Options{}), slog.New(slog.NewTextHandler(io.Discard, nil)))
	closed.stop()
	if err := closed.Connect(gone, protocol.ClientMsg{Type: protocol.MsgJoin, Name: "Cy"}); !errors.Is(err, ErrRoomClosed) {
		t.Fatalf("connect to closed room: %v", err)
	}
	closed.Submit(gone, protocol.ClientMsg{Type: protocol.MsgSitOut}) // must not block
	closed.Disconnect(gone)
}

func TestMaxRooms(t *testing.T) {
	m := newTestManager()
	m.MaxRooms = 2
	for range 2 {
		if _, _, err := m.Create("Ann"); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := m.Create("Ann"); !errors.Is(err, ErrTooManyRooms) {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveAndLoadAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.json")
	m := newTestManager()
	if n, err := m.LoadAll(path); err != nil || n != 0 {
		t.Fatalf("missing file: %d, %v", n, err)
	}
	code, token, _ := m.Create("Ann")
	m.Get(code).Inspect(func(r *Room) {
		_, _ = r.Join("Bob")
		_ = r.Handle(r.hostID, protocol.ClientMsg{Type: protocol.MsgStart})
	})
	if err := m.SaveAll(path); err != nil {
		t.Fatal(err)
	}

	// A fresh server picks the rooms up, hand in progress and all.
	m2 := newTestManager()
	if n, err := m2.LoadAll(path); err != nil || n != 1 {
		t.Fatalf("loaded %d rooms, err %v", n, err)
	}
	var before, after string
	m.Get(code).Inspect(func(r *Room) { before = viewJSON(t, r, "p1") })
	m2.Get(code).Inspect(func(r *Room) { after = viewJSON(t, r, "p1") })
	if before != after {
		t.Fatalf("restored room differs\n got: %s\nwant: %s", after, before)
	}
	c := NewClient()
	if err := m2.Get(code).Connect(c, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatalf("resume after restore: %v", err)
	}

	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newTestManager().LoadAll(path); err == nil {
		t.Fatal("a corrupt file should be an error")
	}
	if err := m.SaveAll(filepath.Join(t.TempDir(), "missing-dir", "rooms.json")); err == nil {
		t.Fatal("saving into a missing directory should fail")
	}
}

// Regression (race detector): a room closing while someone connects must
// not leave the connect half-run. Each call either completes or never runs.
func TestCloseRacesWithCalls(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for i := range 200 {
		a := newActor(New("RACE01", Options{}), log)
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				err := a.Connect(NewClient(), protocol.ClientMsg{Type: protocol.MsgJoin, Name: "P" + strconv.Itoa(i)})
				if err != nil && !errors.Is(err, ErrRoomClosed) && !strings.Contains(err.Error(), "taken") {
					t.Errorf("unexpected error: %v", err)
				}
				_ = a.Summary()
			})
		}
		wg.Go(a.stop)
		wg.Wait()
	}
}
