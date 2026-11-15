package engine

import (
	"fmt"
	"slices"

	"pokerroom/internal/cards"
)

// Peekaboo's roll (docs/rules.md §4.10). After the deal and before the
// pre-flop betting, everyone rolls a die, all at once and in any order. A
// 1 to 5 lets the player peek at that board card (1-3 the flop, 4 the turn,
// 5 the river) before it's dealt; a 6 means no peek. Everyone sees each
// roll, so they know which card a player saw, but not the card itself.
//
// The dice are fixed by the hand's seed when the hand starts, so a hand
// replays exactly; rolling only reveals them. The board cards are the next
// five in the deck, since nothing else is dealt before the flop.

// NoPeek is the die roll that shows nothing.
const NoPeek = 6

// roller is a variant where everyone rolls a die before betting.
type roller interface{ RollsDice() bool }

func rollsDice(v Variant) bool {
	r, ok := v.(roller)
	return ok && r.RollsDice()
}

// setDice fixes each player's die for the hand.
func (h *Hand) setDice(opts Options) error {
	n := len(h.Players)
	if opts.Dice != nil {
		if len(opts.Dice) != n {
			return fmt.Errorf("%d dice for %d players", len(opts.Dice), n)
		}
		for _, d := range opts.Dice {
			if d < 1 || d > 6 {
				return fmt.Errorf("invalid die %d", d)
			}
		}
		h.dice = slices.Clone(opts.Dice)
		return nil
	}
	// A stream of its own, so the shuffle is the same as without dice.
	seed := opts.Seed
	seed[0] ^= 0x9d
	r := seed.Rand()
	h.dice = make([]int, n)
	for i := range h.dice {
		h.dice[i] = 1 + r.IntN(6)
	}
	return nil
}

// RollPending lists the players who still have to roll.
func (h *Hand) RollPending() []int {
	if !h.Rolling {
		return nil
	}
	var out []int
	for i, p := range h.Players {
		if !p.Folded && p.Die == 0 {
			out = append(out, i)
		}
	}
	return out
}

// roll handles a Roll: player i rolls their die and peeks at that card.
func (h *Hand) roll(i int, a Action) error {
	if i < 0 || i >= len(h.Players) || !slices.Contains(h.RollPending(), i) {
		return ErrNotYourTurn
	}
	if a.Type != Roll {
		return fmt.Errorf("%w: roll your die first", ErrIllegalAction)
	}
	p := h.Players[i]
	p.Die = h.dice[i]
	h.log.Actions = append(h.log.Actions, LoggedAction{Player: i, Action: a})
	h.emit(Event{Kind: EvRoll, Player: i, Count: p.Die})
	if p.Die < NoPeek {
		board, _ := h.deck.Peek(5) // NewHand checked there are enough cards
		p.Peek = board[p.Die-1]
		h.emit(Event{Kind: EvPeek, Player: i, Cards: []cards.Card{p.Peek}, Board: p.Die})
	}
	if len(h.RollPending()) > 0 {
		return nil
	}
	h.Rolling = false
	return h.advance(h.BigBlind)
}

// DefaultRoll is a skipped player's move: their die is rolled for them.
func (h *Hand) DefaultRoll() Action { return Action{Type: Roll} }

// PeekCard is the board card player i saw, if any.
func (h *Hand) PeekCard(i int) (cards.Card, bool) {
	p := h.Players[i]
	return p.Peek, p.Die > 0 && p.Die < NoPeek
}

// PeekPosition names board position n (1-5).
func PeekPosition(n int) string {
	switch n {
	case 1, 2, 3:
		return fmt.Sprintf("flop card %d", n)
	case 4:
		return "the turn"
	case 5:
		return "the river"
	}
	return "no card"
}
