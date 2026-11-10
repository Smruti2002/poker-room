package engine

import (
	"testing"

	"pokerroom/internal/cards"
)

// orderHand starts a hand with the house betting order: 4 players, seat 0
// deals, 1 is the small blind, 2 the big blind, 3 acts first pre-flop.
func orderHand(t *testing.T, stacks ...int64) *Hand {
	t.Helper()
	if stacks == nil {
		stacks = []int64{1000, 1000, 1000, 1000}
	}
	seats := make([]SeatSpec, len(stacks))
	for i, s := range stacks {
		seats[i] = SeatSpec{Stack: s}
	}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20, Order: OrderLastRaiser}, Holdem, seats, 0, Options{Seed: cards.SeedFromUint64(1)})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func play(t *testing.T, h *Hand, actions ...string) {
	t.Helper()
	for _, a := range actions {
		p, act := parseAction(t, a)
		if err := h.Apply(p, act); err != nil {
			t.Fatalf("%q: %v", a, err)
		}
	}
}

func TestHouseBettingOrder(t *testing.T) {
	t.Run("pre-flop is unchanged: left of the big blind starts", func(t *testing.T) {
		if h := orderHand(t); h.ToAct != 3 {
			t.Fatalf("first to act %d, want 3", h.ToAct)
		}
	})

	t.Run("the last raiser of the previous round starts the next", func(t *testing.T) {
		h := orderHand(t)
		play(t, h, "3 call", "0 raise 60", "1 call", "2 call", "3 call")
		if h.Street != Flop || h.ToAct != 0 {
			t.Fatalf("flop starts with %d, want 0 (the pre-flop raiser)", h.ToAct)
		}
		// On the flop, 2 bets and 1 raises: 1 starts the turn.
		play(t, h, "0 check", "1 check", "2 bet 40", "3 call", "0 call", "1 raise 120", "2 call", "3 call", "0 call")
		if h.Street != Turn || h.ToAct != 1 {
			t.Fatalf("turn starts with %d, want 1 (the flop's last raiser)", h.ToAct)
		}
	})

	t.Run("nobody raised: the big blind starts", func(t *testing.T) {
		h := orderHand(t)
		play(t, h, "3 call", "0 call", "1 call", "2 check")
		if h.Street != Flop || h.ToAct != 2 {
			t.Fatalf("flop starts with %d, want 2 (the big blind)", h.ToAct)
		}
		// Everyone checks the flop: the big blind starts the turn too.
		play(t, h, "2 check", "3 check", "0 check", "1 check")
		if h.Street != Turn || h.ToAct != 2 {
			t.Fatalf("turn starts with %d, want 2", h.ToAct)
		}
	})

	t.Run("a starter who can't act passes it to the next player who can", func(t *testing.T) {
		// The big blind folds (allowed even when checking is free): the
		// flop starts with the next player still in, seat 3.
		h := orderHand(t)
		play(t, h, "3 call", "0 call", "1 call", "2 fold")
		if h.ToAct != 3 {
			t.Fatalf("flop starts with %d, want 3", h.ToAct)
		}
		// Seat 3 raises all-in on the flop: the turn starts with seat 0.
		h = orderHand(t, 1000, 1000, 1000, 300)
		play(t, h, "3 call", "0 call", "1 call", "2 check", "2 check", "3 bet 280", "0 call", "1 call", "2 call")
		if !h.Players[3].AllIn || h.Street != Turn || h.ToAct != 0 {
			t.Fatalf("turn starts with %d, want 0 (after the all-in raiser)", h.ToAct)
		}
	})

	t.Run("hands logged with the classic order still replay with it", func(t *testing.T) {
		h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20}, Holdem, []SeatSpec{{Stack: 1000}, {Stack: 1000}, {Stack: 1000}, {Stack: 1000}}, 0, Options{Seed: cards.SeedFromUint64(1)})
		if err != nil {
			t.Fatal(err)
		}
		play(t, h, "3 call", "0 raise 60", "1 call", "2 call", "3 call")
		if h.ToAct != 1 {
			t.Fatalf("classic order: flop starts with %d, want 1 (left of the dealer)", h.ToAct)
		}
		if _, err := Replay(h.Log()); err != nil {
			t.Fatal(err)
		}
	})
}
