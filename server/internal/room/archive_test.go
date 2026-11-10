package room

import (
	"errors"
	"testing"

	"pokerroom/internal/engine"
	"pokerroom/internal/history"
	"pokerroom/internal/protocol"
)

// Every hand is archived once, as it finishes, with the players' names;
// rebuilding a room after a restart doesn't archive its hands again.
func TestEveryFinishedHandIsArchived(t *testing.T) {
	var got []history.Record
	opts := Options{Variants: engine.AllVariants(), Archive: func(r history.Record) { got = append(got, r) }}
	h := newHarnessOpts(t, opts, "Ann", "Bob", "Cy")
	h.start()
	for len(got) < 30 { // more than the 20 hands a room keeps itself
		if err := h.tryAdvance(); err != nil {
			t.Fatalf("%v (stage %q, after %d hands)", err, h.r.stage, len(got))
		}
	}
	for i, r := range got {
		if r.Number != i+1 || r.Room != "ABCDEF" || len(r.Players) == 0 || r.Players[0].Name == "" {
			t.Fatalf("record %d: %+v", i, r)
		}
		if _, err := engine.Replay(r.Log); err != nil {
			t.Fatalf("hand %d doesn't replay: %v", r.Number, err)
		}
	}
	n := len(got)
	if _, err := Restore(h.r.Snapshot(), opts); err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatal("restoring a room must not archive its hands again")
	}
}

// The download: only for players of the room, only once the game is over
// (it shows everyone's cards), and only where hands are archived.
func TestHandHistoryDownload(t *testing.T) {
	m := newTestManager()
	store, err := history.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.KeepHistory(store)
	code, annToken, _ := m.Create("Ann")
	a := m.Get(code)
	var bobToken string
	a.Inspect(func(r *Room) {
		p, _ := r.Join("Bob")
		bobToken = p.Token
		r.settings.Variants = []string{"holdem"}
	})

	a.Inspect(func(r *Room) {
		h := &harness{t: t, r: r, ps: r.players}
		h.start()
		for range 3 {
			h.playPassively()
			h.advance()
		}
		h.playPassively()
	})
	if _, err := a.HandHistory(annToken); !errors.Is(err, ErrNotOver) {
		t.Fatalf("during the game: %v", err)
	}
	a.Inspect(func(r *Room) { _ = r.Handle(r.hostID, protocol.ClientMsg{Type: protocol.MsgEndGame}) })
	if _, err := a.HandHistory("not-a-token"); !errors.Is(err, ErrNotYourRoom) {
		t.Fatalf("a stranger: %v", err)
	}

	export, err := a.HandHistory(bobToken)
	if err != nil {
		t.Fatal(err)
	}
	if export.Room != code || export.HandsPlayed != 4 || len(export.Hands) != 4 || len(export.FinalStandings) != 2 {
		t.Fatalf("export: room %s, played %d, hands %d, standings %d", export.Room, export.HandsPlayed, len(export.Hands), len(export.FinalStandings))
	}
	var profit int64
	for _, s := range export.FinalStandings {
		profit += s.Profit
	}
	if profit != 0 {
		t.Fatalf("no loans were taken, so profits should cancel out: %d", profit)
	}
	for _, hand := range export.Hands {
		for _, s := range hand.Seats {
			if len(s.Hole) != 2 {
				t.Fatalf("hand %d: %s's cards missing", hand.Number, s.Name)
			}
		}
	}

	// A server without an archive says so.
	plain := newTestManager()
	c2, tok, _ := plain.Create("Ann")
	if _, err := plain.Get(c2).HandHistory(tok); !errors.Is(err, ErrNoArchive) {
		t.Fatalf("no archive: %v", err)
	}
}
