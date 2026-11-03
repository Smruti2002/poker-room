package engine

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"pokerroom/internal/cards"
)

// genHand draws a random table: 2-10 players, stacks from tiny to deep,
// some players paying double, random blinds, structure and dealer.
func genHand(t *rapid.T) *Hand {
	n := rapid.IntRange(2, MaxPlayers).Draw(t, "players")
	bb := rapid.Int64Range(2, 100).Draw(t, "bigBlind")
	cfg := Config{
		SmallBlind: rapid.Int64Range(1, bb).Draw(t, "smallBlind"),
		BigBlind:   bb,
		Structure:  rapid.SampledFrom([]Structure{NoLimit, PotLimit}).Draw(t, "structure"),
	}
	seats := make([]SeatSpec, n)
	for i := range seats {
		mult := rapid.SampledFrom([]int64{1, 1, 1, 2}).Draw(t, fmt.Sprintf("mult%d", i))
		// Mix very short stacks (all-in on the blinds) with deep ones.
		stack := rapid.OneOf(
			rapid.Int64Range(mult, 3*bb*mult),
			rapid.Int64Range(mult, 200*bb),
		).Draw(t, fmt.Sprintf("stack%d", i))
		seats[i] = SeatSpec{ID: fmt.Sprintf("P%d", i), Stack: stack, Mult: mult}
	}
	dealer := rapid.IntRange(0, n-1).Draw(t, "dealer")
	seed := cards.SeedFromUint64(rapid.Uint64().Draw(t, "seed"))

	v := rapid.SampledFrom([]Variant{Holdem, Omaha, ShowTwo, Dacoit}).Draw(t, "variant")
	h, err := NewHand(cfg, v, seats, dealer, Options{Seed: seed})
	if err != nil {
		t.Fatalf("NewHand: %v", err)
	}
	return h
}

// drawAction picks a random legal action for the player to act.
func drawAction(t *rapid.T, h *Hand) Action {
	l := h.Legal(h.ToAct)
	if l.Choose {
		return Action{Type: Choose, Count: rapid.SampledFrom([]int{4, l.ChooseMax}).Draw(t, "cards")}
	}
	if l.Show {
		// Reveal a random pair of hole cards.
		hole := h.Players[h.ToAct].Hole
		perm := rapid.Permutation(hole).Draw(t, "reveal")
		return Action{Type: Show, Cards: perm[:h.Variant.Reveal()]}
	}
	var choices []ActionType
	if l.Fold {
		choices = append(choices, Fold)
	}
	if l.Check {
		choices = append(choices, Check)
	}
	if l.Call {
		choices = append(choices, Call, Call) // weight calls so hands reach showdown
	}
	if l.Bet {
		choices = append(choices, Bet)
	}
	if l.Raise {
		choices = append(choices, Raise)
	}
	a := Action{Type: rapid.SampledFrom(choices).Draw(t, "action")}
	if a.Type == Bet || a.Type == Raise {
		a.To = rapid.OneOf(
			rapid.Just(l.MinTo),
			rapid.Just(l.MaxTo),
			rapid.Int64Range(l.MinTo, l.MaxTo),
		).Draw(t, "to")
	}
	return a
}

func TestPropertyRandomHands(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		h := genHand(t)
		check := func() {
			if err := h.CheckInvariants(); err != nil {
				t.Fatalf("%v\n--- hand ---\n%s", err, strings.Join(h.Narrate(), "\n"))
			}
		}
		check()

		// Every hand ends: each action either closes the round or moves to
		// another player, so a generous bound catches infinite loops.
		for steps := 0; !h.Done; steps++ {
			if steps > 500 {
				t.Fatalf("hand did not finish in 500 actions\n%s", strings.Join(h.Narrate(), "\n"))
			}
			a := drawAction(t, h)
			if err := h.Apply(h.ToAct, a); err != nil {
				t.Fatalf("legal action %v rejected: %v", a, err)
			}
			check()
		}

		// Only showdown or a fold leaves the hand; folded players win nothing.
		live := 0
		for _, p := range h.Players {
			if !p.Folded {
				live++
			}
		}
		if live > 1 {
			for _, b := range h.Boards {
				if len(b) != 5 {
					t.Fatalf("showdown with %d board cards", len(b))
				}
			}
		}

		// Replaying the log reproduces the hand exactly.
		r, err := Replay(h.Log())
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		if !slices.Equal(stacksOf(r), stacksOf(h)) {
			t.Fatalf("replay stacks %v != %v", stacksOf(r), stacksOf(h))
		}
		if !slices.Equal(r.Narrate(), h.Narrate()) {
			t.Fatalf("replay events differ:\n%s\n---\n%s", strings.Join(r.Narrate(), "\n"), strings.Join(h.Narrate(), "\n"))
		}
	})
}

// Illegal amounts are always rejected and leave the hand untouched.
func TestPropertyIllegalAmountsRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		h := genHand(t)
		if h.Done {
			return
		}
		l := h.Legal(h.ToAct)
		if !l.Bet && !l.Raise {
			return
		}
		typ := Raise
		if l.Bet {
			typ = Bet
		}
		bad := rapid.OneOf(
			rapid.Int64Range(-10, l.MinTo-1),
			rapid.Int64Range(l.MaxTo+1, l.MaxTo+1000),
		).Draw(t, "badTo")
		before := stacksOf(h)
		if err := h.Apply(h.ToAct, Action{Type: typ, To: bad}); err == nil {
			t.Fatalf("%v to %d accepted; legal %+v", typ, bad, l)
		}
		if !slices.Equal(before, stacksOf(h)) {
			t.Fatal("rejected action changed stacks")
		}
	})
}
