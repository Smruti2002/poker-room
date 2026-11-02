package eval

import (
	"slices"

	"pokerroom/internal/cards"
)

// refKey is a deliberately naive, independently written hand ranking used to
// cross-check Eval5. It compares (category, tie-breaks) lexicographically.
type refKey struct {
	cat  Category
	tieb []cards.Rank
}

func refCompare(a, b refKey) int {
	if a.cat != b.cat {
		return int(a.cat) - int(b.cat)
	}
	return slices.Compare(a.tieb, b.tieb)
}

func refEval(hand []cards.Card) refKey {
	counts := map[cards.Rank]int{}
	suits := map[cards.Suit]bool{}
	for _, c := range hand {
		counts[c.Rank()]++
		suits[c.Suit()] = true
	}
	flush := len(suits) == 1

	// Straight: look for five consecutive ranks, treating the ace as 1 too.
	has := func(r int) bool {
		if r == 1 {
			return counts[cards.Ace] > 0
		}
		return counts[cards.Rank(r)] > 0
	}
	straightTop := cards.Rank(0)
	for top := 14; top >= 5 && straightTop == 0; top-- {
		if has(top) && has(top-1) && has(top-2) && has(top-3) && has(top-4) {
			straightTop = cards.Rank(top)
		}
	}

	// ranksWith returns ranks appearing exactly n times, highest first.
	ranksWith := func(n int) []cards.Rank {
		var out []cards.Rank
		for r, c := range counts {
			if c == n {
				out = append(out, r)
			}
		}
		slices.Sort(out)
		slices.Reverse(out)
		return out
	}
	quads, trips, pairs, singles := ranksWith(4), ranksWith(3), ranksWith(2), ranksWith(1)

	switch {
	case straightTop != 0 && flush:
		return refKey{StraightFlush, []cards.Rank{straightTop}}
	case len(quads) == 1:
		return refKey{FourOfAKind, []cards.Rank{quads[0], singles[0]}}
	case len(trips) == 1 && len(pairs) == 1:
		return refKey{FullHouse, []cards.Rank{trips[0], pairs[0]}}
	case flush:
		return refKey{Flush, singles}
	case straightTop != 0:
		return refKey{Straight, []cards.Rank{straightTop}}
	case len(trips) == 1:
		return refKey{ThreeOfAKind, append([]cards.Rank{trips[0]}, singles...)}
	case len(pairs) == 2:
		return refKey{TwoPair, append(pairs, singles...)}
	case len(pairs) == 1:
		return refKey{OnePair, append([]cards.Rank{pairs[0]}, singles...)}
	default:
		return refKey{HighCard, singles}
	}
}

// refBest brute-forces every 5-card subset of all and returns the best key.
func refBest(all []cards.Card) refKey {
	var best refKey
	first := true
	n := len(all)
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			for c := b + 1; c < n; c++ {
				for d := c + 1; d < n; d++ {
					for e := d + 1; e < n; e++ {
						k := refEval([]cards.Card{all[a], all[b], all[c], all[d], all[e]})
						if first || refCompare(k, best) > 0 {
							best, first = k, false
						}
					}
				}
			}
		}
	}
	return best
}

// refOmaha brute-forces exactly 2 hole + 3 board with plain nested loops.
func refOmaha(hole, board []cards.Card) refKey {
	var best refKey
	first := true
	for i := 0; i < len(hole); i++ {
		for j := i + 1; j < len(hole); j++ {
			for a := 0; a < len(board); a++ {
				for b := a + 1; b < len(board); b++ {
					for c := b + 1; c < len(board); c++ {
						k := refEval([]cards.Card{hole[i], hole[j], board[a], board[b], board[c]})
						if first || refCompare(k, best) > 0 {
							best, first = k, false
						}
					}
				}
			}
		}
	}
	return best
}
