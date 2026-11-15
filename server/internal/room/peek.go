package room

import (
	"errors"
	"slices"

	"pokerroom/internal/protocol"
)

// Peekaboo's peek (docs/rules.md §4.10): a player whose die gives them a
// board card may look at it once. They turn it over (MsgPeek) and back
// (MsgPeekDone), and it can't be reopened. There's no timer: it stays open
// until they close it, and it's out of the way once that card is dealt.
// This is room state, not part of the hand: the hand only fixes which card.

const (
	peekOpen = 1
	peekUsed = 2
)

// peekable reports whether player p's peeked card is still face down on
// the board: there's a card, and it hasn't been dealt yet.
func (r *Room) peekable(p *Player) (card string, ok bool) {
	if r.stage != protocol.StageHand || r.hand == nil || r.hand.Done {
		return "", false
	}
	i := slices.Index(r.handIDs, p.ID)
	if i < 0 {
		return "", false
	}
	c, ok := r.hand.PeekCard(i)
	if !ok || len(r.hand.Boards[0]) >= r.hand.Players[i].Die {
		return "", false
	}
	return c.String(), true
}

func (r *Room) peek(p *Player) error {
	if _, ok := r.peekable(p); !ok || r.peeks[p.ID] != 0 {
		return errors.New("there's no card for you to peek at")
	}
	if r.peeks == nil {
		r.peeks = map[string]int{}
	}
	r.peeks[p.ID] = peekOpen
	return nil
}

func (r *Room) peekDone(p *Player) error {
	if r.peeks[p.ID] != peekOpen {
		return errors.New("you're not peeking")
	}
	r.peeks[p.ID] = peekUsed
	return nil
}

// peekView fills in the viewer's own peek.
func (r *Room) peekView(v *protocol.PlayerView, i int) {
	p := r.player(r.handIDs[i])
	card, ok := r.peekable(p)
	switch {
	case !ok:
	case r.peeks[p.ID] == peekOpen:
		v.PeekCard = card
	case r.peeks[p.ID] == peekUsed:
		v.PeekUsed = true
	default:
		v.CanPeek = true
	}
}
