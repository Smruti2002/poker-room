package room

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

func peekabooHarness(t *testing.T) *harness {
	return newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"peekaboo": engine.Peekaboo}}, "Ann", "Bob", "Cy")
}

// rollAll rolls for everyone and returns the first player who may peek and
// the first who rolled a 6 (nil if nobody did).
func rollAll(h *harness) (peeker, blind *Player) {
	h.t.Helper()
	for _, i := range h.r.hand.RollPending() {
		h.must(h.r.player(h.r.handIDs[i]), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "roll"}})
	}
	for i, id := range h.r.handIDs {
		switch d := h.r.hand.Players[i].Die; {
		case d < engine.NoPeek && peeker == nil:
			peeker = h.r.player(id)
		case d == engine.NoPeek && blind == nil:
			blind = h.r.player(id)
		}
	}
	return peeker, blind
}

func TestPeekabooPeekOnce(t *testing.T) {
	h := peekabooHarness(t)
	h.start()
	if !h.r.hand.Rolling || !seatOf(h.r.View(h.ps[0].ID), h.ps[0].ID).Rolling {
		t.Fatal("a Peekaboo hand starts with everyone rolling")
	}
	if seatOf(h.r.View(h.ps[0].ID), h.ps[0].ID).CanPeek {
		t.Fatal("peek offered before rolling")
	}

	// Play hands until someone gets a card to peek at.
	var peeker *Player
	for range 30 {
		if peeker, _ = rollAll(h); peeker != nil {
			break
		}
		h.playPassively()
		h.advance()
	}
	if peeker == nil {
		t.Fatal("nobody rolled 1-5 in 30 hands")
	}
	var other *Player
	for _, p := range h.ps {
		if p != peeker {
			other = p
		}
	}
	i := slices.Index(h.r.handIDs, peeker.ID)
	die := h.r.hand.Players[i].Die
	want, _ := h.r.hand.PeekCard(i)

	// Everyone sees the roll; only the peeker may look.
	if seatOf(h.r.View(other.ID), peeker.ID).Die != die {
		t.Fatal("the roll should be public")
	}
	me := seatOf(h.r.View(peeker.ID), peeker.ID)
	if !me.CanPeek || me.PeekCard != "" {
		t.Fatalf("before peeking: %+v", me)
	}
	if err := h.do(peeker, protocol.ClientMsg{Type: protocol.MsgPeekDone}); err == nil {
		t.Fatal("closed a peek that wasn't open")
	}

	h.must(peeker, protocol.ClientMsg{Type: protocol.MsgPeek})
	if me = seatOf(h.r.View(peeker.ID), peeker.ID); me.PeekCard != want.String() || me.CanPeek {
		t.Fatalf("while peeking: %+v, want %v", me, want)
	}
	hiddenFromOthers(t, h, peeker, want.String(), "while peeking")

	// It survives a restart while open.
	b, _ := json.Marshal(h.r.Snapshot())
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	r, err := Restore(s, h.r.opts)
	if err != nil {
		t.Fatal(err)
	}
	if seatOf(r.View(peeker.ID), peeker.ID).PeekCard != want.String() {
		t.Fatal("open peek lost after a restart")
	}

	// Closed, it can't be reopened.
	h.must(peeker, protocol.ClientMsg{Type: protocol.MsgPeekDone})
	if me = seatOf(h.r.View(peeker.ID), peeker.ID); me.PeekCard != "" || !me.PeekUsed || me.CanPeek {
		t.Fatalf("after closing: %+v", me)
	}
	if err := h.do(peeker, protocol.ClientMsg{Type: protocol.MsgPeek}); err == nil {
		t.Fatal("peeked twice")
	}
	hiddenFromOthers(t, h, peeker, want.String(), "after closing")
	// Not even the peeker gets it back, anywhere in their state.
	if b, _ := json.Marshal(h.r.View(peeker.ID)); strings.Contains(string(b), `"`+want.String()+`"`) {
		t.Fatalf("closed peek still in the peeker's state: %s", b)
	}

	// Once the card is dealt there's nothing left to peek at, and it's the
	// card they saw.
	h.playPassively()
	if got := h.r.hand.Boards[0][die-1]; got != want {
		t.Fatalf("board card %d is %v, peeked %v", die, got, want)
	}
	if me = seatOf(h.r.View(peeker.ID), peeker.ID); me.CanPeek || me.PeekUsed || me.PeekCard != "" {
		t.Fatalf("after the hand: %+v", me)
	}
}

// A 6 sees nothing, and the next hand starts with fresh peeks.
func TestPeekabooSixAndNewHand(t *testing.T) {
	h := peekabooHarness(t)
	h.start()
	var blind *Player
	for range 40 {
		if _, blind = rollAll(h); blind != nil {
			break
		}
		h.playPassively()
		h.advance()
	}
	if blind == nil {
		t.Fatal("nobody rolled a 6 in 40 hands")
	}
	if seatOf(h.r.View(blind.ID), blind.ID).CanPeek || h.do(blind, protocol.ClientMsg{Type: protocol.MsgPeek}) == nil {
		t.Fatal("a 6 shouldn't peek")
	}
	h.playPassively()
	h.advance()
	if h.r.peeks != nil {
		t.Fatalf("peeks carried into the next hand: %v", h.r.peeks)
	}
}

// hiddenFromOthers checks that no other player's state mentions card
// anywhere: not on a seat, the board, the log or the hand view. The card
// isn't dealt yet, so it can't legitimately appear.
func hiddenFromOthers(t *testing.T, h *harness, peeker *Player, card, when string) {
	t.Helper()
	for _, p := range h.ps {
		if p == peeker {
			continue
		}
		b, err := json.Marshal(h.r.View(p.ID))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"`+card+`"`) {
			t.Fatalf("%s: %s's state contains the peeked card %s:\n%s", when, p.Name, card, b)
		}
		c, err := cards.Parse(card)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range h.r.View(p.ID).Log {
			if strings.Contains(line, c.Pretty()) {
				t.Fatalf("%s: %s's log mentions %s: %q", when, p.Name, card, line)
			}
		}
	}
}
