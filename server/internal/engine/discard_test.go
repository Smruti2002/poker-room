package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

// discardHand starts a Discard Hold'em hand: seat 0 deals, 1 is the small
// blind, 2 the big blind. The deck gives each seat six hole cards in seat
// order, then the flop, turn and river.
func discardHand(t *testing.T, stacks []int64, deck string) *Hand {
	t.Helper()
	seats := make([]SeatSpec, len(stacks))
	for i, s := range stacks {
		seats[i] = SeatSpec{Stack: s}
	}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20, Order: OrderLastRaiser}, DiscardHoldem, seats, 0,
		Options{Seed: cards.SeedFromUint64(5), Deck: cards.MustParseList(deck)})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func throwAway(t *testing.T, h *Hand, i int, cs string) {
	t.Helper()
	if err := h.Apply(i, Action{Type: Discard, Cards: cards.MustParseList(cs)}); err != nil {
		t.Fatalf("player %d discards %q: %v", i, cs, err)
	}
}

func checkAround(t *testing.T, h *Hand) {
	t.Helper()
	for !h.Discarding && !h.Done {
		l := h.Legal(h.ToAct)
		a := Action{Type: Check}
		if l.Call {
			a.Type = Call
		}
		if err := h.Apply(h.ToAct, a); err != nil {
			t.Fatal(err)
		}
	}
}

const discardDeck = "As Ad 2c 3c 4h 5h  Ks Kd 7c 8c 9h Th  Qs Qd 2d 3d 4d 6s  Ac Kc 9c  Qh  2h"

func TestDiscardHoldemFromDealToShowdown(t *testing.T) {
	h := discardHand(t, []int64{1000, 1000, 1000}, discardDeck)
	if len(h.Players[0].Hole) != 6 || h.Discarding {
		t.Fatalf("deal: %d hole cards, discarding %v", len(h.Players[0].Hole), h.Discarding)
	}

	// Pre-flop betting with six cards, then the flop opens with a discard.
	checkAround(t, h)
	if !h.Discarding || h.Street != Flop || h.ToAct != -1 || len(h.Boards[0]) != 3 {
		t.Fatalf("after pre-flop: discarding %v, street %v, to act %d", h.Discarding, h.Street, h.ToAct)
	}
	if l := h.Legal(1); !l.Discard || l.DiscardCount != 2 || l.Call || l.Check {
		t.Fatalf("legal during the discard: %+v", l)
	}

	// Mistakes are refused and change nothing.
	for _, bad := range []string{"As", "As Ad 2c", "Ks Kd", "As As"} {
		if err := h.Apply(0, Action{Type: Discard, Cards: cards.MustParseList(bad)}); !errors.Is(err, ErrIllegalAction) {
			t.Fatalf("discarding %q: %v", bad, err)
		}
	}
	if err := h.Apply(0, Action{Type: Check}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("checking during the discard: %v", err)
	}

	// Everyone discards, in any order; nothing happens until the last one.
	throwAway(t, h, 2, "2d 3d")
	throwAway(t, h, 0, "4h 5h")
	if !h.Discarding || !slices.Equal(h.DiscardPending(), []int{1}) {
		t.Fatalf("pending %v", h.DiscardPending())
	}
	if err := h.Apply(2, Action{Type: Discard, Cards: cards.MustParseList("4d 6s")}); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("discarding twice: %v", err)
	}
	throwAway(t, h, 1, "9h Th")
	if h.Discarding || h.ToAct < 0 || len(h.Players[1].Hole) != 4 {
		t.Fatalf("after the discard: discarding %v, to act %d, hole %v", h.Discarding, h.ToAct, h.Players[1].Hole)
	}
	hasCards(t, h.Players[0], "As Ad 2c 3c")

	// Flop betting with four cards, then the turn opens with a discard.
	checkAround(t, h)
	if !h.Discarding || h.Street != Turn {
		t.Fatalf("after the flop: discarding %v, street %v", h.Discarding, h.Street)
	}
	throwAway(t, h, 0, "2c 3c")
	throwAway(t, h, 1, "7c 8c")
	throwAway(t, h, 2, "4d 6s")
	if len(h.Players[2].Hole) != 2 {
		t.Fatalf("after the turn's discard, %d cards", len(h.Players[2].Hole))
	}

	// Turn and river betting with two cards: no discard on the river, so
	// checking around runs to the showdown.
	checkAround(t, h)
	if h.DiscardRound != 2 || !h.Done {
		t.Fatal("hand not finished")
	}
	// Board Ac Kc 9c Qh 2h: Ann (As Ad) has trip aces and wins.
	if h.Players[0].Won == 0 || h.Players[1].Won != 0 || h.Players[2].Won != 0 {
		t.Fatalf("won: %d %d %d", h.Players[0].Won, h.Players[1].Won, h.Players[2].Won)
	}
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}

	// Discards are private: others only see how many.
	log := strings.Join(h.NarrateFor(2), "\n")
	if !strings.Contains(log, "P0 discards 2") || strings.Contains(log, "4h") {
		t.Fatalf("Cy's view of the discards:\n%s", log)
	}
	if !strings.Contains(strings.Join(h.NarrateFor(0), "\n"), "P0 throws away 4♥ 5♥") {
		t.Fatalf("Ann's own discards missing:\n%s", strings.Join(h.NarrateFor(0), "\n"))
	}

	// The hand replays exactly.
	r, err := Replay(h.Log())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.Narrate(), h.Narrate()) {
		t.Fatal("replay differs")
	}
}

// All-in players still discard, and the board runs out between discards.
func TestDiscardHoldemAllIn(t *testing.T) {
	h := discardHand(t, []int64{1000, 100, 1000}, discardDeck)
	// Ann raises; Bob, short, goes all in; Cy folds; Ann calls.
	if err := h.Apply(0, Action{Type: Raise, To: 100}); err != nil {
		t.Fatal(err)
	}
	if err := h.Apply(1, Action{Type: Call}); err != nil {
		t.Fatal(err)
	}
	if err := h.Apply(2, Action{Type: Fold}); err != nil {
		t.Fatal(err)
	}
	if !h.Discarding || !slices.Equal(h.DiscardPending(), []int{0, 1}) {
		t.Fatalf("all-in Bob and Ann should discard; pending %v", h.DiscardPending())
	}
	throwAway(t, h, 0, "4h 5h")
	throwAway(t, h, 1, "9h Th")
	// Nobody can bet, so the turn comes at once, with its discard.
	if !h.Discarding || h.Street != Turn {
		t.Fatalf("street %v, discarding %v", h.Street, h.Discarding)
	}
	throwAway(t, h, 0, "2c 3c")
	throwAway(t, h, 1, "7c 8c")
	if !h.Done {
		t.Fatal("the board should run out to the showdown")
	}
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}
}

// A skipped player keeps the cards that make their best hand.
func TestDefaultDiscard(t *testing.T) {
	h := discardHand(t, []int64{1000, 1000, 1000}, discardDeck)
	checkAround(t, h)
	// Board Ac Kc 9c: Ann (As Ad 2c 3c 4h 5h) keeps her aces and clubs.
	a := h.DefaultDiscard(0)
	if a.Type != Discard || len(a.Cards) != 2 || slices.Contains(a.Cards, cards.MustParseList("As")[0]) || slices.Contains(a.Cards, cards.MustParseList("Ad")[0]) {
		t.Fatalf("default discard %v", a.Cards)
	}
	if err := h.Apply(0, a); err != nil {
		t.Fatal(err)
	}
}
