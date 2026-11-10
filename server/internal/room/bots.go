package room

import (
	"errors"
	"fmt"
	"slices"

	"pokerroom/internal/bot"
	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// The host can seat computer players ("Bot 1", "Bot 2", ...). They play
// every variant on their own (package bot), pick the game when they deal,
// and borrow like everyone else when they run out of chips. They act the
// moment the game waits on them: there are no timers, so they don't pause
// to "think". They never vote, and never become host.

func (r *Room) addBot(p *Player) error {
	switch {
	case p.ID != r.hostID:
		return ErrNotHost
	case r.phase == protocol.PhaseEnded:
		return ErrGameOver
	}
	name := ""
	for n := 1; name == ""; n++ {
		candidate := fmt.Sprintf("Bot %d", n)
		if !slices.ContainsFunc(r.players, func(q *Player) bool { return q.Name == candidate }) {
			name = candidate
		}
	}
	b, err := r.Join(name)
	if err != nil {
		return err
	}
	b.Bot = true
	return nil
}

func (r *Room) removeBot(p *Player, target string) error {
	b := r.player(target)
	switch {
	case p.ID != r.hostID:
		return ErrNotHost
	case b == nil || !b.Bot || b.Kicked:
		return errors.New("no such bot")
	}
	if r.phase == protocol.PhaseLobby {
		r.players = slices.DeleteFunc(r.players, func(q *Player) bool { return q == b })
		return nil
	}
	// Mid-game it leaves like a kicked player: on the leaderboard, its
	// chips in the pot staying there.
	r.kick(b)
	return nil
}

// runBots makes every move the game is waiting on a bot for.
func (r *Room) runBots() {
	for range 10_000 { // a bound, in case of a bug; a hand needs far fewer
		if !r.botStep() {
			return
		}
	}
}

// botStep makes one bot move, if one is due.
func (r *Room) botStep() bool {
	if r.phase != protocol.PhasePlaying {
		return false
	}
	if r.stage != protocol.StageHand {
		for _, b := range r.players {
			if b.Bot && !b.Kicked && !b.SittingOut && b.Stack == 0 && r.takeLoan(b) == nil {
				return true
			}
		}
	}
	switch r.stage {
	case protocol.StageChoosing:
		if d := r.player(r.dealerID); d != nil && d.Bot && len(r.options) > 0 {
			return r.chooseVariant(d, r.options[r.opts.Rand.IntN(len(r.options))]) == nil
		}
	case protocol.StageHand:
		h := r.hand
		if h.Done {
			return false
		}
		waiting := []int{h.ToAct}
		if h.Allocating || h.Drawing {
			waiting = append(h.Pending(), h.DrawPending()...)
		}
		for _, i := range waiting {
			if i < 0 {
				continue
			}
			b := r.player(r.handIDs[i])
			if b == nil || !b.Bot || b.Kicked {
				continue
			}
			a := bot.Decide(r.botView(i), r.opts.Rand)
			if err := h.Apply(i, a); err != nil {
				// Shouldn't happen; fall back to the default move rather
				// than leave the table waiting on a bot.
				if err := h.Apply(i, r.defaultMove(i)); err != nil {
					return false
				}
			}
			r.afterHandChange()
			return true
		}
	}
	return false
}

// botView is what bot i may see: its own cards and what's public.
func (r *Room) botView(i int) bot.View {
	h := r.hand
	p := h.Players[i]
	live := 0
	for _, q := range h.Players {
		if !q.Folded {
			live++
		}
	}
	return bot.View{
		Variant: r.variant, Legal: h.Legal(i), Hole: p.Hole, Shown: p.Shown, Boards: h.Boards,
		Pot: h.PotChips(), Stack: p.Stack, Mult: p.Mult, Live: live, Reveal: h.Variant.Reveal(),
		Split: func() engine.Action { return h.DefaultSplit(i) },
	}
}
