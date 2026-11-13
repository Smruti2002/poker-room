package room

import (
	"encoding/json"
	"reflect"
	"testing"

	"pokerroom/internal/protocol"
)

// playHands plays n hands: the first player to act folds every other hand,
// everyone else checks or calls, so there are fold wins and showdowns.
func playHands(h *harness, n int) {
	h.t.Helper()
	for i := range n {
		if i%2 == 1 {
			h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
		}
		h.playPassively()
		if i < n-1 {
			h.advance()
		}
	}
}

func TestGameStats(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	playHands(h, 6)
	if v := h.r.View(h.ps[0].ID); v.Stats != nil {
		t.Fatal("stats sent before the game is over")
	}
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgEndGame})
	g := h.r.View(h.ps[1].ID).Stats
	if g == nil {
		t.Fatal("no stats once the game is over")
	}

	if g.Hands != 6 || len(g.Variants) != 1 || g.Variants[0] != (protocol.VariantCount{Variant: "holdem", Hands: 6}) {
		t.Fatalf("hands %d, variants %+v", g.Hands, g.Variants)
	}
	if len(g.Players) != 3 {
		t.Fatalf("%d players with stats", len(g.Players))
	}
	var net, biggestWin int64
	won, folded := 0, 0
	for _, p := range g.Players {
		if p.Hands != 6 {
			t.Fatalf("%s dealt %d hands, want 6", p.ID, p.Hands)
		}
		if p.ShowdownsWon > p.Showdowns || p.Showdowns+p.Folded > p.Hands || p.Won > p.Hands {
			t.Fatalf("inconsistent stats %+v", p)
		}
		for _, n := range p.Net {
			net += n.Chips
		}
		won += p.Won
		folded += p.Folded
		biggestWin = max(biggestWin, p.BiggestWin)
	}
	// Chips only move between players during hands.
	if net != 0 {
		t.Fatalf("net chips sum to %d, want 0", net)
	}
	if folded != 3 || won < 6 {
		t.Fatalf("folded %d (want 3), won %d (want at least one winner per hand)", folded, won)
	}
	// Each player's net matches how their stack moved: no loans were taken.
	for i, p := range g.Players {
		if got, want := p.Net[0].Chips, h.ps[i].Stack-h.r.settings.StartingStack; got != want {
			t.Fatalf("%s net %d, stack moved %d", p.ID, got, want)
		}
	}
	if g.BiggestPot == nil || g.BiggestPot.Chips < biggestWin || len(g.BiggestPot.Winners) == 0 {
		t.Fatalf("biggest pot %+v, biggest single win %d", g.BiggestPot, biggestWin)
	}
}

// Stats survive a restart.
func TestGameStatsSnapshot(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	playHands(h, 3)
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgEndGame})

	b, err := json.Marshal(h.r.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	r, err := Restore(s, h.r.opts)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := r.View(h.ps[0].ID).Stats, h.r.View(h.ps[0].ID).Stats; !reflect.DeepEqual(got, want) {
		t.Fatalf("after restore %+v\nwant %+v", got, want)
	}
}

// A snapshot from before stats existed loads, with empty stats.
func TestGameStatsOldSnapshot(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	h.playPassively()
	s := h.r.Snapshot()
	s.Stats = gameStats{}
	for i := range s.Players {
		s.Players[i].Stats = playerStats{}
	}
	r, err := Restore(s, h.r.opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.statsView() != nil {
		t.Fatal("expected no stats from an old snapshot")
	}
}
