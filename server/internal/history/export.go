package history

import (
	"fmt"
	"regexp"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
)

// Hand is an archived hand replayed into readable form: every player's
// cards, every action in order, the boards and who won what. It shows all
// hole cards, so it's only for after the game.
type Hand struct {
	Number  int        `json:"number"`
	Variant string     `json:"variant"`
	Blinds  [2]int64   `json:"blinds"`
	Seats   []Seat     `json:"seats"`
	Boards  [][]string `json:"boards"`
	Events  []Event    `json:"events"`
	Log     []string   `json:"log"`
}

// Seat is one player in a hand, start to finish.
type Seat struct {
	Name       string     `json:"name"`
	Dealer     bool       `json:"dealer,omitempty"`
	StartStack int64      `json:"startStack"`
	EndStack   int64      `json:"endStack"`
	Net        int64      `json:"net"`
	Mult       int64      `json:"mult"`             // chips per unit: 2 for a 6-card Dacoit player
	Picked     int        `json:"picked,omitempty"` // Dacoit: 4 or 6 cards
	Hole       []string   `json:"hole"`
	Shown      []string   `json:"shownBeforeBetting,omitempty"` // Show 2 Play 2
	Split      [][]string `json:"split,omitempty"`              // Double/Triple Texas
	Folded     bool       `json:"folded"`
	AllIn      bool       `json:"allIn"`
	Won        int64      `json:"won"`
}

// Event is one thing that happened, in order: blinds, actions, boards,
// showdowns, wins.
type Event struct {
	Kind   string     `json:"kind"`
	Player string     `json:"player,omitempty"`
	Action string     `json:"action,omitempty"`
	Units  int64      `json:"units,omitempty"` // street total after an action, in table units
	Chips  int64      `json:"chips,omitempty"` // the same in this player's chips
	Cards  []string   `json:"cards,omitempty"`
	Board  int        `json:"board,omitempty"`
	Pot    int        `json:"pot,omitempty"`
	Desc   string     `json:"desc,omitempty"`
	Best   [][]string `json:"best,omitempty"`
	Count  int        `json:"count,omitempty"`
}

var playerID = regexp.MustCompile(`\bp\d+\b`)

// Replay turns archived records into readable hands.
func Replay(records []Record) ([]Hand, error) {
	hands := make([]Hand, 0, len(records))
	for _, r := range records {
		h, err := engine.Replay(r.Log)
		if err != nil {
			return nil, fmt.Errorf("hand %d: %w", r.Number, err)
		}
		names := map[string]string{}
		for _, p := range r.Players {
			names[p.ID] = p.Name
		}
		name := func(i int) string {
			if i < 0 || i >= len(r.Players) {
				return ""
			}
			return r.Players[i].Name
		}
		out := Hand{Number: r.Number, Variant: r.Variant, Blinds: [2]int64{r.Log.Config.SmallBlind, r.Log.Config.BigBlind}}
		for i, p := range h.Players {
			start := r.Log.Seats[i].Stack
			s := Seat{
				Name: name(i), Dealer: i == h.Dealer, StartStack: start, EndStack: p.Stack, Net: p.Stack - start,
				Mult: p.Mult, Picked: p.Picked, Hole: strs(p.Hole), Shown: strs(p.Shown),
				Folded: p.Folded, AllIn: p.AllIn, Won: p.Won,
			}
			for _, pair := range p.Split {
				s.Split = append(s.Split, strs(pair))
			}
			out.Seats = append(out.Seats, s)
		}
		for _, b := range h.Boards {
			out.Boards = append(out.Boards, strs(b))
		}
		for _, e := range h.Events {
			ev := Event{
				Kind: string(e.Kind), Player: name(e.Player), Units: e.Units, Chips: e.Chips,
				Cards: strs(e.Cards), Board: e.Board, Pot: e.Pot, Desc: e.Desc, Count: e.Count,
			}
			if e.Kind == engine.EvAction {
				ev.Action = e.Action.Type.String()
			}
			for _, b := range e.Best {
				ev.Best = append(ev.Best, strs(b))
			}
			out.Events = append(out.Events, ev)
		}
		for _, line := range h.Narrate() {
			out.Log = append(out.Log, playerID.ReplaceAllStringFunc(line, func(id string) string {
				if n, ok := names[id]; ok {
					return n
				}
				return id
			}))
		}
		hands = append(hands, out)
	}
	return hands, nil
}

// strs is nil for no cards, so empty fields are left out of the JSON.
func strs(cs []cards.Card) []string {
	if len(cs) == 0 {
		return nil
	}
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.String()
	}
	return out
}
