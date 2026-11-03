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
		check(p.Hole)
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
		if h.Picking {
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
			if want := [...]int{0, 3, 4, 5}[h.Street]; len(b) != want {
				fail("%v board has %d cards, want %d", h.Street, len(b), want)
			}
		}
	}
	return errors.Join(errs...)
}
