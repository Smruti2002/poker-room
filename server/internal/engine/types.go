package engine

import (
	"errors"
	"fmt"

	"pokerroom/internal/cards"
)

// Errors returned by Apply. Illegal actions wrap ErrIllegalAction with detail.
var (
	ErrHandOver      = errors.New("hand is over")
	ErrNotYourTurn   = errors.New("not your turn")
	ErrIllegalAction = errors.New("illegal action")
)

// ActionType is what a player does on their turn.
type ActionType uint8

const (
	Fold ActionType = iota + 1
	Check
	Call
	Bet    // open the betting on a street; To is the street total
	Raise  // raise an existing bet; To is the street total
	Show   // Show 2 Play 2: reveal Cards before betting
	Choose // Dacoit: take Count hole cards before the deal
)

var actionNames = map[ActionType]string{Fold: "fold", Check: "check", Call: "call", Bet: "bet", Raise: "raise", Show: "show", Choose: "choose"}

func (a ActionType) String() string {
	if s, ok := actionNames[a]; ok {
		return s
	}
	return fmt.Sprintf("ActionType(%d)", uint8(a))
}

// MarshalText encodes the action as its lowercase name.
func (a ActionType) MarshalText() ([]byte, error) {
	if _, ok := actionNames[a]; !ok {
		return nil, fmt.Errorf("invalid action type %d", uint8(a))
	}
	return []byte(a.String()), nil
}

// UnmarshalText parses a lowercase action name.
func (a *ActionType) UnmarshalText(b []byte) error {
	for t, s := range actionNames {
		if s == string(b) {
			*a = t
			return nil
		}
	}
	return fmt.Errorf("unknown action %q", b)
}

// Action is a player's move. For Bet and Raise, To is the player's total
// units on this street after the action ("raise to", not "raise by"). For
// Show, Cards are the hole cards revealed.
type Action struct {
	Type  ActionType   `json:"type"`
	To    int64        `json:"to,omitempty"`
	Cards []cards.Card `json:"cards,omitempty"`
	// Count is how many hole cards to take, for Choose.
	Count int `json:"count,omitempty"`
}

func (a Action) String() string {
	if a.Type == Bet || a.Type == Raise {
		return fmt.Sprintf("%v %d", a.Type, a.To)
	}
	if a.Type == Show {
		return fmt.Sprintf("show %v", a.Cards)
	}
	if a.Type == Choose {
		return fmt.Sprintf("choose %d", a.Count)
	}
	return a.Type.String()
}

// Structure is the betting structure.
type Structure uint8

const (
	NoLimit Structure = iota
	PotLimit
)

// Config holds the per-hand table settings, in units.
type Config struct {
	SmallBlind int64     `json:"smallBlind"`
	BigBlind   int64     `json:"bigBlind"`
	Structure  Structure `json:"structure"`
}

// SeatSpec describes a player entering a hand. Players are listed in
// clockwise seat order.
type SeatSpec struct {
	ID    string `json:"id"`
	Stack int64  `json:"stack"`
	// Mult is the chips this player pays per unit: 1 normally, 2 for a
	// Dacoit 6-card player. Zero means 1.
	Mult int64 `json:"mult,omitempty"`
}

// Street is the stage of a hand.
type Street uint8

const (
	Preflop Street = iota
	Flop
	Turn
	River
	Showdown
)

var streetNames = [...]string{"Pre-flop", "Flop", "Turn", "River", "Showdown"}

func (s Street) String() string {
	if int(s) < len(streetNames) {
		return streetNames[s]
	}
	return fmt.Sprintf("Street(%d)", uint8(s))
}

// boardCards is how many cards each board receives when the street starts.
func (s Street) boardCards() int {
	switch s {
	case Flop:
		return 3
	case Turn, River:
		return 1
	}
	return 0
}

// Legal describes what the player to act may do.
type Legal struct {
	// Show is set during Show 2 Play 2's reveal phase, when it is the only
	// legal action.
	Show bool `json:"show,omitempty"`
	// Choose is set during Dacoit's choice phase, when it is the only legal
	// action; ChooseMax is the most cards this player may take (4 when the
	// deck or their stack can't support 6).
	Choose    bool `json:"choose,omitempty"`
	ChooseMax int  `json:"chooseMax,omitempty"`
	Fold      bool `json:"fold"`
	Check     bool `json:"check"`
	Call      bool `json:"call"`
	// CallUnits is what calling adds; less than the amount owed when it puts
	// the player all-in.
	CallUnits int64 `json:"callUnits,omitempty"`
	Bet       bool  `json:"bet"`
	Raise     bool  `json:"raise"`
	// MinTo and MaxTo bound the street total for a Bet or Raise. They are
	// equal when the only possible raise is all-in.
	MinTo int64 `json:"minTo,omitempty"`
	MaxTo int64 `json:"maxTo,omitempty"`
}
