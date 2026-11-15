package engine

import (
	"fmt"
	"slices"

	"pokerroom/internal/cards"
	"pokerroom/internal/eval"
)

// Discard Hold'em's discards (docs/rules.md §4.9). Right after the flop is
// dealt, and again after the turn, everyone still in throws away a fixed
// number of cards, all at once and in any order, before that street's
// betting. All-in players discard too. That a player discarded is public;
// which cards is not. Discarded cards are out of play for the hand.

// discarder is a variant with discards on the flop and turn.
type discarder interface{ DiscardsPerRound() int }

// discardsPerRound is how many cards each player throws away per discard:
// 0 for variants without discards.
func discardsPerRound(v Variant) int {
	if d, ok := v.(discarder); ok {
		return d.DiscardsPerRound()
	}
	return 0
}

// discardsDue reports whether the street just dealt opens with a discard:
// the flop and the turn, while more than one player is still in.
func (h *Hand) discardsDue() bool {
	return discardsPerRound(h.Variant) > 0 && (h.Street == Flop || h.Street == Turn) && h.liveCount() > 1
}

// DiscardPending lists the players who still have to choose their discard.
func (h *Hand) DiscardPending() []int {
	if !h.Discarding {
		return nil
	}
	var out []int
	for i, p := range h.Players {
		if !p.Folded && p.discard == nil {
			out = append(out, i)
		}
	}
	return out
}

// startDiscard opens a discard; the street's betting starts with lead.
func (h *Hand) startDiscard(lead int) error {
	h.Discarding = true
	h.ToAct = -1
	h.nextLead = lead
	for _, p := range h.Players {
		p.discard = nil
	}
	return nil
}

// discardCards handles a Discard: Cards are the hole cards player i throws away.
func (h *Hand) discardCards(i int, a Action) error {
	if i < 0 || i >= len(h.Players) || !slices.Contains(h.DiscardPending(), i) {
		return ErrNotYourTurn
	}
	p := h.Players[i]
	n := discardsPerRound(h.Variant)
	if a.Type != Discard {
		return fmt.Errorf("%w: choose %d cards to discard first", ErrIllegalAction, n)
	}
	if len(a.Cards) != n {
		return fmt.Errorf("%w: discard exactly %d cards", ErrIllegalAction, n)
	}
	for j, c := range a.Cards {
		if !slices.Contains(p.Hole, c) || slices.Contains(a.Cards[:j], c) {
			return fmt.Errorf("%w: %v is not one of your cards, or is listed twice", ErrIllegalAction, c)
		}
	}
	p.discard = slices.Clone(a.Cards)
	h.log.Actions = append(h.log.Actions, LoggedAction{Player: i, Action: a})
	if len(h.DiscardPending()) > 0 {
		return nil
	}
	return h.finishDiscard()
}

// finishDiscard takes everyone's discards out of their hands, in seat order
// from left of the dealer, and starts the street's betting.
func (h *Hand) finishDiscard() error {
	n := len(h.Players)
	for k := 1; k <= n; k++ {
		i := (h.Dealer + k) % n
		p := h.Players[i]
		if p.discard == nil {
			continue
		}
		p.Hole = slices.DeleteFunc(p.Hole, func(c cards.Card) bool { return slices.Contains(p.discard, c) })
		p.Discarded = append(p.Discarded, p.discard...)
		h.emit(Event{Kind: EvDiscard, Player: i, Count: len(p.discard)})
		h.emit(Event{Kind: EvDiscarded, Player: i, Cards: p.discard})
		p.discard = nil
	}
	h.Discarding = false
	h.DiscardRound++
	return h.startBetting(h.nextLead)
}

// DefaultDiscard is a skipped player's discard: keep the cards that make
// the best hand with the board so far, throwing away the rest.
func (h *Hand) DefaultDiscard(i int) Action {
	p := h.Players[i]
	keep := len(p.Hole) - discardsPerRound(h.Variant)
	var best eval.Hand
	var bestKeep []cards.Card
	for _, k := range combinations(p.Hole, keep) {
		hand, err := eval.Holdem(k, h.Boards[0])
		if err != nil {
			continue
		}
		if bestKeep == nil || hand.Rank > best.Rank {
			best, bestKeep = hand, k
		}
	}
	var out []cards.Card
	for _, c := range p.Hole {
		if !slices.Contains(bestKeep, c) {
			out = append(out, c)
		}
	}
	return Action{Type: Discard, Cards: out}
}

// combinations lists every way to pick k of cs, in a fixed order.
func combinations(cs []cards.Card, k int) [][]cards.Card {
	var out [][]cards.Card
	var pick func(from int, chosen []cards.Card)
	pick = func(from int, chosen []cards.Card) {
		if len(chosen) == k {
			out = append(out, slices.Clone(chosen))
			return
		}
		for j := from; j < len(cs); j++ {
			pick(j+1, append(chosen, cs[j]))
		}
	}
	pick(0, nil)
	return out
}
