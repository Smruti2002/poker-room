package engine

import (
	"fmt"
	"maps"

	"pokerroom/internal/cards"
	"pokerroom/internal/eval"
)

// Variant is a poker game played on the shared betting engine. Variants with
// extra phases (Dacoit's choice, multi-board allocation) will add hooks here
// as they are implemented.
type Variant interface {
	Name() string
	// HoleCards is how many cards each player is dealt.
	HoleCards() int
	// Boards is how many 5-card boards are dealt.
	Boards() int
	// Reveal is how many hole cards each player shows, in seat order, before
	// the pre-flop betting (Show 2 Play 2). Zero for most variants.
	Reveal() int
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

// AllVariants returns every registered variant.
func AllVariants() map[string]Variant { return maps.Clone(variants) }

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
func (holdem) Reveal() int    { return 0 }

func (holdem) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.Holdem(p.Hole, boards[0])
	return []eval.Hand{h}, err
}

// Omaha: four hole cards, exactly two of them plus three from the board
// (docs/rules.md §4.2).
var Omaha = register(omaha{})

type omaha struct{}

func (omaha) Name() string   { return "omaha" }
func (omaha) HoleCards() int { return 4 }
func (omaha) Boards() int    { return 1 }
func (omaha) Reveal() int    { return 0 }

func (omaha) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.Omaha(p.Hole, boards[0])
	return []eval.Hand{h}, err
}

// ShowTwo is Show 2 Play 2: four hole cards, two shown to everyone before
// the betting; the final hand is three board cards plus exactly one shown
// and one hidden card (docs/rules.md §4.6).
var ShowTwo = register(showTwo{})

type showTwo struct{}

func (showTwo) Name() string   { return "show2" }
func (showTwo) HoleCards() int { return 4 }
func (showTwo) Boards() int    { return 1 }
func (showTwo) Reveal() int    { return 2 }

func (showTwo) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.ShowTwo(p.Shown, p.Hidden(), boards[0])
	return []eval.Hand{h}, err
}
