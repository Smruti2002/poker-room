package engine

import (
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

// bbHand starts a BB hand: seat 0 deals, 1 is the small blind, 2 the big
// blind. With a deck, the hole cards come from it in seat order, then
// exchanges deal from what's left (no shuffle), then folded cards and
// discards.
func bbHand(t *testing.T, stacks []int64, deck string) *Hand {
	t.Helper()
	seats := make([]SeatSpec, len(stacks))
	for i, s := range stacks {
		seats[i] = SeatSpec{Stack: s}
	}
	opts := Options{Seed: cards.SeedFromUint64(3)}
	if deck != "" {
		opts.Deck = cards.MustParseList(deck)
	}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20, Order: OrderLastRaiser}, BB, seats, 0, opts)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func exchange(t *testing.T, h *Hand, i int, discards string) {
	t.Helper()
	var cs []cards.Card
	if discards != "" {
		cs = cards.MustParseList(discards)
	}
	if err := h.Apply(i, Action{Type: Draw, Cards: cs}); err != nil {
		t.Fatalf("player %d exchanges %q: %v", i, discards, err)
	}
}

func hasCards(t *testing.T, p *Player, want string) {
	t.Helper()
	got, exp := slices.Clone(p.Hole), cards.MustParseList(want)
	slices.Sort(got)
	slices.Sort(exp)
	if !slices.Equal(got, exp) {
		t.Fatalf("holds %v, want %v", p.Hole, exp)
	}
}

func TestBBHandFromDealToShowdown(t *testing.T) {
	thousand := []int64{1000, 1000, 1000}
	h := bbHand(t, thousand, "As Ks Qs Js 2c  2d 3d 4d 5d 9h  7c 7d 7h 8c 8d  6d Ts")
	if len(h.Boards) != 0 || len(h.Players[0].Hole) != 5 || h.ToAct != 0 {
		t.Fatalf("deal: boards %d, hole %d, to act %d", len(h.Boards), len(h.Players[0].Hole), h.ToAct)
	}

	// Round 1 of betting, then the 1st exchange: everyone at once.
	play(t, h, "0 call", "1 call", "2 check")
	if !h.Drawing || h.ToAct != -1 || !slices.Equal(h.DrawPending(), []int{0, 1, 2}) {
		t.Fatalf("1st exchange: drawing %v, to act %d, pending %v", h.Drawing, h.ToAct, h.DrawPending())
	}
	if l := h.Legal(1); !l.Draw || l.Fold || l.Check {
		t.Fatalf("during an exchange the only action is to exchange: %+v", l)
	}
	exchange(t, h, 2, "") // keeps the full house
	exchange(t, h, 0, "2c")
	if !h.Drawing {
		t.Fatal("still waiting for player 1")
	}
	exchange(t, h, 1, "9h")
	// Replacements go in seat order from the dealer's left: 1 gets 6d, 0 Ts.
	hasCards(t, h.Players[1], "2d 3d 4d 5d 6d")
	hasCards(t, h.Players[0], "As Ks Qs Js Ts")
	if h.Players[0].Draws[0] != 1 || h.Players[2].Draws[0] != 0 || h.DrawRound != 1 {
		t.Fatalf("draw counts %v %v, round %d", h.Players[0].Draws, h.Players[2].Draws, h.DrawRound)
	}

	// Round 2: nobody raised in round 1, so the big blind starts.
	if h.Street != Flop || h.ToAct != 2 {
		t.Fatalf("round 2 starts with %d, want 2", h.ToAct)
	}
	play(t, h, "2 bet 40", "0 raise 120", "1 call", "2 call")
	exchange(t, h, 0, "")
	exchange(t, h, 1, "")
	exchange(t, h, 2, "")
	// Round 3 starts with round 2's raiser.
	if h.Street != Turn || h.ToAct != 0 {
		t.Fatalf("round 3 starts with %d, want 0", h.ToAct)
	}
	play(t, h, "0 check", "1 check", "2 check")
	for i := range 3 {
		exchange(t, h, i, "")
	}
	// Round 4, the last: then the showdown. The royal flush wins.
	if h.Street != River || h.DrawRound != 3 {
		t.Fatalf("street %v after %d exchanges", h.Street, h.DrawRound)
	}
	play(t, h, "2 check", "0 check", "1 check")
	if !h.Done || h.Players[0].Stack != 1000-140+420 {
		t.Fatalf("done %v, winner's stack %d", h.Done, h.Players[0].Stack)
	}
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}

	// Everyone sees how many each player exchanged; only you see your cards.
	theirs := strings.Join(h.NarrateFor(2), "\n")
	mine := strings.Join(h.NarrateFor(1), "\n")
	if !strings.Contains(theirs, "P0 exchanges 1") || !strings.Contains(theirs, "P2 keeps their cards") || strings.Contains(theirs, "gives back") {
		t.Fatalf("player 2's log:\n%s", theirs)
	}
	if !strings.Contains(mine, "P1 gives back 9♥ and gets 6♦") || strings.Contains(mine, "P0 gives back") {
		t.Fatalf("player 1's log:\n%s", mine)
	}
	if _, err := Replay(h.Log()); err != nil {
		t.Fatal(err)
	}
}

