package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
)

// playHand plays one heads-up Hold'em hand to the end: Ann raises, Bob calls
// down, and it goes to a showdown.
func playHand(t *testing.T, seed uint64) engine.Log {
	t.Helper()
	h, err := engine.NewHand(engine.Config{SmallBlind: 10, BigBlind: 20}, engine.Holdem,
		[]engine.SeatSpec{{ID: "p1", Stack: 1000}, {ID: "p2", Stack: 1000}}, 0, engine.Options{Seed: cards.SeedFromUint64(seed)})
	if err != nil {
		t.Fatal(err)
	}
	for !h.Done {
		i := h.ToAct
		l := h.Legal(i)
		a := engine.Action{Type: engine.Call}
		switch {
		case l.Raise && h.Street == engine.Preflop && i == 0:
			a = engine.Action{Type: engine.Raise, To: 60}
		case l.Check:
			a = engine.Action{Type: engine.Check}
		}
		if err := h.Apply(i, a); err != nil {
			t.Fatal(err)
		}
	}
	return h.Log()
}

func record(t *testing.T, n int, seed uint64) Record {
	return Record{
		Room: "ABCDEF", Number: n, Variant: "holdem",
		Players: []Player{{ID: "p1", Name: "Ann"}, {ID: "p2", Name: "Bob"}},
		Log:     playHand(t, seed),
	}
}

func TestStoreKeepsEveryHandInOrder(t *testing.T) {
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.Load("ABCDEF"); err != nil || got != nil {
		t.Fatalf("no archive yet: %v, %v", got, err)
	}
	for _, n := range []int{1, 2, 3} {
		if err := s.Append(record(t, n, uint64(n))); err != nil {
			t.Fatal(err)
		}
	}
	// Hand 2 played out again after a crash: the later one counts.
	again := record(t, 2, 99)
	if err := s.Append(again); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("ABCDEF")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Number != 1 || got[1].Number != 2 || got[2].Number != 3 {
		t.Fatalf("hands %+v", got)
	}
	if got[1].Log.Seed != again.Log.Seed {
		t.Fatal("the later copy of a hand should win")
	}
}

func TestStoreRejectsOddRoomCodes(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	for _, code := range []string{"../etc", "a/b", "", "abc"} {
		r := record(t, 1, 1)
		r.Room = code
		if err := s.Append(r); err == nil {
			t.Errorf("room %q should be refused", code)
		}
	}
}

func TestPruneDeletesOnlyOldArchives(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(dir)
	for _, code := range []string{"OLD111", "NEW222"} {
		r := record(t, 1, 1)
		r.Room = code
		if err := s.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	if err := os.Chtimes(filepath.Join(dir, "OLD111.jsonl"), now.Add(-31*24*time.Hour), now.Add(-31*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Prune(30*24*time.Hour, now); err != nil || n != 1 {
		t.Fatalf("pruned %d, %v", n, err)
	}
	if got, _ := s.Load("OLD111"); got != nil {
		t.Fatal("the old archive should be gone")
	}
	if got, _ := s.Load("NEW222"); len(got) != 1 {
		t.Fatal("the recent archive should stay")
	}
}

func TestReplayShowsEveryCardAndAction(t *testing.T) {
	hands, err := Replay([]Record{record(t, 7, 7)})
	if err != nil {
		t.Fatal(err)
	}
	h := hands[0]
	if h.Number != 7 || h.Variant != "holdem" || h.Blinds != [2]int64{10, 20} || len(h.Seats) != 2 {
		t.Fatalf("hand %+v", h)
	}
	var net int64
	for _, s := range h.Seats {
		if s.Name != "Ann" && s.Name != "Bob" || len(s.Hole) != 2 || s.StartStack != 1000 {
			t.Fatalf("seat %+v", s)
		}
		net += s.Net
	}
	if net != 0 {
		t.Fatalf("chips won and lost should cancel out, got %d", net)
	}
	if len(h.Boards) != 1 || len(h.Boards[0]) != 5 {
		t.Fatalf("boards %v", h.Boards)
	}
	var raise bool
	for _, e := range h.Events {
		if e.Kind == "action" && e.Action == "raise" && e.Player == "Ann" && e.Units == 60 {
			raise = true
		}
	}
	if !raise {
		t.Fatalf("Ann's raise to 60 is missing: %+v", h.Events)
	}
	log := strings.Join(h.Log, "\n")
	if !strings.Contains(log, "Ann raises to 60") || strings.Contains(log, "p1") {
		t.Fatalf("log should use names:\n%s", log)
	}
}
