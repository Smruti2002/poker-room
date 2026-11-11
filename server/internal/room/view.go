package room

import (
	"cmp"
	"slices"
	"time"

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
		Chat:     append([]protocol.ChatLine{}, r.chat...),
		Now:      r.opts.Now().UnixMilli(),
	}
	s.Settings.Variants = slices.Clone(r.settings.Variants)
	s.KickVotes = r.kickVoteViews()
	if c := r.ClosesAt(); !c.IsZero() {
		s.ClosesAt = c.UnixMilli()
	}
	if r.nextCode != "" {
		s.NextRoom = &protocol.NextRoom{Code: r.nextCode, Token: r.nextTokens[viewer]}
	}
	now := r.opts.Now()
	s.Reactions = []protocol.Reaction{}
	for _, x := range r.reactions {
		if now.Sub(time.UnixMilli(x.At)) < reactionTTL {
			s.Reactions = append(s.Reactions, x)
		}
	}

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

	callouts := betCallouts(h)
	for _, p := range r.players {
		v := protocol.PlayerView{
			ID:         p.ID,
			Name:       p.Name,
			Stack:      p.Stack,
			Debt:       p.Debt,
			Loans:      p.Loans,
			Profit:     r.Profit(p),
			Connected:  p.Connections > 0 || p.Bot,
			SittingOut: p.SittingOut,
			Kicked:     p.Kicked,
			Bot:        p.Bot,
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
			v.LastBet = callouts[i]
			v.HoleCount = len(hp.Hole)
			v.Picked = hp.Picked
			v.Draws = hp.Draws // how many, not which: public
			v.Exchanging = slices.Contains(h.DrawPending(), i)
			// Others only learn that a split is locked in; the pairs are
			// shown with the hand at the showdown.
			v.Allocated = h.Allocating && hp.Split != nil
			if shown[i] || (p.ID == viewer && hp.Split != nil) {
				for _, pair := range hp.Split {
					v.Split = append(v.Split, cardStrings(pair))
				}
			}
			if hp.Shown != nil {
				v.Shown = cardStrings(hp.Shown) // public once revealed
			}
			if live {
				v.Stack = hp.Stack
			}
			if len(hp.Hole) > 0 && (p.ID == viewer || shown[i]) {
				v.Hole = cardStrings(hp.Hole)
			}
			if shown[i] {
				v.HandDesc = showDesc(h, i)
			}
		}
		v.Broke = v.Stack == 0 && (!live || !v.InHand)
		v.CanBorrow = r.phase == protocol.PhasePlaying && !p.Kicked && p.Stack < EarlyLoanBelow && !r.inLiveHand(p)
		s.Players = append(s.Players, v)

		s.Leaderboard = append(s.Leaderboard, protocol.Standing{ID: p.ID, Name: p.Name, Profit: r.Profit(p), Kicked: p.Kicked})
	}
	slices.SortStableFunc(s.Leaderboard, func(a, b protocol.Standing) int { return cmp.Compare(b.Profit, a.Profit) })

	if h != nil {
		s.Hand = r.handView(viewer)
		// Keep Log a JSON array even before anything has happened (a Dacoit
		// hand has no events until everyone has chosen).
		if lines := h.NarrateFor(slices.Index(r.handIDs, viewer)); lines != nil {
			s.Log = lines
		}
	}
	if r.stage == protocol.StageChoosing {
		s.Choosing = &protocol.ChoosingView{
			DealerID: r.dealerID,
			Options:  slices.Clone(r.options),
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
		Picking:    h.Picking,
		Allocating: h.Allocating,
		Drawing:    h.Drawing,
		DrawRound:  h.DrawRound,
		Reveal:     h.Variant.Reveal(),
		Pot:        h.PotChips(),
		CurrentBet: h.CurrentBet,
		Hint:       r.hint(h, viewer),
		Done:       h.Done,
	}
	hv.Boards = [][]string{} // a list even with no board (BB), never null
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
		for _, ids := range h.BoardWinners() {
			var names []string
			for _, i := range ids {
				names = append(names, r.handIDs[i])
			}
			hv.BoardWinners = append(hv.BoardWinners, names)
		}
		return hv
	}
	if h.ToAct >= 0 {
		hv.ToAct = r.handIDs[h.ToAct]
	}
	// Usually only the player to act has options, but everyone still in
	// splits at once in Double/Triple Texas.
	if i := slices.Index(r.handIDs, viewer); i >= 0 && h.Legal(i) != (engine.Legal{}) {
		l := h.Legal(i)
		hv.Legal = &protocol.Legal{
			Show:   l.Show,
			Choose: l.Choose, ChooseMax: l.ChooseMax,
			Allocate: l.Allocate, Draw: l.Draw,
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
func (r *Room) winners() []protocol.WinnerView { return winnersOf(r.hand, r.handIDs) }

// winnersOf totals each winner's chips across pots in a finished hand.
func winnersOf(h *engine.Hand, handIDs []string) []protocol.WinnerView {
	var out []protocol.WinnerView
	for _, e := range h.Events {
		if e.Kind != engine.EvWin {
			continue
		}
		id := handIDs[e.Player]
		i := slices.IndexFunc(out, func(w protocol.WinnerView) bool { return w.PlayerID == id })
		if i < 0 {
			out = append(out, protocol.WinnerView{PlayerID: id, Desc: e.Desc})
			i = len(out) - 1
		}
		out[i].Chips += e.Chips
	}
	for i := range out {
		for _, e := range h.Events {
			if e.Kind == engine.EvShow && handIDs[e.Player] == out[i].PlayerID {
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
