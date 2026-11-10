// Package bot decides moves for computer players. A bot sees only what its
// seat may see (View): its own cards, the boards, cards others have shown,
// and the betting. Its play is simple but sensible: it bets strong hands,
// calls when the price is right, folds weak ones, and bluffs now and then.
package bot

import (
	"math/rand/v2"
	"slices"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
	"pokerroom/internal/eval"
)

// View is everything a bot may know when it's asked to act.
type View struct {
	Variant string
	Legal   engine.Legal
	Hole    []cards.Card   // its own cards
	Shown   []cards.Card   // which of them it has shown (Show 2 Play 2)
	Boards  [][]cards.Card // the boards so far
	Pot     int64          // chips in the pot
	Stack   int64          // its chips behind
	Mult    int64          // its chips per unit (2 for a 6-card Dacoit player)
	Live    int            // players still in the hand, itself included
	Reveal  int            // cards to show (Show 2 Play 2)
	Split   func() engine.Action
}

// Decide picks a legal move.
func Decide(v View, r *rand.Rand) engine.Action {
	l := v.Legal
	switch {
	case l.Choose:
		// Dacoit: usually 4 cards; sometimes the 6-card gamble.
		if l.ChooseMax > 4 && r.IntN(5) == 0 {
			return engine.Action{Type: engine.Choose, Count: l.ChooseMax}
		}
		return engine.Action{Type: engine.Choose, Count: 4}
	case l.Show:
		return engine.Action{Type: engine.Show, Cards: showCards(v)}
	case l.Allocate:
		return v.Split()
	case l.Draw:
		return engine.Action{Type: engine.Draw, Cards: discards(v.Hole)}
	}
	return bet(v, strength(v), r)
}

// bet chooses a betting move from the hand's strength (0 weakest, 1 best).
func bet(v View, s float64, r *rand.Rand) engine.Action {
	l := v.Legal
	roll := r.Float64()
	size := func(fraction float64) engine.Action {
		typ := engine.Raise
		if l.Bet {
			typ = engine.Bet
		}
		// Aim for a fraction of the pot (in chips), in this bot's units.
		to := l.MinTo + int64(fraction*float64(v.Pot))/max(v.Mult, 1)
		return engine.Action{Type: typ, To: min(max(to, l.MinTo), l.MaxTo)}
	}
	canRaise := l.Bet || l.Raise
	if l.Check {
		switch {
		case canRaise && s > 0.75 && roll < 0.75:
			return size(0.5 + roll/2)
		case canRaise && s > 0.55 && roll < 0.35:
			return size(0.4)
		case canRaise && roll < 0.05: // a bluff
			return size(0.5)
		}
		return engine.Action{Type: engine.Check}
	}
	// Facing a bet: what share of the pot would calling cost?
	toCall := l.CallUnits * max(v.Mult, 1)
	price := float64(toCall) / float64(v.Pot+toCall)
	switch {
	case canRaise && s > 0.85 && roll < 0.6:
		return size(0.75)
	case s > price+0.15, s > 0.8:
		return engine.Action{Type: engine.Call}
	case roll < 0.04: // a hero call
		return engine.Action{Type: engine.Call}
	}
	return engine.Action{Type: engine.Fold}
}

// strength rates the bot's hand from 0 to 1, with a small discount for
// each extra opponent.
func strength(v View) float64 {
	s := madeHand(v)
	if s < 0 {
		s = startingHand(v.Hole)
	}
	return max(0, s-0.03*float64(max(v.Live-2, 0)))
}

// madeHand rates the best hand the bot can make now, or -1 before there's
// enough board to judge.
func madeHand(v View) float64 {
	if v.Variant == "bb" {
		h, err := eval.BestOf(eval.Group{Cards: v.Hole, Take: 5})
		if err != nil {
			return -1
		}
		return category(h.Rank)
	}
	if len(v.Boards) == 0 || len(v.Boards[0]) < 3 {
		return -1
	}
	total := 0.0
	for _, b := range v.Boards {
		var h eval.Hand
		var err error
		switch v.Variant {
		case "omaha", "dacoit":
			h, err = eval.Omaha(v.Hole, b)
		case "double", "triple":
			h, err = eval.BestOf(eval.Group{Cards: v.Hole, Take: 2}, eval.Group{Cards: b, Take: 3})
		case "show2":
			if len(v.Shown) == 0 {
				return -1
			}
			hidden := slices.DeleteFunc(slices.Clone(v.Hole), func(c cards.Card) bool { return slices.Contains(v.Shown, c) })
			h, err = eval.ShowTwo(v.Shown, hidden, b)
		default:
			h, err = eval.Holdem(v.Hole, b)
		}
		if err != nil {
			return -1
		}
		total += category(h.Rank)
	}
	return total / float64(len(v.Boards))
}

// category maps a hand to a strength, with its top card nudging it up.
func category(h eval.HandRank) float64 {
	base := [...]float64{0.12, 0.4, 0.6, 0.72, 0.8, 0.84, 0.92, 0.97, 1}[h.Category()]
	return min(1, base+float64(h.TieBreak(0))/200)
}

// startingHand rates hole cards before there's a board: the best pair of
// cards in it, counting pairs, high cards and suits.
func startingHand(hole []cards.Card) float64 {
	best := 0.0
	for i := range hole {
		for j := i + 1; j < len(hole); j++ {
			a, b := hole[i], hole[j]
			s := float64(a.Rank()+b.Rank()) / 56 // up to 0.5 for A-A
			if a.Rank() == b.Rank() {
				s += 0.3
			}
			if a.Suit() == b.Suit() {
				s += 0.05
			}
			best = max(best, s)
		}
	}
	return min(best, 0.95)
}

// showCards picks what to show in Show 2 Play 2: the two cards that pair
// up best with the rest stay hidden, so it shows the others' lowest.
func showCards(v View) []cards.Card {
	hole := slices.Clone(v.Hole)
	slices.SortFunc(hole, func(a, b cards.Card) int { return int(a.Rank()) - int(b.Rank()) })
	return hole[:v.Reveal]
}

// discards picks what to give back in a BB exchange: keep anything that's
// part of a made hand or four to a flush, and otherwise the two highest.
func discards(hole []cards.Card) []cards.Card {
	h, err := eval.BestOf(eval.Group{Cards: hole, Take: 5})
	if err != nil || h.Rank.Category() >= eval.Straight {
		return nil // keep a straight or better
	}
	count := map[cards.Rank]int{}
	suits := map[cards.Suit]int{}
	for _, c := range hole {
		count[c.Rank()]++
		suits[c.Suit()]++
	}
	keep := func(c cards.Card) bool { return count[c.Rank()] >= 2 }
	if !slices.ContainsFunc(hole, keep) {
		// No pair: chase four to a flush, else keep the two highest.
		for s, n := range suits {
			if n == 4 {
				keep = func(c cards.Card) bool { return c.Suit() == s }
			}
		}
		if !slices.ContainsFunc(hole, keep) {
			sorted := slices.Clone(hole)
			slices.SortFunc(sorted, func(a, b cards.Card) int { return int(b.Rank()) - int(a.Rank()) })
			top := sorted[:2]
			keep = func(c cards.Card) bool { return slices.Contains(top, c) }
		}
	}
	var out []cards.Card
	for _, c := range hole {
		if !keep(c) {
			out = append(out, c)
		}
	}
	return out
}
