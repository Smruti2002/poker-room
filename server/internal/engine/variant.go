package engine

import (
	"fmt"
	"maps"

	"pokerroom/internal/cards"
	"pokerroom/internal/eval"
)

// Variant is a poker game played on the shared betting engine. Variants with
// extra phases add hooks here; multi-board allocation is still to come.
type Variant interface {
	Name() string
	// HoleCards is how many cards each player is dealt.
	HoleCards() int
	// Boards is how many 5-card boards are dealt.
	Boards() int
	// Reveal is how many hole cards each player shows, in seat order, before
	// the pre-flop betting (Show 2 Play 2). Zero for most variants.
	Reveal() int
	// Choices are the hole-card counts players pick from before the deal
	// (Dacoit), each with the chips per unit it costs. The first is the
	// default and the dealer's fixed choice. Nil for most variants.
	Choices() []Choice
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

func (holdem) Name() string      { return "holdem" }
func (holdem) HoleCards() int    { return 2 }
func (holdem) Boards() int       { return 1 }
func (holdem) Reveal() int       { return 0 }
func (holdem) Choices() []Choice { return nil }

func (holdem) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.Holdem(p.Hole, boards[0])
	return []eval.Hand{h}, err
}

// Omaha: four hole cards, exactly two of them plus three from the board
// (docs/rules.md §4.2).
var Omaha = register(omaha{})

type omaha struct{}

func (omaha) Name() string      { return "omaha" }
func (omaha) HoleCards() int    { return 4 }
func (omaha) Boards() int       { return 1 }
func (omaha) Reveal() int       { return 0 }
func (omaha) Choices() []Choice { return nil }

func (omaha) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.Omaha(p.Hole, boards[0])
	return []eval.Hand{h}, err
}

// ShowTwo is Show 2 Play 2: four hole cards, two shown to everyone before
// the betting; the final hand is three board cards plus exactly one shown
// and one hidden card (docs/rules.md §4.6).
var ShowTwo = register(showTwo{})

type showTwo struct{}

func (showTwo) Name() string      { return "show2" }
func (showTwo) HoleCards() int    { return 4 }
func (showTwo) Boards() int       { return 1 }
func (showTwo) Reveal() int       { return 2 }
func (showTwo) Choices() []Choice { return nil }

func (showTwo) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.ShowTwo(p.Shown, p.Hidden(), boards[0])
	return []eval.Hand{h}, err
}

// Choice is a hole-card count a player may pick and what it costs: Mult
// chips per unit of every bet.
type Choice struct {
	Cards int
	Mult  int64
}

// Dacoit: before the deal, each player but the dealer takes 4 or 6 cards;
// 6-card players pay double for everything. Showdown is Omaha's exactly two
// hole cards plus three from the board (docs/rules.md §4.3).
var Dacoit = register(dacoit{})

type dacoit struct{}

func (dacoit) Name() string { return "dacoit" }

// HoleCards is the minimum; each player's actual count is their choice.
func (dacoit) HoleCards() int    { return 4 }
func (dacoit) Boards() int       { return 1 }
func (dacoit) Reveal() int       { return 0 }
func (dacoit) Choices() []Choice { return []Choice{{Cards: 4, Mult: 1}, {Cards: 6, Mult: 2}} }

func (dacoit) Rank(p *Player, boards [][]cards.Card) ([]eval.Hand, error) {
	h, err := eval.Omaha(p.Hole, boards[0])
	return []eval.Hand{h}, err
}
