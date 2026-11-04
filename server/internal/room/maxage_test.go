package room

import (
	"encoding/json"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"pokerroom/internal/protocol"
)

// A room is closed when it reaches its maximum age, even with people in
// it and a hand going; rooms opened later get their own full time.
func TestSweepClosesRoomsAtTheirMaxAge(t *testing.T) {
	var ms atomic.Int64
	ms.Store(1_000_000)
	now := func() time.Time { return time.UnixMilli(ms.Load()) }
	m := NewManager(Options{Now: now, MaxAge: 12 * time.Hour}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	old, token, _ := m.Create("Ann")
	c := NewClient()
	if err := m.Get(old).Connect(c, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatal(err)
	}
	m.Get(old).Inspect(func(r *Room) {
		if got, want := r.View(r.players[0].ID).ClosesAt, time.UnixMilli(1_000_000).Add(12*time.Hour).UnixMilli(); got != want {
			t.Fatalf("closesAt %d, want %d", got, want)
		}
	})

	ms.Add((6 * time.Hour).Milliseconds())
	newer, _, _ := m.Create("Bob")

	ms.Add((6*time.Hour - time.Minute).Milliseconds()) // 11h59m after the first room
	if n := m.Sweep(24 * time.Hour); n != 0 {
		t.Fatalf("swept %d rooms before their time", n)
	}
	ms.Add(time.Minute.Milliseconds()) // exactly 12h
	if n := m.Sweep(24 * time.Hour); n != 1 {
		t.Fatalf("swept %d rooms, want 1", n)
	}
	if m.Get(old) != nil {
		t.Fatal("the 12-hour-old room should be gone, busy or not")
	}
	select {
	case <-c.Done(): // closed by the room's goroutine as it stops
	case <-time.After(2 * time.Second):
		t.Fatal("players in a closed room are disconnected")
	}
	if m.Get(newer) == nil {
		t.Fatal("the newer room has 6 hours left")
	}
}

func TestNoMaxAgeMeansNoClosingTime(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	if !h.r.ClosesAt().IsZero() || h.r.View(h.ps[0].ID).ClosesAt != 0 {
		t.Fatal("without a maximum age a room never closes on its own")
	}
}

// A restart keeps each room's creation time, so it doesn't get a fresh 12
// hours; rooms saved before this field existed count from the restart.
func TestCreationTimeSurvivesRestart(t *testing.T) {
	at := time.UnixMilli(5_000_000)
	h := newHarnessOpts(t, Options{MaxAge: 12 * time.Hour}, "Ann", "Bob")
	h.r.CreatedAt = at

	later := Options{MaxAge: 12 * time.Hour, Now: func() time.Time { return at.Add(3 * time.Hour) }}
	b, _ := json.Marshal(h.r.Snapshot())
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	back, err := Restore(s, later)
	if err != nil {
		t.Fatal(err)
	}
	if !back.CreatedAt.Equal(at) || !back.ClosesAt().Equal(at.Add(12*time.Hour)) {
		t.Fatalf("created %v, closes %v", back.CreatedAt, back.ClosesAt())
	}

	s.CreatedAt = 0 // an older snapshot
	back, err = Restore(s, later)
	if err != nil {
		t.Fatal(err)
	}
	if !back.CreatedAt.Equal(at.Add(3 * time.Hour)) {
		t.Fatalf("old snapshot: created %v, want the restart time", back.CreatedAt)
	}
}
