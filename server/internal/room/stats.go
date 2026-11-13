package room

import (
	"cmp"
	"maps"
	"slices"

	"pokerroom/internal/protocol"
)

// Game stats: running totals for the whole game, shown on the final
// standings (docs/rules.md §1). They're built from what everyone saw at the
// table (who won, who folded, how stacks moved), never from hidden cards,
// and kept per room rather than read back from the hand archive, so they
// work without one and survive restarts through the snapshot.

// playerStats are one player's totals.
type playerStats struct {
	Hands        int              `json:"hands,omitempty"`        // dealt in
	Won          int              `json:"won,omitempty"`          // won chips from a pot
	Folded       int              `json:"folded,omitempty"`       // folded before the end
	Showdowns    int              `json:"showdowns,omitempty"`    // still in at a showdown
	ShowdownsWon int              `json:"showdownsWon,omitempty"` // ...and won chips there
	BiggestWin   int64            `json:"biggestWin,omitempty"`   // most chips won in one hand
	Net          map[string]int64 `json:"net,omitempty"`          // chips won or lost, per variant
}

// gameStats are the table's totals.
type gameStats struct {
	Hands    int            `json:"hands,omitempty"`
	Variants map[string]int `json:"variants,omitempty"` // hands per variant
	// The biggest pot: the most chips paid out in one hand.
	BiggestPot        int64    `json:"biggestPot,omitempty"`
	BiggestPotHand    int      `json:"biggestPotHand,omitempty"`
	BiggestPotVariant string   `json:"biggestPotVariant,omitempty"`
	BiggestPotWinners []string `json:"biggestPotWinners,omitempty"`
}

// recordStats adds the finished hand to the totals. It runs before the
// room's stacks are updated, so a player's stack is still their stack at the
// start of the hand. (Nobody can borrow while dealt in.)
func (r *Room) recordStats() {
	h := r.hand
	live := 0
	for _, hp := range h.Players {
		if !hp.Folded {
			live++
		}
	}
	var paid int64
	var winners []string
	for i, id := range r.handIDs {
		p, hp := r.player(id), h.Players[i]
		s := &p.Stats
		s.Hands++
		if hp.Folded {
			s.Folded++
		}
		if live > 1 && !hp.Folded {
			s.Showdowns++
			if hp.Won > 0 {
				s.ShowdownsWon++
			}
		}
		if hp.Won > 0 {
			s.Won++
			s.BiggestWin = max(s.BiggestWin, hp.Won)
			winners = append(winners, id)
		}
		if s.Net == nil {
			s.Net = map[string]int64{}
		}
		s.Net[r.variant] += hp.Stack - p.Stack
		paid += hp.Won
	}

	g := &r.stats
	g.Hands++
	if g.Variants == nil {
		g.Variants = map[string]int{}
	}
	g.Variants[r.variant]++
	if paid > g.BiggestPot {
		g.BiggestPot, g.BiggestPotHand, g.BiggestPotVariant, g.BiggestPotWinners = paid, r.handNo, r.variant, winners
	}
}

// statsView is the game's stats as sent to players once the game is over.
func (r *Room) statsView() *protocol.GameStats {
	if r.stats.Hands == 0 {
		return nil
	}
	g := &protocol.GameStats{Hands: r.stats.Hands}
	for _, v := range slices.Sorted(maps.Keys(r.stats.Variants)) {
		g.Variants = append(g.Variants, protocol.VariantCount{Variant: v, Hands: r.stats.Variants[v]})
	}
	slices.SortStableFunc(g.Variants, func(a, b protocol.VariantCount) int { return cmp.Compare(b.Hands, a.Hands) })
	if r.stats.BiggestPot > 0 {
		g.BiggestPot = &protocol.PotStat{
			Chips:   r.stats.BiggestPot,
			Hand:    r.stats.BiggestPotHand,
			Variant: r.stats.BiggestPotVariant,
			Winners: slices.Clone(r.stats.BiggestPotWinners),
		}
	}
	for _, p := range r.players {
		s := p.Stats
		if s.Hands == 0 {
			continue
		}
		ps := protocol.PlayerStats{
			ID: p.ID, Hands: s.Hands, Won: s.Won, Folded: s.Folded,
			Showdowns: s.Showdowns, ShowdownsWon: s.ShowdownsWon, BiggestWin: s.BiggestWin,
		}
		for _, v := range slices.Sorted(maps.Keys(s.Net)) {
			ps.Net = append(ps.Net, protocol.VariantNet{Variant: v, Chips: s.Net[v]})
		}
		g.Players = append(g.Players, ps)
	}
	return g
}

// clone copies the totals, so a snapshot shares nothing with the live room.
func (s playerStats) clone() playerStats {
	s.Net = maps.Clone(s.Net)
	return s
}

func (g gameStats) clone() gameStats {
	g.Variants = maps.Clone(g.Variants)
	g.BiggestPotWinners = slices.Clone(g.BiggestPotWinners)
	return g
}
