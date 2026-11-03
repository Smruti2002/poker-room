// Package sim plays many hands between bots to shake out engine bugs.
// Every action is followed by an invariant check, and a failing hand's log
// is returned so it can be replayed with cmd/replay.
package sim

import (
	"fmt"
	"math/rand/v2"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
)

// Config describes a simulated game.
type Config struct {
	// Variant is the game to play; empty means Texas Hold'em.
	Variant    string
	Players    int
	Hands      int
	StartStack int64
	SmallBlind int64
	BigBlind   int64
	Structure  engine.Structure
	// DoubleChance is the probability that a player pays 2 chips per unit
	// in a hand, exercising the Dacoit pot math.
	DoubleChance float64
	Seed         uint64
	// OnHand, if set, is called after every finished hand.
	OnHand func(*engine.Hand)
}

// Stats summarises a run.
type Stats struct {
	Hands     int
	Actions   int
	Showdowns int
	FoldWins  int
	SidePots  int // hands with more than one pot
	SplitPots int // pots shared by more than one winner
	Rebuys    int
	Stacks    []int64
}

// Failure is a hand that broke an invariant or rejected a legal action.
type Failure struct {
	Hand int
	Log  engine.Log
	Err  error
}

func (f *Failure) Error() string { return fmt.Sprintf("hand %d: %v", f.Hand, f.Err) }
func (f *Failure) Unwrap() error { return f.Err }

// Run plays cfg.Hands hands. Busted players rebuy for StartStack, standing
// in for the room's loan flow.
func Run(cfg Config) (Stats, error) {
	r := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0x9e3779b97f4a7c15))
	stacks := make([]int64, cfg.Players)
	for i := range stacks {
		stacks[i] = cfg.StartStack
	}
	var st Stats
	dealer := r.IntN(cfg.Players)
	variant := engine.Holdem
	if cfg.Variant != "" {
		v, err := engine.VariantByName(cfg.Variant)
		if err != nil {
			return st, err
		}
		variant = v
	}
	bot := RandomBot{}

	for n := range cfg.Hands {
		seats := make([]engine.SeatSpec, cfg.Players)
		for i := range seats {
			if stacks[i] < 2 { // can't cover even one unit as a double-pay player
				stacks[i] += cfg.StartStack
				st.Rebuys++
			}
			mult := int64(1)
			if r.Float64() < cfg.DoubleChance {
				mult = 2
			}
			seats[i] = engine.SeatSpec{ID: fmt.Sprintf("bot%d", i), Stack: stacks[i], Mult: mult}
		}
		var seed cards.Seed
		for i := range seed {
			seed[i] = byte(r.Uint32())
		}
		h, err := engine.NewHand(engine.Config{
			SmallBlind: cfg.SmallBlind, BigBlind: cfg.BigBlind, Structure: cfg.Structure,
		}, variant, seats, dealer, engine.Options{Seed: seed})
		if err != nil {
			return st, fmt.Errorf("hand %d setup: %w", n, err)
		}
		fail := func(err error) error { return &Failure{Hand: n, Log: h.Log(), Err: err} }
		if err := h.CheckInvariants(); err != nil {
			return st, fail(err)
		}

		for !h.Done {
			a := bot.Act(h, h.ToAct, r)
			if err := h.Apply(h.ToAct, a); err != nil {
				return st, fail(fmt.Errorf("bot action %v rejected: %w", a, err))
			}
			if err := h.CheckInvariants(); err != nil {
				return st, fail(err)
			}
			st.Actions++
		}

		st.Hands++
		live := 0
		for i, p := range h.Players {
			stacks[i] = p.Stack
			if !p.Folded {
				live++
			}
		}
		if live > 1 {
			st.Showdowns++
		} else {
			st.FoldWins++
		}
		if len(h.Pots) > 1 {
			st.SidePots++
		}
		st.SplitPots += countSplits(h)
		if cfg.OnHand != nil {
			cfg.OnHand(h)
		}
		dealer = (dealer + 1) % cfg.Players
	}
	st.Stacks = stacks
	return st, nil
}

func countSplits(h *engine.Hand) int {
	winners := map[int]int{}
	for _, e := range h.Events {
		if e.Kind == engine.EvWin {
			winners[e.Pot]++
		}
	}
	n := 0
	for _, w := range winners {
		if w > 1 {
			n++
		}
	}
	return n
}

// RandomBot picks loosely realistic random actions: mostly checks and
// calls, some folds, and raises sized from minimum to all-in.
type RandomBot struct{}

// Act chooses an action for player i, which must be the player to act.
func (RandomBot) Act(h *engine.Hand, i int, r *rand.Rand) engine.Action {
	l := h.Legal(i)
	if l.Show {
		// Reveal a random pair, or the default when time runs out.
		if r.IntN(4) == 0 {
			return h.DefaultShow(i)
		}
		hole := h.Players[i].Hole
		perm := r.Perm(len(hole))
		return engine.Action{Type: engine.Show, Cards: []cards.Card{hole[perm[0]], hole[perm[1]]}}
	}
	roll := r.Float64()
	switch {
	case l.Fold && roll < 0.2:
		return engine.Action{Type: engine.Fold}
	case (l.Bet || l.Raise) && roll > 0.75:
		typ := engine.Raise
		if l.Bet {
			typ = engine.Bet
		}
		to := l.MinTo
		switch size := r.Float64(); {
		case size > 0.85:
			to = l.MaxTo
		case size > 0.5:
			to = l.MinTo + r.Int64N(l.MaxTo-l.MinTo+1)
		}
		return engine.Action{Type: typ, To: to}
	case l.Check:
		return engine.Action{Type: engine.Check}
	default:
		return engine.Action{Type: engine.Call}
	}
}
