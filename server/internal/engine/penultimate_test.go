package engine

import (
	"testing"

	"pokerroom/internal/cards"
)

// penultimateHand deals three seats (0 deals, 1 small blind, 2 big blind)
// two cards each in seat order, then the board.
func penultimateHand(t *testing.T, stacks []int64, deck string) *Hand {
	t.Helper()
	seats := make([]SeatSpec, len(stacks))
	for i, s := range stacks {
		seats[i] = SeatSpec{Stack: s}
	}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20, Order: OrderLastRaiser}, Penultimate, seats, 0,
		Options{Seed: cards.SeedFromUint64(4), Deck: cards.MustParseList(deck)})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// callDown checks or calls until the hand ends.
func callDown(t *testing.T, h *Hand) {
	t.Helper()
	for !h.Done {
		a := Action{Type: Check}
		if h.Legal(h.ToAct).Call {
			a.Type = Call
		}
		if err := h.Apply(h.ToAct, a); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}
}

func won(h *Hand) []int64 {
	out := make([]int64, len(h.Players))
	for i, p := range h.Players {
		out[i] = p.Won
	}
	return out
}

const board = "2c 7d 9h Jc 3s"

func TestPenultimateSecondBestWins(t *testing.T) {
	// Ann: kings. Bob: aces (the best). Cy: fives. Kings are second best.
	h := penultimateHand(t, []int64{1000, 1000, 1000}, "Kh Kd  Ah Ad  5h 5d  "+board)
	callDown(t, h)
	if w := won(h); w[0] != 60 || w[1] != 0 || w[2] != 0 {
		t.Fatalf("won %v, want Ann (second best) to take 60", w)
	}
	if bw := h.BoardWinners(); len(bw) != 1 || len(bw[0]) != 1 || bw[0][0] != 0 {
		t.Fatalf("board winners %v", bw)
	}
}

// Each pot is judged among its own players: heads-up, the weaker hand wins.
func TestPenultimateSidePots(t *testing.T) {
	// Bob (aces, the best) is all-in for 100. Main pot, all three: Ann's
	// kings are second best. Side pot, Ann and Cy only: Cy's fives.
	h := penultimateHand(t, []int64{1000, 100, 1000}, "Kh Kd  Ah Ad  5h 5d  "+board)
	for _, step := range []struct {
		player int
		a      Action
	}{
		{0, Action{Type: Raise, To: 500}},
		{1, Action{Type: Call}},
		{2, Action{Type: Call}},
	} {
		if err := h.Apply(step.player, step.a); err != nil {
			t.Fatal(err)
		}
	}
	callDown(t, h)
	if w := won(h); w[0] != 300 || w[1] != 0 || w[2] != 800 {
		t.Fatalf("won %v, want Ann 300 (main) and Cy 800 (side)", w)
	}
}

// Two players tie for the best: the next hand down wins.
func TestPenultimateTiedBestLoses(t *testing.T) {
	h := penultimateHand(t, []int64{1000, 1000, 1000}, "Ah Ad  As Ac  Kh Kd  "+board)
	callDown(t, h)
	if w := won(h); w[0] != 0 || w[1] != 0 || w[2] != 60 {
		t.Fatalf("won %v, want Cy (kings) to take 60", w)
	}
}

// Everyone ties: there's no second-best hand, so they split.
func TestPenultimateEveryoneTies(t *testing.T) {
	h := penultimateHand(t, []int64{1000, 1000, 1000}, "2c 3d  4c 5d  6c 7c  Ah Kh Qh Jh Th")
	callDown(t, h)
	if w := won(h); w[0] != 20 || w[1] != 20 || w[2] != 20 {
		t.Fatalf("won %v, want a three-way split", w)
	}
}

// Everyone folding to one player works as usual.
func TestPenultimateFoldWin(t *testing.T) {
	h := penultimateHand(t, []int64{1000, 1000, 1000}, "Kh Kd  Ah Ad  5h 5d  "+board)
	for !h.Done {
		if err := h.Apply(h.ToAct, Action{Type: Fold}); err != nil {
			t.Fatal(err)
		}
	}
	// The big blind wins the pot (and its unmatched 10 comes back).
	if w := won(h); w[0] != 0 || w[1] != 0 || w[2] != 20 {
		t.Fatalf("won %v", w)
	}
}
