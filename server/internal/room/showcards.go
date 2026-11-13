package room

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// Showing your cards after a hand (docs/rules.md §1): once a hand is over,
// a player who was dealt in may show everyone the cards nobody saw, such as
// the winner when everyone else folded, or a player who folded. It's
// optional, and only possible until the next hand starts.

// lastHand is the finished hand on the table, if any.
func (r *Room) lastHand() *pastHand {
	if len(r.past) == 0 || r.hand == nil || !r.hand.Done || r.past[len(r.past)-1].hand != r.hand {
		return nil
	}
	return &r.past[len(r.past)-1]
}

// canShowCards reports whether p may show their cards now.
func (r *Room) canShowCards(p *Player) bool {
	ph := r.lastHand()
	if ph == nil || r.phase != protocol.PhasePlaying || r.stage != protocol.StageResult || p.Kicked || p.Bot {
		return false
	}
	i := slices.Index(ph.ids, p.ID)
	return i >= 0 && len(ph.hand.Players[i].Hole) > 0 && !shownAtShowdown(ph.hand, i) && !slices.Contains(ph.showed, p.ID)
}

func (r *Room) showCards(p *Player) error {
	if !r.canShowCards(p) {
		return errors.New("you can show your cards once a hand you played is over, before the next one")
	}
	ph := r.lastHand()
	ph.showed = append(ph.showed, p.ID)
	return nil
}

func shownAtShowdown(h *engine.Hand, i int) bool {
	return slices.ContainsFunc(h.Events, func(e engine.Event) bool { return e.Kind == engine.EvShow && e.Player == i })
}

// showedAfter reports whether player id showed their cards after hand ph.
func (ph *pastHand) showedAfter(id string) bool {
	return ph != nil && slices.Contains(ph.showed, id)
}

// showLines narrates who showed their cards after hand ph.
func (r *Room) showLines(ph *pastHand) []string {
	var out []string
	for _, id := range ph.showed {
		i := slices.Index(ph.ids, id)
		if i < 0 {
			continue
		}
		cs := make([]string, len(ph.hand.Players[i].Hole))
		for j, c := range ph.hand.Players[i].Hole {
			cs[j] = c.Pretty()
		}
		out = append(out, fmt.Sprintf("%s shows %s", ph.hand.Players[i].ID, strings.Join(cs, " ")))
	}
	return out
}