func TestBBFoldedCardsGoBackIntoTheDeck(t *testing.T) {
	// Only the hole cards are in the deck: the exchange has to come from
	// the folded hand.
	h := bbHand(t, []int64{1000, 1000, 1000}, "2c 3c 4c 5c 7d  9s 9h 9d Kc Kd  Ah Ad Ac As Kh")
	play(t, h, "0 call", "1 call", "2 fold")
	exchange(t, h, 0, "2c 3c 4c 5c 7d")
	exchange(t, h, 1, "")
	hasCards(t, h.Players[0], "Ah Ad Ac As Kh")
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}
}

func TestBBAllInPlayersGetOneMoreExchange(t *testing.T) {
	// Player 1 (small blind) has 50: all-in in round 1, they still take
	// the 1st exchange but not the 2nd or 3rd. Player 2 has 600 and goes
	// all-in in round 2.
	h := bbHand(t, []int64{1000, 50, 600}, "")
	play(t, h, "0 raise 100", "1 call", "2 call")
	if !h.Players[1].AllIn || !slices.Contains(h.DrawPending(), 1) {
		t.Fatalf("all-in %v, pending %v", h.Players[1].AllIn, h.DrawPending())
	}
	for _, i := range h.DrawPending() {
		exchange(t, h, i, "")
	}
	// Round 2: player 2 goes all-in.
	if h.ToAct != 0 {
		t.Fatalf("round 2 starts with %d", h.ToAct)
	}
	play(t, h, "0 check", "2 bet 500", "0 call")
	if !h.Players[2].AllIn || h.Players[0].AllIn {
		t.Fatal("player 2 should be all-in, player 0 not")
	}
	// 2nd exchange: player 2 (just all-in) draws; player 1 doesn't.
	if got := h.DrawPending(); !slices.Equal(got, []int{0, 2}) {
		t.Fatalf("2nd exchange pending %v, want [0 2]", got)
	}
	exchange(t, h, 0, "")
	exchange(t, h, 2, "")
	// Nobody can bet any more, but player 0 still has exchanges: the 3rd
	// is player 0's alone.
	if got := h.DrawPending(); !slices.Equal(got, []int{0}) {
		t.Fatalf("3rd exchange pending %v, want [0]", got)
	}
	exchange(t, h, 0, "")
	if !h.Done {
		t.Fatal("with no betting left, the hand goes to the showdown")
	}
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}
}

func TestBBExchangeMistakes(t *testing.T) {
	h := bbHand(t, []int64{1000, 1000}, "2c 3c 4c 5c 7d  9s 9h 9d Kc Kd  Ah")
	play(t, h, "0 call", "1 check")
	for name, cs := range map[string][]cards.Card{
		"a card you don't hold": cards.MustParseList("Ah"),
		"the same card twice":   cards.MustParseList("2c 2c"),
	} {
		if err := h.Apply(0, Action{Type: Draw, Cards: cs}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := h.Apply(0, Action{Type: Check}); err == nil {
		t.Error("betting during an exchange: accepted")
	}
	exchange(t, h, 0, "")
	if err := h.Apply(0, Action{Type: Draw}); err == nil {
		t.Error("exchanging twice: accepted")
	}
	if got := h.Hint(1); got == "" {
		t.Error("BB hands can be described from the deal")
	}
}
