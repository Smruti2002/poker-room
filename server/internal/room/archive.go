package room

import (
	"errors"
	"slices"

	"pokerroom/internal/history"
	"pokerroom/internal/protocol"
)

// archive hands the hand that just finished to Options.Archive.
func (r *Room) archive() {
	if r.opts.Archive == nil {
		return
	}
	rec := history.Record{
		Room:     r.Code,
		Number:   r.handNo,
		Variant:  r.variant,
		Log:      r.hand.Log(),
		Finished: r.opts.Now().UnixMilli(),
	}
	for _, id := range r.handIDs {
		rec.Players = append(rec.Players, history.Player{ID: id, Name: r.player(id).Name})
	}
	r.opts.Archive(rec)
}

// Export is a whole game's history, as downloaded from the final standings.
type Export struct {
	Room           string              `json:"room"`
	HandsPlayed    int                 `json:"handsPlayed"`
	Settings       protocol.Settings   `json:"settings"`
	FinalStandings []ExportStanding    `json:"finalStandings"`
	Chat           []protocol.ChatLine `json:"chat"`
	Hands          []history.Hand      `json:"hands"`
}

// ExportStanding is a player's result for the whole game.
type ExportStanding struct {
	Name    string `json:"name"`
	Stack   int64  `json:"finalStack"`
	Loans   int    `json:"loans"`
	Debt    int64  `json:"debt"`
	Profit  int64  `json:"profit"`
	Removed bool   `json:"removed,omitempty"`
}

// Errors from downloading the history.
var (
	ErrNoArchive   = errors.New("this server doesn't keep hand histories")
	ErrNotYourRoom = errors.New("only players in this room can download its history")
	ErrNotOver     = errors.New("the hand history can be downloaded once the game has ended")
)

// HandHistory is the whole game, for a player of this room once it's over:
// it shows every player's cards in every hand, so not during play.
func (a *Actor) HandHistory(token string) (Export, error) {
	var out Export
	err := ErrRoomClosed
	a.call(func() {
		r := a.room
		p := r.PlayerByToken(token)
		switch {
		case a.mgr == nil || a.mgr.History == nil:
			err = ErrNoArchive
			return
		case p == nil:
			err = ErrNotYourRoom
			return
		case r.phase != protocol.PhaseEnded:
			err = ErrNotOver
			return
		}
		var records []history.Record
		if records, err = a.mgr.History.Load(r.Code); err != nil {
			return
		}
		var hands []history.Hand
		if hands, err = history.Replay(records); err != nil {
			return
		}
		out = Export{Room: r.Code, HandsPlayed: r.handNo, Settings: r.settings, Chat: slices.Clone(r.chat), Hands: hands}
		out.Settings.Variants = slices.Clone(r.settings.Variants)
		if out.Chat == nil {
			out.Chat = []protocol.ChatLine{}
		}
		for _, s := range r.View(p.ID).Leaderboard {
			q := r.player(s.ID)
			out.FinalStandings = append(out.FinalStandings, ExportStanding{
				Name: q.Name, Stack: q.Stack, Loans: q.Loans, Debt: q.Debt, Profit: s.Profit, Removed: q.Kicked,
			})
		}
	})
	return out, err
}
