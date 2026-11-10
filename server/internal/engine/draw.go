package engine

import (
	"fmt"
	"slices"

	"pokerroom/internal/cards"
)

// BB's exchanges (docs/rules.md §4.8). After each of the first three
// betting rounds, everyone still in says which of their cards (0-5) they
// exchange, all at once and in any order. Then the discards, the cards of
// players who have folded and the undealt cards are shuffled together, and
// replacements are dealt in seat order starting left of the dealer. How
// many cards each player exchanged is public; which cards is not.
//
// A player who is all-in gets one more exchange, then keeps their hand.

// drawsLeft is how many exchanges are still to come.
func (h *Hand) drawsLeft() int { return draws(h.Variant) - h.DrawRound }

// mayDraw reports whether player p takes part in this exchange.
func mayDraw(p *Player) bool { return !p.Folded && (!p.AllIn || p.allInDraws == 0) }

// DrawPending lists the players who still have to choose their exchange.
func (h *Hand) DrawPending() []int {
	if !h.Drawing {
		return nil
	}
	var out []int
	for i, p := range h.Players {
		if mayDraw(p) && !p.drew {
			out = append(out, i)
		}
	}
	return out
}

// startDraw opens an exchange; the next betting round starts with lead.
func (h *Hand) startDraw(lead int) error {
	h.Drawing = true
	h.ToAct = -1
	h.nextLead = lead
	for _, p := range h.Players {
		p.drew, p.discards = false, nil
	}
	if len(h.DrawPending()) == 0 {
		return h.finishDraw()
	}
	return nil
}

// draw handles a Draw: Cards are the hole cards player i gives back.
func (h *Hand) draw(i int, a Action) error {
	if i < 0 || i >= len(h.Players) || !slices.Contains(h.DrawPending(), i) {
		return ErrNotYourTurn
	}
	p := h.Players[i]
	if a.Type != Draw {
		return fmt.Errorf("%w: choose which cards to exchange first", ErrIllegalAction)
	}
	if len(a.Cards) > len(p.Hole) {
		return fmt.Errorf("%w: you have only %d cards", ErrIllegalAction, len(p.Hole))
	}
	for j, c := range a.Cards {
		if !slices.Contains(p.Hole, c) || slices.Contains(a.Cards[:j], c) {
			return fmt.Errorf("%w: %v is not one of your cards, or is listed twice", ErrIllegalAction, c)
		}
	}
	p.drew = true
	p.discards = slices.Clone(a.Cards)
	h.log.Actions = append(h.log.Actions, LoggedAction{Player: i, Action: a})
	if len(h.DrawPending()) > 0 {
		return nil
	}
	return h.finishDraw()
}

// finishDraw reshuffles the discards and folded cards into the deck, deals
// the replacements, and starts the next betting round.
func (h *Hand) finishDraw() error {
	n := len(h.Players)
	for i := range n {
		h.muck = append(h.muck, h.Players[i].discards...)
	}
	rest, _ := h.deck.Deal(h.deck.Remaining())
	h.deck = cards.NewDeckOf(append(rest, h.muck...))
	h.muck = nil
	if h.log.Deck == nil {
		// A fresh shuffle for each exchange, still fixed by the hand's seed
		// so the hand replays exactly.
		seed := h.log.Seed
		seed[len(seed)-1] ^= byte(h.DrawRound + 1)
		h.deck.Shuffle(seed.Rand())
	}

	for k := 1; k <= n; k++ {
		i := (h.Dealer + k) % n
		p := h.Players[i]
		if !p.drew {
			continue
		}
		got := h.deal(len(p.discards))
		p.Hole = slices.DeleteFunc(p.Hole, func(c cards.Card) bool { return slices.Contains(p.discards, c) })
		p.Hole = append(p.Hole, got...)
		p.Draws = append(p.Draws, len(got))
		if p.AllIn {
			p.allInDraws++
		}
		h.emit(Event{Kind: EvDraw, Player: i, Count: len(got)})
		if len(got) > 0 {
			h.emit(Event{Kind: EvDrawn, Player: i, Cards: got, Discards: p.discards})
		}
		p.drew, p.discards = false, nil
	}
	h.Drawing = false
	h.DrawRound++
	return h.nextStreet(h.nextLead)
}

// DefaultDraw is a skipped player's exchange: keep every card.
func (h *Hand) DefaultDraw() Action { return Action{Type: Draw} }
