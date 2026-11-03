package engine

import (
	"fmt"
	"strings"

	"pokerroom/internal/cards"
)

// EventKind identifies what happened.
type EventKind string

const (
	EvBlind  EventKind = "blind"  // Player posted Units/Chips
	EvHole   EventKind = "hole"   // Player was dealt Cards (private)
	EvAction EventKind = "action" // Player did Action; Units/Chips is their street total
	EvBoard  EventKind = "board"  // Cards were added to Board
	EvRefund EventKind = "refund" // Player's uncalled Units/Chips came back
	EvShow   EventKind = "show"   // Player showed Cards at showdown; Desc and Best are the hand
	EvReveal EventKind = "reveal" // Player showed Cards before betting (Show 2 Play 2), to everyone
	EvChoose EventKind = "choose" // Player took Count hole cards, paying Mult chips per unit (Dacoit)
	EvWin    EventKind = "win"    // Player won Chips from Pot
)

// Event is one thing that happened in a hand, in order. The room layer turns
// these into per-player messages, hiding other players' hole cards.
type Event struct {
	Kind   EventKind    `json:"kind"`
	Player int          `json:"player"` // -1 for table events
	Action Action       `json:"action,omitzero"`
	Units  int64        `json:"units,omitempty"`
	Chips  int64        `json:"chips,omitempty"`
	Cards  []cards.Card `json:"cards,omitempty"`
	Board  int          `json:"board,omitempty"`
	Pot    int          `json:"pot,omitempty"`
	Desc   string       `json:"desc,omitempty"`
	// Best is, for a show event, the five cards that make the hand on each board.
	Best [][]cards.Card `json:"best,omitempty"`
	// Count and Mult are a choose event's hole cards and chips per unit.
	Count int   `json:"count,omitempty"`
	Mult  int64 `json:"mult,omitempty"`
}

func (h *Hand) emit(e Event) { h.Events = append(h.Events, e) }

// Narrate renders every event, including all hole cards, as human-readable
// lines. It is for logs and the replay tool, never for players.
func (h *Hand) Narrate() []string { return h.narrate(func(Event) bool { return true }) }

// NarrateFor renders the events player `viewer` may see: their own hole
// cards but nobody else's. Use -1 for a spectator.
func (h *Hand) NarrateFor(viewer int) []string {
	return h.narrate(func(e Event) bool { return e.Kind != EvHole || e.Player == viewer })
}

func (h *Hand) narrate(visible func(Event) bool) []string {
	name := func(i int) string {
		if id := h.Players[i].ID; id != "" {
			return id
		}
		return fmt.Sprintf("P%d", i)
	}
	pretty := func(cs []cards.Card) string {
		s := make([]string, len(cs))
		for i, c := range cs {
			s[i] = c.Pretty()
		}
		return strings.Join(s, " ")
	}
	board := 0
	var out []string
	for _, e := range h.Events {
		if !visible(e) {
			continue
		}
		switch e.Kind {
		case EvBlind:
			out = append(out, fmt.Sprintf("%s posts %d", name(e.Player), e.Chips))
		case EvHole:
			out = append(out, fmt.Sprintf("%s is dealt %s", name(e.Player), pretty(e.Cards)))
		case EvAction:
			switch e.Action.Type {
			case Fold, Check:
				out = append(out, fmt.Sprintf("%s %ss", name(e.Player), e.Action.Type))
			case Call:
				out = append(out, fmt.Sprintf("%s calls (%d in)", name(e.Player), e.Chips))
			default:
				out = append(out, fmt.Sprintf("%s %ss to %d", name(e.Player), e.Action.Type, e.Chips))
			}
		case EvBoard:
			label := [...]string{"Flop", "Turn", "River"}[min(board/len(h.Boards), 2)]
			if len(h.Boards) > 1 {
				label += fmt.Sprintf(" (board %d)", e.Board+1)
			}
			board++
			out = append(out, fmt.Sprintf("%s: %s", label, pretty(e.Cards)))
		case EvRefund:
			out = append(out, fmt.Sprintf("%d uncalled returned to %s", e.Chips, name(e.Player)))
		case EvChoose:
			line := fmt.Sprintf("%s takes %d cards", name(e.Player), e.Count)
			if e.Mult > 1 {
				line += fmt.Sprintf(" (pays %dx)", e.Mult)
			}
			out = append(out, line)
		case EvReveal:
			out = append(out, fmt.Sprintf("%s shows %s", name(e.Player), pretty(e.Cards)))
		case EvShow:
			out = append(out, fmt.Sprintf("%s shows %s (%s)", name(e.Player), pretty(e.Cards), e.Desc))
		case EvWin:
			line := fmt.Sprintf("%s wins %d from pot %d", name(e.Player), e.Chips, e.Pot+1)
			if e.Desc != "" {
				line += " with " + e.Desc
			}
			out = append(out, line)
		}
	}
	return out
}
