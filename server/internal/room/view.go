package room

import (
	"cmp"
	"slices"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// View is the room as player `viewer` may see it. This is the only place
// that decides what is visible: another player's hole cards appear only after
// they are shown at showdown.
func (r *Room) View(viewer string) protocol.State {
	s := protocol.State{
		Code:     r.Code,
		Phase:    r.phase,
		Stage:    r.stage,
		You:      viewer,
		HostID:   r.hostID,
		Settings: r.settings,
		Variants: r.variantNames,
		Log:      []string{},
		Now:      r.opts.Now().UnixMilli(),
	}
	s.Settings.Variants = slices.Clone(r.settings.Variants)

	h := r.hand
	live := h != nil && !h.Done
	shown := map[int]bool{}
	if h != nil {
		for _, e := range h.Events {
			if e.Kind == engine.EvShow {
				shown[e.Player] = true
			}
		}
	}

	for _, p := range r.players {
		v := protocol.PlayerView{
			ID:         p.ID,
			Name:       p.Name,
			Stack:      p.Stack,
			Debt:       p.Debt,
			Loans:      p.Loans,
			Profit:     r.Profit(p),
			Connected:  p.Connections > 0,
			SittingOut: p.SittingOut,
			Mult:       1,
		}
		if i := slices.Index(r.handIDs, p.ID); h != nil && i >= 0 {
			hp := h.Players[i]
			v.InHand = true
			v.Dealer = i == h.Dealer
			v.Folded = hp.Folded
			v.AllIn = hp.AllIn
			v.Bet = hp.Street
			v.Mult = hp.Mult
			v.Won = hp.Won
			v.HoleCount = len(hp.Hole)
			if hp.Shown != nil {
				v.Shown = cardStrings(hp.Shown) // public once revealed
			}
			if live {
				v.Stack = hp.Stack
			}
			if p.ID == viewer || shown[i] {
				v.Hole = cardStrings(hp.Hole)
			}
			if shown[i] {
				v.HandDesc = showDesc(h, i)
			}
		}
		v.Broke = v.Stack == 0 && (!live || !v.InHand)
		s.Players = append(s.Players, v)

		s.Leaderboard = append(s.Leaderboard, protocol.Standing{ID: p.ID, Name: p.Name, Profit: r.Profit(p)})
	}
	slices.SortStableFunc(s.Leaderboard, func(a, b protocol.Standing) int { return cmp.Compare(b.Profit, a.Profit) })

	if h != nil {
		s.Hand = r.handView(viewer)
		s.Log = h.NarrateFor(slices.Index(r.handIDs, viewer))
	}
	if r.stage == protocol.StageChoosing {
		s.Choosing = &protocol.ChoosingView{
			DealerID: r.dealerID,
			Options:  slices.Clone(r.options),
			Deadline: r.deadline.UnixMilli(),
		}
	}
	return s
}

func (r *Room) handView(viewer string) *protocol.HandView {
	h := r.hand
	hv := &protocol.HandView{
		Number:     r.handNo,
		Variant:    r.variant,
		Street:     h.Street.String(),
		Showing:    h.Showing,
		Reveal:     h.Variant.Reveal(),
		Pot:        h.PotChips(),
		CurrentBet: h.CurrentBet,
		Done:       h.Done,
	}
	for _, b := range h.Boards {
		hv.Boards = append(hv.Boards, cardStrings(b))
	}
	if h.Done {
		for _, pot := range h.Pots {
			pv := protocol.PotView{Chips: pot.Chips}
			for _, i := range pot.Eligible {
				pv.Eligible = append(pv.Eligible, r.handIDs[i])
			}
			hv.Pots = append(hv.Pots, pv)
		}
		hv.Winners = r.winners()
		if r.stage == protocol.StageResult {
			hv.Deadline = r.deadline.UnixMilli()
		}
		return hv
	}
	hv.ToAct = r.handIDs[h.ToAct]
	hv.Deadline = r.deadline.UnixMilli()
	if hv.ToAct == viewer {
		l := h.Legal(h.ToAct)
		hv.Legal = &protocol.Legal{
			Show: l.Show,
			Fold: l.Fold, Check: l.Check, Call: l.Call, CallUnits: l.CallUnits,
			Bet: l.Bet, Raise: l.Raise, MinTo: l.MinTo, MaxTo: l.MaxTo,
		}
	}
	return hv
}

func showDesc(h *engine.Hand, player int) string {
	for _, e := range h.Events {
		if e.Kind == engine.EvShow && e.Player == player {
			return e.Desc
		}
	}
	return ""
}

func cardStrings(cs []cards.Card) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.String()
	}
	return out
}

// winners totals each winner's chips across pots, with their shown hand.
func (r *Room) winners() []protocol.WinnerView {
	h := r.hand
	var out []protocol.WinnerView
	for _, e := range h.Events {
		if e.Kind != engine.EvWin {
			continue
		}
		id := r.handIDs[e.Player]
		i := slices.IndexFunc(out, func(w protocol.WinnerView) bool { return w.PlayerID == id })
		if i < 0 {
			out = append(out, protocol.WinnerView{PlayerID: id, Desc: e.Desc})
			i = len(out) - 1
		}
		out[i].Chips += e.Chips
	}
	for i := range out {
		for _, e := range h.Events {
			if e.Kind == engine.EvShow && r.handIDs[e.Player] == out[i].PlayerID {
				for _, b := range e.Best {
					out[i].Cards = append(out[i].Cards, cardStrings(b))
				}
			}
		}
	}
	// Biggest winner first.
	slices.SortStableFunc(out, func(a, b protocol.WinnerView) int { return cmp.Compare(b.Chips, a.Chips) })
	return out
}
