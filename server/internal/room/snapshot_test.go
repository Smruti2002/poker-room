package room

import (
	"encoding/json"
	mrand "math/rand/v2"
	"slices"
	"testing"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// roundTrip snapshots r, encodes it as JSON, and restores it.
func roundTrip(t *testing.T, r *Room) *Room {
	t.Helper()
	b, err := json.Marshal(r.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	back, err := Restore(s, r.opts)
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func viewJSON(t *testing.T, r *Room, viewer string) string {
	t.Helper()
	v := r.View(viewer)
	v.Reactions = nil // short-lived, not saved
	v.Now = 0         // wall-clock time of the view itself
	b, err := json.Marshal(struct {
		View    protocol.State
		History []protocol.HistoryHand
	}{v, r.History(viewer)})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// randomStep sends one plausible message: mostly the moves the game is
// waiting for, sometimes noise. Errors are fine.
func randomStep(h *harness, rng *mrand.Rand) {
	ps := h.r.players
	p := ps[rng.IntN(len(ps))]
	var m protocol.ClientMsg
	switch rng.IntN(12) {
	case 0:
		m = protocol.ClientMsg{Type: protocol.MsgChat, Text: "gg"}
	case 1:
		m = protocol.ClientMsg{Type: protocol.MsgSitOut}
	case 2:
		m = protocol.ClientMsg{Type: protocol.MsgSitIn}
	case 3:
		m = protocol.ClientMsg{Type: protocol.MsgTakeLoan}
	case 6:
		m = protocol.ClientMsg{Type: protocol.MsgAddBot}
		p = h.r.player(h.r.hostID)
	case 5:
		// Come and go, and vote people out.
		p.Connections = rng.IntN(2)
		h.r.resolveKicks()
		m = protocol.ClientMsg{Type: protocol.MsgVoteKick, Target: ps[rng.IntN(len(ps))].ID, Yes: rng.IntN(3) > 0}
	case 4:
		if h.r.stage == protocol.StageChoosing {
			p = h.r.player(h.r.dealerID)
			m = protocol.ClientMsg{Type: protocol.MsgChooseVariant, Variant: h.r.options[rng.IntN(len(h.r.options))]}
		}
	default:
		// The move the game is waiting for, by the right player.
		_ = h.tryAdvanceWithMoves(rng)
		return
	}
	_ = h.r.Handle(p.ID, m)
}

// tryAdvanceWithMoves makes a real move for whoever the game waits on, or
// takes the next hand.
func (h *harness) tryAdvanceWithMoves(rng *mrand.Rand) error {
	if h.r.stage != protocol.StageHand {
		return h.tryAdvance()
	}
	hand := h.r.hand
	i := hand.ToAct
	if hand.Allocating {
		i = hand.Pending()[0]
	}
	if hand.Drawing {
		i = hand.DrawPending()[0]
	}
	l := hand.Legal(i)
	var a *protocol.Action
	switch {
	case l.Draw:
		hole := cardStrings(hand.Players[i].Hole)
		a = &protocol.Action{Type: "draw", Cards: hole[:rng.IntN(len(hole)+1)]}
	case l.Allocate:
		a = &protocol.Action{Type: "allocate", Cards: cardStrings(hand.DefaultSplit(i).Cards)}
	case l.Choose:
		a = &protocol.Action{Type: "choose", Count: []int{4, l.ChooseMax}[rng.IntN(2)]}
	case l.Show:
		a = &protocol.Action{Type: "show", Cards: cardStrings(hand.Players[i].Hole[:2])}
	case (l.Raise || l.Bet) && rng.IntN(4) == 0:
		typ := "raise"
		if l.Bet {
			typ = "bet"
		}
		a = &protocol.Action{Type: typ, To: l.MinTo + rng.Int64N(l.MaxTo-l.MinTo+1)}
	case l.Fold && rng.IntN(5) == 0:
		a = &protocol.Action{Type: "fold"}
	case l.Check:
		a = &protocol.Action{Type: "check"}
	default:
		a = &protocol.Action{Type: "call"}
	}
	return h.r.Handle(h.r.handIDs[i], protocol.ClientMsg{Type: protocol.MsgAction, Action: a})
}

// After every step of many random games across all variants, a snapshot
// restored from JSON shows every player exactly what the original does.
func TestSnapshotRestoresEveryState(t *testing.T) {
	games := 40
	if testing.Short() {
		games = 8
	}
	for g := range games {
		rng := mrand.New(mrand.NewPCG(uint64(g), 99))
		h := newHarnessOpts(t, Options{Variants: engine.AllVariants()}, "Ann", "Bob", "Cy", "Dee")
		h.r.settings.SmallBlind, h.r.settings.BigBlind = 50, 100 // more all-ins and loans
		checkAll := func(step int) {
			back := roundTrip(t, h.r)
			for _, p := range h.r.players {
				back.player(p.ID).Connections = p.Connections // live, not saved
			}
			for _, p := range h.r.players {
				if got, want := viewJSON(t, back, p.ID), viewJSON(t, h.r, p.ID); got != want {
					t.Fatalf("game %d step %d: %s's view differs after restore\n got: %s\nwant: %s", g, step, p.Name, got, want)
				}
			}
		}
		checkAll(0)
		h.start()
		for step := 1; step <= 120; step++ {
			randomStep(h, rng)
			checkAll(step)
		}
	}
}

// A restored room keeps playing: the hand in progress continues, and
// players resume their seats with their old session tokens.
func TestRestoredRoomPlaysOn(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"holdem": engine.Holdem}}, "Ann", "Bob", "Cy")
	h.start()
	h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "call"}})

	back := roundTrip(t, h.r)
	if back.PlayerByToken(h.ps[1].Token) == nil {
		t.Fatal("tokens must survive a restore")
	}
	h.r = back
	h.playPassively()
	if got := h.totalStacks(); got != 6000 {
		t.Fatalf("chips after restore = %d", got)
	}
	h.advance()
	if h.r.handNo != 2 {
		t.Fatalf("hand %d after next hand", h.r.handNo)
	}
}

func TestRestoreRejectsBadSnapshots(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	s := h.r.Snapshot()

	old := s
	old.Version = 0
	if _, err := Restore(old, h.r.opts); err == nil {
		t.Fatal("an unknown version should be refused")
	}
	broken := s
	l := *s.Hand
	l.Actions = append(slices.Clone(l.Actions), engine.LoggedAction{Player: 9, Action: engine.Action{Type: engine.Check}})
	broken.Hand = &l
	if _, err := Restore(broken, h.r.opts); err == nil {
		t.Fatal("a hand that can't be replayed should be refused")
	}
}
