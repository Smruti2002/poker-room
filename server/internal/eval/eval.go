// Package eval ranks poker hands.
//
// Eval5 ranks exactly five cards. BestOf finds the strongest five-card hand
// under a variant's selection rule, e.g. Omaha's "exactly 2 hole + 3 board".
// Every variant in docs/rules.md is expressed as a list of Groups.
package eval

import (
	"fmt"
	"math/bits"

	"pokerroom/internal/cards"
)

// Category is the type of a five-card hand, weakest first.
type Category uint8

const (
	HighCard Category = iota
	OnePair
	TwoPair
	ThreeOfAKind
	Straight
	Flush
	FullHouse
	FourOfAKind
	StraightFlush
)

var categoryNames = [...]string{
	"High card", "One pair", "Two pair", "Three of a kind", "Straight",
	"Flush", "Full house", "Four of a kind", "Straight flush",
}

func (c Category) String() string {
	if int(c) < len(categoryNames) {
		return categoryNames[c]
	}
	return fmt.Sprintf("Category(%d)", c)
}

// HandRank orders five-card hands: a larger value is a stronger hand and
// equal values tie. Layout: category in bits 20-23, then up to five
// tie-break ranks of 4 bits each, most significant first.
type HandRank uint32

// Category returns the hand's category.
func (h HandRank) Category() Category { return Category(h >> 20) }

// TieBreak returns the i-th tie-break rank (0 = most significant).
func (h HandRank) TieBreak(i int) cards.Rank {
	return cards.Rank(h >> (16 - 4*i) & 0xF)
}

// String describes the hand, e.g. "Full house, K over A".
func (h HandRank) String() string {
	r := func(i int) string { return rankName(h.TieBreak(i)) }
	switch h.Category() {
	case StraightFlush:
		if h.TieBreak(0) == cards.Ace {
			return "Royal flush"
		}
		return "Straight flush, " + r(0) + " high"
	case FourOfAKind:
		return "Four of a kind, " + r(0) + "s"
	case FullHouse:
		return "Full house, " + r(0) + "s full of " + r(1) + "s"
	case Flush:
		return "Flush, " + r(0) + " high"
	case Straight:
		return "Straight, " + r(0) + " high"
	case ThreeOfAKind:
		return "Three of a kind, " + r(0) + "s"
	case TwoPair:
		return "Two pair, " + r(0) + "s and " + r(1) + "s"
	case OnePair:
		return "Pair of " + r(0) + "s"
	default:
		return r(0) + " high"
	}
}

func rankName(r cards.Rank) string {
	names := [...]string{"Two", "Three", "Four", "Five", "Six", "Seven", "Eight",
		"Nine", "Ten", "Jack", "Queen", "King", "Ace"}
	if r < cards.Two || r > cards.Ace {
		return "?"
	}
	return names[r-cards.Two]
}

func makeRank(cat Category, ranks ...cards.Rank) HandRank {
	h := HandRank(cat) << 20
	for i, r := range ranks {
		h |= HandRank(r) << (16 - 4*i)
	}
	return h
}

// Eval5 ranks exactly five distinct cards.
func Eval5(c [5]cards.Card) HandRank {
	var counts [15]uint8
	var mask uint16 // bit r set when rank r is present
	suits := uint8(0)
	for _, x := range c {
		r := x.Rank()
		counts[r]++
		mask |= 1 << r
		suits |= 1 << x.Suit()
	}
	flush := suits&(suits-1) == 0 // exactly one suit bit set

	if bits.OnesCount16(mask) == 5 {
		var h HandRank
		for i := range 5 { // ranks high to low become the tie-breaks
			top := bits.Len16(mask) - 1
			mask &^= 1 << top
			h |= HandRank(top) << (16 - 4*i)
		}
		hi, lo := h.TieBreak(0), h.TieBreak(4)
		straight := hi-lo == 4
		if hi == cards.Ace && h.TieBreak(1) == cards.Five {
			straight, hi = true, cards.Five // wheel: A-5-4-3-2 is five-high
		}
		switch {
		case straight && flush:
			return makeRank(StraightFlush, hi)
		case flush:
			return HandRank(Flush)<<20 | h
		case straight:
			return makeRank(Straight, hi)
		default:
			return HandRank(HighCard)<<20 | h
		}
	}

	// Paired hands: bucket the (at most four) distinct ranks by count,
	// highest rank first. Buckets in count order give the tie-break order.
	var quad, trip cards.Rank
	var pairs, singles [3]cards.Rank
	np, ns := 0, 0
	for m := mask; m != 0; {
		r := cards.Rank(bits.Len16(m) - 1)
		m &^= 1 << r
		switch counts[r] {
		case 4:
			quad = r
		case 3:
			trip = r
		case 2:
			pairs[np] = r
			np++
		default:
			singles[ns] = r
			ns++
		}
	}
	switch {
	case quad != 0:
		return makeRank(FourOfAKind, quad, singles[0])
	case trip != 0 && np == 1:
		return makeRank(FullHouse, trip, pairs[0])
	case trip != 0:
		return makeRank(ThreeOfAKind, trip, singles[0], singles[1])
	case np == 2:
		return makeRank(TwoPair, pairs[0], pairs[1], singles[0])
	default:
		return makeRank(OnePair, pairs[0], singles[0], singles[1], singles[2])
	}
}
