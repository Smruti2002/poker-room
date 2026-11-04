package room

import (
	"errors"
	"slices"

	"pokerroom/internal/protocol"
)

// After a game ends the host can start a new game with the same players:
// a new room with the same settings, every player already seated, and each
// player moved there by their client. The old room keeps its final
// leaderboard and points to the new one, so players who were offline get
// moved too when they come back.

func (r *Room) checkRematch(p *Player) error {
	switch {
	case p.ID != r.hostID:
		return ErrNotHost
	case r.phase != protocol.PhaseEnded:
		return errors.New("end the game first")
	case r.nextCode != "":
		return errors.New("a new game has already been started")
	}
	return nil
}

// rematchSeats returns the settings for the new room and the players to
// seat there, host first so they stay host.
func (r *Room) rematchSeats() (protocol.Settings, []*Player) {
	s := r.settings
	s.Variants = slices.Clone(r.settings.Variants)
	seats := []*Player{r.player(r.hostID)}
	for _, p := range r.players {
		if p.ID != r.hostID {
			seats = append(seats, p)
		}
	}
	return s, seats
}

// rematch runs on a's goroutine: it opens the new room through the manager
// and records where everyone went.
func (a *Actor) rematch(pid string) error {
	p := a.room.player(pid)
	if p == nil {
		return errors.New("unknown player")
	}
	if err := a.room.checkRematch(p); err != nil {
		return err
	}
	if a.mgr == nil {
		return errors.New("can't start a new game here")
	}
	settings, seats := a.room.rematchSeats()
	names := make([]string, len(seats))
	for i, q := range seats {
		names[i] = q.Name
	}
	code, tokens, err := a.mgr.create(&settings, names)
	if err != nil {
		return err
	}
	a.room.nextCode = code
	a.room.nextTokens = map[string]string{}
	for i, q := range seats {
		a.room.nextTokens[q.ID] = tokens[i]
	}
	a.log.Info("new game with the same players", "next", code)
	return nil
}
