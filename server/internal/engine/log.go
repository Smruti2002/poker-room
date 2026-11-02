package engine

import (
	"fmt"

	"pokerroom/internal/cards"
)

// Log is everything needed to replay a hand exactly: the setup, the shuffle
// seed (or a fixed deck) and every action in order.
type Log struct {
	Variant string         `json:"variant"`
	Config  Config         `json:"config"`
	Seats   []SeatSpec     `json:"seats"`
	Dealer  int            `json:"dealer"`
	Seed    cards.Seed     `json:"seed"`
	Deck    []cards.Card   `json:"deck,omitempty"`
	Actions []LoggedAction `json:"actions"`
}

// LoggedAction is one action in a Log.
type LoggedAction struct {
	Player int    `json:"player"`
	Action Action `json:"action"`
}

// Replay rebuilds a hand from its log.
func Replay(l Log) (*Hand, error) {
	v, err := VariantByName(l.Variant)
	if err != nil {
		return nil, err
	}
	h, err := NewHand(l.Config, v, l.Seats, l.Dealer, Options{Seed: l.Seed, Deck: l.Deck})
	if err != nil {
		return nil, err
	}
	for n, a := range l.Actions {
		if err := h.Apply(a.Player, a.Action); err != nil {
			return h, fmt.Errorf("action %d (player %d %v): %w", n, a.Player, a.Action, err)
		}
	}
	return h, nil
}
