package room

import (
	"slices"
	"testing"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

func act(typ string, to int64) protocol.ClientMsg {
	return protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: typ, To: to}}
}

// lastBet is what everyone sees on p's seat.
func lastBet(h *harness, p *Player) *protocol.BetCallout {
	h.t.Helper()
	v := h.r.View(h.ps[0].ID)
	i := slices.IndexFunc(v.Players, func(q protocol.PlayerView) bool { return q.ID == p.ID })
	return v.Players[i].LastBet
}

func TestBetCallouts(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy") // blinds 10/20
	h.start()

	first := h.toAct()
	h.must(first, act("raise", 60))
	if got := lastBet(h, first); got == nil || *got != (protocol.BetCallout{Type: "raise", By: 40, To: 60}) {
		t.Fatalf("raise to 60 over the big blind: %+v", got)
	}

	second := h.toAct()
	h.must(second, act("raise", 200))
	if got := lastBet(h, second); got == nil || got.By != 140 || got.To != 200 {
		t.Fatalf("re-raise: %+v", got)
	}
	if lastBet(h, first) == nil {
		t.Fatal("an earlier raiser keeps their callout until they act again")
	}

	// Everyone else calls: callouts go as each player acts again.
	for h.r.hand.Street == engine.Preflop {
		p := h.toAct()
		h.must(p, act("call", 0))
		if p == first && lastBet(h, first) != nil {
			t.Fatal("calling replaces the earlier raise")
		}
	}
	// A new street starts clean.
	for _, p := range h.ps {
		if lastBet(h, p) != nil {
			t.Fatalf("%s still has a callout on the flop", p.Name)
		}
	}

	better := h.toAct()
	h.must(better, act("bet", 50))
	if got := lastBet(h, better); got == nil || *got != (protocol.BetCallout{Type: "bet", By: 50, To: 50}) {
		t.Fatalf("flop bet: %+v", got)
	}

	h.playPassively()
	for _, p := range h.ps {
		if lastBet(h, p) != nil {
			t.Fatal("no callouts once the hand is over")
		}
	}
}

func TestAllInRaiseCallout(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	p := h.toAct()
	l := h.r.hand.Legal(h.r.hand.ToAct)
	h.must(p, act("raise", l.MaxTo))
	got := lastBet(h, p)
	if got == nil || !got.AllIn || got.To != 2000 || got.By != 1980 {
		t.Fatalf("all-in raise: %+v", got)
	}
}

// A Dacoit player who took 6 cards puts in twice the chips, and the callout
// shows what they actually put in.
func TestDacoitDoubleCallout(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"dacoit": engine.Dacoit}}, "Ann", "Bob", "Cy")
	h.start()
	for h.r.hand.Picking {
		i := h.r.hand.ToAct
		count := 4
		if h.r.hand.Legal(i).ChooseMax == 6 {
			count = 6
		}
		h.must(h.r.player(h.r.handIDs[i]), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "choose", Count: count}})
	}
	// Find a 6-card player and let them raise.
	for !h.r.hand.Done {
		i := h.r.hand.ToAct
		p := h.toAct()
		if h.r.hand.Players[i].Mult == 2 && h.r.hand.Legal(i).Raise {
			cb := h.r.hand.CurrentBet
			h.must(p, act("raise", cb+50))
			got := lastBet(h, p)
			if got == nil || got.By != 100 || got.To != (cb+50)*2 {
				t.Fatalf("6-card raise of 50 units: %+v (current bet %d units)", got, cb)
			}
			return
		}
		h.must(p, act("call", 0))
	}
	t.Fatal("no 6-card player got to raise")
}
