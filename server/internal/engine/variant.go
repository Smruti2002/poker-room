package engine

import (
	"fmt"

	"pokerroom/internal/cards"
	"pokerroom/internal/eval"
)

// Variant is a poker game played on the shared betting engine. Variants with
// extra phases (Dacoit's choice, Show 2 Play 2's reveal, multi-board
// allocation) will add hooks here as they are implemented.
type Variant interface {
	Name() string
	// HoleCards is how many cards each player is dealt.
	HoleCards() int
	// Boards is how many 5-card boards are dealt.
	Boards() int
	// Rank returns the player's best hand on each board.
	Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error)
}

// cardsNeeded is how many cards a hand of v uses with n players.
func cardsNeeded(v Variant, n int) int {
	return n*v.HoleCards() + 5*v.Boards()
}

var variants = map[string]Variant{}

func register(v Variant) Variant {
	variants[v.Name()] = v
	return v
}

// VariantByName looks up a registered variant.
func VariantByName(name string) (Variant, error) {
	v, ok := variants[name]
	if !ok {
		return nil, fmt.Errorf("unknown variant %q", name)
	}
	return v, nil
}

// Holdem is Texas Hold'em (docs/rules.md §4.1).
var Holdem = register(holdem{})

type holdem struct{}

func (holdem) Name() string   { return "holdem" }
func (holdem) HoleCards() int { return 2 }
func (holdem) Boards() int    { return 1 }

func (holdem) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.Holdem(p.Hole, boards[0])
	return []eval.Hand{h}, err
}
