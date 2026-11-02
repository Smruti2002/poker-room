package engine

import (
	"slices"
)

// Pot is an amount of chips and the players who can win it.
type Pot struct {
	Chips    int64 `json:"chips"`
	Eligible []int `json:"eligible"` // player indices
}

// buildPots splits everyone's contributions into a main pot and side pots.
//
// Contributions are in units; each player pays Mult chips per unit. Levels
// are the distinct totals of players still in the hand. For each level, every
// player contributes the units between the previous level and this one, and
// players who reached the level (and haven't folded) are eligible.
// See docs/rules.md §4.3 example 4.3-b.
func buildPots(players []*Player) []Pot {
	var levels []int64
	for _, p := range players {
		if !p.Folded && p.Total > 0 && !slices.Contains(levels, p.Total) {
			levels = append(levels, p.Total)
		}
	}
	slices.Sort(levels)

	var pots []Pot
	prev := int64(0)
	for _, level := range levels {
		pot := Pot{}
		for i, p := range players {
			pot.Chips += p.Mult * clamp(p.Total-prev, 0, level-prev)
			if !p.Folded && p.Total >= level {
				pot.Eligible = append(pot.Eligible, i)
			}
		}
		// Merge with the previous pot when the same players can win both.
		if n := len(pots); n > 0 && slices.Equal(pots[n-1].Eligible, pot.Eligible) {
			pots[n-1].Chips += pot.Chips
		} else {
			pots = append(pots, pot)
		}
		prev = level
	}

	// Folded players can't have put in more than the top live player once
	// uncalled bets are returned, but keep any excess in the last pot so
	// chips are never lost.
	for _, p := range players {
		if extra := p.Mult * max(p.Total-prev, 0); extra > 0 && len(pots) > 0 {
			pots[len(pots)-1].Chips += extra
		}
	}
	return pots
}

// splitPot divides chips among winners. Leftover chips go one at a time to
// winners in seat order starting left of the dealer.
func splitPot(chips int64, winners []int, dealer, numPlayers int) map[int]int64 {
	order := slices.Clone(winners)
	slices.SortFunc(order, func(a, b int) int {
		return seatDistance(dealer, a, numPlayers) - seatDistance(dealer, b, numPlayers)
	})
	share := chips / int64(len(order))
	odd := chips % int64(len(order))
	out := make(map[int]int64, len(order))
	for i, w := range order {
		out[w] = share
		if int64(i) < odd {
			out[w]++
		}
	}
	return out
}

// seatDistance is how many seats clockwise from the dealer seat i is; the
// player left of the dealer is 1 and the dealer is numPlayers.
func seatDistance(dealer, i, numPlayers int) int {
	d := (i - dealer + numPlayers) % numPlayers
	if d == 0 {
		return numPlayers
	}
	return d
}

func clamp(x, lo, hi int64) int64 {
	return max(lo, min(x, hi))
}
