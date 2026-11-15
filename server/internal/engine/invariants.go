package engine

import (
	"errors"
	"fmt"

	"pokerroom/internal/cards"
)

// CheckInvariants verifies properties that must hold after every action.
// Tests and the simulator call it; a failure means an engine bug.
func (h *Hand) CheckInvariants() error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	// Chips are conserved: stacks plus what's in the pot equal the start.
	var stacks, pot, maxStreet int64
	for i, p := range h.Players {
		stacks += p.Stack
		if !h.Done {
			pot += p.Total * p.Mult
		}
		if p.Stack < 0 || p.Street < 0 || p.Total < 0 || p.Street > p.Total {
			fail("player %d has negative or inconsistent amounts: %+v", i, *p)
		}
		if p.Folded && p.Won > 0 {
			fail("folded player %d won %d", i, p.Won)
		}
		if !h.Done && p.AllIn != (p.Stack < p.Mult) {
			fail("player %d all-in flag %v with stack %d mult %d", i, p.AllIn, p.Stack, p.Mult)
		}
		maxStreet = max(maxStreet, p.Street)
	}
	if stacks+pot != h.chips {
		fail("chips not conserved: stacks %d + pot %d != %d", stacks, pot, h.chips)
	}

	// No card appears twice.
	var seen cards.Set
	check := func(cs []cards.Card) {
		for _, c := range cs {
			if !seen.Add(c) {
				fail("card %v dealt twice", c)
			}
		}
	}
	for _, p := range h.Players {
		// In BB a folded hand goes back into the deck at the next exchange,
		// so its cards may be dealt again.
		if p.Folded && draws(h.Variant) > 0 {
			continue
		}
		check(p.Hole)
		check(p.Discarded)
	}
	for _, b := range h.Boards {
		check(b)
	}

	if h.Done {
		if h.ToAct != -1 {
			fail("hand done but ToAct = %d", h.ToAct)
		}
		var won int64
		for _, p := range h.Players {
			won += p.Won
		}
		var potChips int64
		for _, pot := range h.Pots {
			potChips += pot.Chips
		}
		if won != potChips {
			fail("paid out %d but pots held %d", won, potChips)
		}
	} else {
		if h.Allocating {
			if h.ToAct != -1 || len(h.Pending()) == 0 {
				fail("allocating with ToAct %d and %d players pending", h.ToAct, len(h.Pending()))
			}
		} else if h.Drawing {
			if h.ToAct != -1 || len(h.DrawPending()) == 0 {
				fail("exchanging with ToAct %d and %d players pending", h.ToAct, len(h.DrawPending()))
			}
		} else if h.Rolling {
			if h.ToAct != -1 || len(h.RollPending()) == 0 || h.Street != Preflop {
				fail("rolling with ToAct %d, %d players pending, on %v", h.ToAct, len(h.RollPending()), h.Street)
			}
		} else if h.Discarding {
			if h.ToAct != -1 || len(h.DiscardPending()) == 0 {
				fail("discarding with ToAct %d and %d players pending", h.ToAct, len(h.DiscardPending()))
			}
		} else if h.Picking {
			if h.ToAct < 0 || h.ToAct >= len(h.Players) || h.ToAct == h.Dealer || h.Players[h.ToAct].Picked != 0 {
				fail("ToAct %d is not a player who still has to choose", h.ToAct)
			}
			if pot != 0 {
				fail("%d chips in the pot before the deal", pot)
			}
		} else if h.Showing {
			if h.ToAct < 0 || h.ToAct >= len(h.Players) || h.Players[h.ToAct].Shown != nil {
				fail("ToAct %d is not a player who still has to show", h.ToAct)
			}
		} else if h.ToAct < 0 || h.ToAct >= len(h.Players) || !h.needsToAct(h.Players[h.ToAct]) {
			fail("ToAct %d is not a player who needs to act", h.ToAct)
		}
		if maxStreet > h.CurrentBet {
			fail("a street bet %d exceeds the current bet %d", maxStreet, h.CurrentBet)
		}
		for _, b := range h.Boards {
			if want := [...]int{0, 3, 4, 5, 5}[h.Street]; len(b) != want {
				fail("%v board has %d cards, want %d", h.Street, len(b), want)
			}
		}
	}
	if draws(h.Variant) > 0 {
		for i, p := range h.Players {
			if !p.Folded && len(p.Hole) != h.Variant.HoleCards() {
				fail("player %d holds %d cards, want %d", i, len(p.Hole), h.Variant.HoleCards())
			}
		}
		if h.DrawRound > int(h.Street) || h.DrawRound > draws(h.Variant) {
			fail("%d exchanges done by %v", h.DrawRound, h.Street)
		}
	}
	if rollsDice(h.Variant) {
		for i, p := range h.Players {
			if p.Die < 0 || p.Die > 6 || (p.Die != 0 && p.Die != h.dice[i]) {
				fail("player %d rolled %d, die is %d", i, p.Die, h.dice[i])
			}
			if !h.Rolling && !h.Picking && p.Die == 0 && !p.Folded {
				fail("player %d is betting without rolling", i)
			}
			// What they peeked at is the board card, once it's dealt.
			if c, ok := h.PeekCard(i); ok && len(h.Boards[0]) >= p.Die && h.Boards[0][p.Die-1] != c {
				fail("player %d peeked at %v but board card %d is %v", i, c, p.Die, h.Boards[0][p.Die-1])
			}
		}
	}
	if n := discardsPerRound(h.Variant); n > 0 {
		// Before the flop's discard: all six; then two fewer per discard.
		if h.DiscardRound > 2 || (h.Street == Preflop && h.DiscardRound > 0) {
			fail("%d discards done by %v", h.DiscardRound, h.Street)
		}
		for i, p := range h.Players {
			if want := h.Variant.HoleCards() - n*h.DiscardRound; !p.Folded && len(p.Hole) != want {
				fail("player %d holds %d cards after %d discards, want %d", i, len(p.Hole), h.DiscardRound, want)
			}
			if len(p.Discarded) > n*h.DiscardRound {
				fail("player %d discarded %d cards in %d discards", i, len(p.Discarded), h.DiscardRound)
			}
		}
		if h.Done && h.liveCount() > 1 && h.DiscardRound != 2 {
			fail("showdown after %d discards", h.DiscardRound)
		}
	}
	return errors.Join(errs...)
}
