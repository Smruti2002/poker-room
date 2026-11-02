package eval

import (
	"errors"
	"fmt"

	"pokerroom/internal/cards"
)

// Group is a pool of cards from which exactly Take cards must be used.
type Group struct {
	Cards []cards.Card
	Take  int
}

// Hand is the best five-card hand found and the cards that make it.
type Hand struct {
	Rank  HandRank
	Cards [5]cards.Card
}

// ErrDuplicateCard is returned when the same card appears twice in the input.
var ErrDuplicateCard = errors.New("duplicate card")

// BestOf returns the strongest five-card hand that uses exactly g.Take cards
// from each group g. The Take values must sum to 5.
func BestOf(groups ...Group) (Hand, error) {
	total := 0
	var seen cards.Set
	for i, g := range groups {
		if g.Take < 0 || g.Take > len(g.Cards) {
			return Hand{}, fmt.Errorf("group %d: cannot take %d of %d cards", i, g.Take, len(g.Cards))
		}
		total += g.Take
		for _, c := range g.Cards {
			if !c.Valid() {
				return Hand{}, fmt.Errorf("group %d: invalid card %d", i, c)
			}
			if !seen.Add(c) {
				return Hand{}, fmt.Errorf("%w: %v", ErrDuplicateCard, c)
			}
		}
	}
	if total != 5 {
		return Hand{}, fmt.Errorf("groups take %d cards, need 5", total)
	}

	s := search{groups: groups}
	s.pick(-1, 0, 0, 0)
	return s.best, nil
}

type search struct {
	groups []Group
	buf    [5]cards.Card
	best   Hand
	found  bool
}

// pick fills buf[filled:]. groupEnd is the buf index where groups[gi]'s
// picks end; the next pick from groups[gi] starts at card index `from`.
func (s *search) pick(gi, groupEnd, filled, from int) {
	if filled == 5 {
		if r := Eval5(s.buf); !s.found || r > s.best.Rank {
			s.best = Hand{Rank: r, Cards: s.buf}
			s.found = true
		}
		return
	}
	if filled == groupEnd { // advance to the next group with something to take
		for groupEnd == filled {
			gi++
			groupEnd += s.groups[gi].Take
		}
		from = 0
	}
	g := s.groups[gi]
	need := groupEnd - filled
	for i := from; i <= len(g.Cards)-need; i++ {
		s.buf[filled] = g.Cards[i]
		s.pick(gi, groupEnd, filled+1, i+1)
	}
}

// Holdem is the Texas Hold'em rule: any five of hole + board.
func Holdem(hole, board []cards.Card) (Hand, error) {
	all := make([]cards.Card, 0, len(hole)+len(board))
	all = append(append(all, hole...), board...)
	return BestOf(Group{Cards: all, Take: 5})
}

// Omaha is the Omaha and Dacoit rule: exactly 2 hole cards + 3 board cards.
func Omaha(hole, board []cards.Card) (Hand, error) {
	return BestOf(Group{Cards: hole, Take: 2}, Group{Cards: board, Take: 3})
}

// ShowTwo is the Show 2 Play 2 rule: exactly 1 shown + 1 hidden + 3 board.
func ShowTwo(shown, hidden, board []cards.Card) (Hand, error) {
	return BestOf(Group{Cards: shown, Take: 1}, Group{Cards: hidden, Take: 1}, Group{Cards: board, Take: 3})
}
