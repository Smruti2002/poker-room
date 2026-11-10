package bot

import (
	"math/rand/v2"
	"slices"
	"testing"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
)

// view is what a seat may see, built the way the room builds it.
func view(h *engine.Hand, i int) View {
	p := h.Players[i]
	live := 0
	for _, q := range h.Players {
		if !q.Folded {
			live++
		}
	}
	return View{
		Variant: h.Variant.Name(), Legal: h.Legal(i), Hole: p.Hole, Shown: p.Shown, Boards: h.Boards,
		Pot: h.PotChips(), Stack: p.Stack, Mult: p.Mult, Live: live, Reveal: h.Variant.Reveal(),
		Split: func() engine.Action { return h.DefaultSplit(i) },
	}
}

// Bots only ever make legal moves, and hands of every variant finish.
func TestBotsPlayEveryVariantLegally(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for name, v := range engine.AllVariants() {
		for n := range 300 {
			players := 2 + n%5
			if name == "triple" {
				players = min(players, 6)
			}
			seats := make([]engine.SeatSpec, players)
			for i := range seats {
				seats[i] = engine.SeatSpec{Stack: int64(100 + r.IntN(3000))}
			}
			h, err := engine.NewHand(engine.Config{SmallBlind: 10, BigBlind: 20, Order: engine.OrderLastRaiser}, v, seats, n%players, engine.Options{Seed: cards.SeedFromUint64(uint64(n))})
			if err != nil {
				t.Fatal(err)
			}
			for steps := 0; !h.Done; steps++ {
				if steps > 500 {
					t.Fatalf("%s hand %d never finished", name, n)
				}
				i := h.ToAct
				if pending := append(h.Pending(), h.DrawPending()...); len(pending) > 0 {
					i = pending[0]
				}
				a := Decide(view(h, i), r)
				if err := h.Apply(i, a); err != nil {
					t.Fatalf("%s hand %d: bot's %v rejected: %v", name, n, a, err)
				}
			}
			if err := h.CheckInvariants(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestBBExchangeChoices(t *testing.T) {
	for hole, want := range map[string]string{
		"Ah Kh Qh Jh Th": "",         // a straight flush: keep everything
		"9c 9d 2s 5h Kd": "2s 5h Kd", // a pair: keep the pair
		"9c 9d 9s 5h Kd": "5h Kd",    // three of a kind
		"9c 9d 5s 5h Kd": "Kd",       // two pair
		"2h 7h 9h Jh Ks": "Ks",       // four to a flush
		"2c 7d 9h Js Ks": "2c 7d 9h", // nothing: keep the two highest
	} {
		got := discards(cards.MustParseList(hole))
		exp := cards.MustParseList(want)
		slices.Sort(got)
		slices.Sort(exp)
		if !slices.Equal(got, exp) {
			t.Errorf("%s: gives back %v, want %v", hole, got, exp)
		}
	}
}

func TestBotsBetStrongHandsAndFoldWeakOnes(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	nuts := View{Variant: "holdem", Hole: cards.MustParseList("As Ah"), Boards: [][]cards.Card{cards.MustParseList("Ad Ac Kd")},
		Pot: 100, Mult: 1, Live: 2, Legal: engine.Legal{Fold: true, Check: true, Bet: true, MinTo: 20, MaxTo: 1000}}
	trash := View{Variant: "holdem", Hole: cards.MustParseList("2s 7h"), Boards: [][]cards.Card{cards.MustParseList("Ad Kc Qd")},
		Pot: 100, Mult: 1, Live: 2, Legal: engine.Legal{Fold: true, Call: true, CallUnits: 500, Raise: true, MinTo: 1000, MaxTo: 1000}}
	bets, folds := 0, 0
	for range 200 {
		if a := Decide(nuts, r); a.Type == engine.Bet {
			bets++
		}
		if a := Decide(trash, r); a.Type == engine.Fold {
			folds++
		}
	}
	if bets < 100 || folds < 180 {
		t.Fatalf("four aces bet %d/200 times; 7-2 facing a big bet folded %d/200", bets, folds)
	}
}
