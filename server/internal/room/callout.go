package room

import (
	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// betCallouts returns each player's bet or raise on the current street, by
// engine index: what the table shows as "Raised 500 → 1000". A callout lasts
// until that player acts again or the street ends, and none are shown once
// the hand is over.
func betCallouts(h *engine.Hand) map[int]*protocol.BetCallout {
	out := map[int]*protocol.BetCallout{}
	if h == nil || h.Done {
		return out
	}
	var high int64 // the street's highest total, in units
	for _, e := range h.Events {
		switch e.Kind {
		case engine.EvBoard:
			clear(out) // a new street
			high = 0
		case engine.EvBlind:
			high = max(high, e.Units)
		case engine.EvAction:
			delete(out, e.Player)
			if t := e.Action.Type; t == engine.Bet || t == engine.Raise {
				mult := h.Players[e.Player].Mult
				out[e.Player] = &protocol.BetCallout{
					Type: t.String(),
					By:   (e.Units - high) * mult,
					To:   e.Chips,
				}
			}
			high = max(high, e.Units)
		}
	}
	for i, c := range out {
		c.AllIn = h.Players[i].AllIn // nothing they've done since: still true of that bet
	}
	return out
}
