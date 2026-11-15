package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

// peekHand starts a Peekaboo hand with fixed dice: seat 0 deals, 1 is the
// small blind, 2 the big blind. The deck gives each seat two hole cards in
// seat order, then the flop, turn and river.
func peekHand(t *testing.T, dice []int, deck string) *Hand {
	t.Helper()
	seats := []SeatSpec{{Stack: 1000}, {Stack: 1000}, {Stack: 1000}}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20, Order: OrderLastRaiser}, Peekaboo, seats, 0,
		Options{Seed: cards.SeedFromUint64(9), Deck: cards.MustParseList(deck), Dice: dice})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

const peekDeck = "As Ad  Ks Kd  Qs Qd  2c 7d 9h  Jc  3s"

func TestPeekabooRollThenHoldem(t *testing.T) {
	h := peekHand(t, []int{3, 6, 5}, peekDeck)
	if !h.Rolling || h.ToAct != -1 || h.Street != Preflop {
		t.Fatalf("after the deal: rolling %v, to act %d, street %v", h.Rolling, h.ToAct, h.Street)
	}
	if l := h.Legal(1); !l.Roll || l.Call || l.Fold {
		t.Fatalf("legal while rolling: %+v", l)
	}
	if err := h.Apply(1, Action{Type: Call}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("calling before rolling: %v", err)
	}

	// Rolls come in any order; betting starts when the last one is in.
	for _, i := range []int{2, 0} {
		if err := h.Apply(i, Action{Type: Roll}); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Apply(2, Action{Type: Roll}); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("rolling twice: %v", err)
	}
	if !h.Rolling || !slices.Equal(h.RollPending(), []int{1}) {
		t.Fatalf("pending %v", h.RollPending())
	}
	if err := h.Apply(1, Action{Type: Roll}); err != nil {
		t.Fatal(err)
	}
	if h.Rolling || h.ToAct < 0 {
		t.Fatalf("betting should start: rolling %v, to act %d", h.Rolling, h.ToAct)
	}

	// Ann rolled 3: the last flop card, 9h. Bob rolled 6: nothing. Cy rolled
	// 5: the river, 3s.
	if c, ok := h.PeekCard(0); !ok || c != cards.MustParseList("9h")[0] {
		t.Fatalf("Ann peeked %v %v", c, ok)
	}
	if _, ok := h.PeekCard(1); ok {
		t.Fatal("a 6 shouldn't peek")
	}
	if c, _ := h.PeekCard(2); c != cards.MustParseList("3s")[0] {
		t.Fatalf("Cy peeked %v", c)
	}

	// Everyone sees the rolls; nobody's log names the peeked card, not even
	// the peeker's.
	for viewer := range 3 {
		log := strings.Join(h.NarrateFor(viewer), "\n")
		if !strings.Contains(log, "P0 rolls a 3: peeks at flop card 3") || !strings.Contains(log, "P1 rolls a 6: no peek") {
			t.Fatalf("viewer %d log:\n%s", viewer, log)
		}
		if strings.Contains(log, "9♥") || strings.Contains(log, "3♠") {
			t.Fatalf("viewer %d sees a peeked card:\n%s", viewer, log)
		}
	}
	if !strings.Contains(strings.Join(h.Narrate(), "\n"), "P0 sees 9♥ (flop card 3)") {
		t.Fatal("the full record should name the peeked card")
	}

	// Then it's Hold'em, and the peeked cards turn up where promised.
	for !h.Done {
		l := h.Legal(h.ToAct)
		a := Action{Type: Check}
		if l.Call {
			a.Type = Call
		}
		if err := h.Apply(h.ToAct, a); err != nil {
			t.Fatal(err)
		}
		if err := h.CheckInvariants(); err != nil {
			t.Fatal(err)
		}
	}
	if h.Boards[0][2] != cards.MustParseList("9h")[0] || h.Boards[0][4] != cards.MustParseList("3s")[0] {
		t.Fatalf("board %v", h.Boards[0])
	}
	r, err := Replay(h.Log())
	if err != nil || !slices.Equal(r.Narrate(), h.Narrate()) {
		t.Fatalf("replay: %v", err)
	}
}

// Dice come from the seed: the same seed rolls the same, and the shuffle is
// the same as Hold'em's with that seed.
func TestPeekabooDiceFromSeed(t *testing.T) {
	seats := []SeatSpec{{Stack: 1000}, {Stack: 1000}, {Stack: 1000}, {Stack: 1000}}
	deal := func(v Variant, seed uint64) *Hand {
		h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20}, v, seats, 0, Options{Seed: cards.SeedFromUint64(seed)})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	seen := map[int]bool{}
	for s := range uint64(50) {
		a, b, plain := deal(Peekaboo, s), deal(Peekaboo, s), deal(Holdem, s)
		if !slices.Equal(a.dice, b.dice) {
			t.Fatalf("seed %d rolled %v and %v", s, a.dice, b.dice)
		}
		for i := range seats {
			if !slices.Equal(a.Players[i].Hole, plain.Players[i].Hole) {
				t.Fatalf("seed %d: dice changed the deal", s)
			}
		}
		for _, d := range a.dice {
			if d < 1 || d > 6 {
				t.Fatalf("die %d", d)
			}
			seen[d] = true
		}
	}
	if len(seen) != 6 {
		t.Fatalf("over 200 rolls only saw %v", seen)
	}
}
