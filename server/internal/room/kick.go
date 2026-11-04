package room

import (
	"errors"
	"slices"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// Players can vote someone out of the room. A vote passes once more than
// half of the connected players other than the target say yes; counting
// only connected players means people who have left can't block it. There
// is no time limit: a vote stays open until it passes, can no longer pass,
// or its starter withdraws it.
//
// A kicked player stays in r.players, marked Kicked, so their hands and
// profit still add up and their name can't be reused. They get the default
// move whenever a hand they're still in waits on them, are never dealt in
// again, and can't reconnect.

// ErrKicked is returned when a removed player tries to come back.
var ErrKicked = errors.New("you were removed from this room by a vote of the other players")

type kickVote struct {
	target, startedBy string
	yes, no           map[string]bool
}

func (r *Room) voteKick(p *Player, target string, yes bool) error {
	t := r.player(target)
	switch {
	case r.phase == protocol.PhaseEnded:
		return ErrGameOver
	case t == nil || t.Kicked:
		return errors.New("no such player")
	case t.ID == p.ID:
		return errors.New("you can't vote on yourself")
	}
	v := r.kickVote(target)
	if v == nil {
		if !yes {
			return errors.New("there's no vote to answer")
		}
		v = &kickVote{target: target, startedBy: p.ID, yes: map[string]bool{}, no: map[string]bool{}}
		r.kickVotes = append(r.kickVotes, v)
	}
	delete(v.yes, p.ID)
	delete(v.no, p.ID)
	if yes {
		v.yes[p.ID] = true
	} else {
		v.no[p.ID] = true
	}
	r.resolveKicks()
	return nil
}

func (r *Room) cancelKick(p *Player, target string) error {
	v := r.kickVote(target)
	switch {
	case v == nil:
		return errors.New("there's no vote to cancel")
	case v.startedBy != p.ID:
		return errors.New("only whoever started the vote can cancel it")
	}
	r.dropKickVote(target)
	return nil
}

func (r *Room) kickVote(target string) *kickVote {
	for _, v := range r.kickVotes {
		if v.target == target {
			return v
		}
	}
	return nil
}

func (r *Room) dropKickVote(target string) {
	r.kickVotes = slices.DeleteFunc(r.kickVotes, func(v *kickVote) bool { return v.target == target })
}

// kickVoters are the players whose votes count against target right now.
func (r *Room) kickVoters(target string) []*Player {
	var out []*Player
	for _, p := range r.players {
		if p.ID != target && !p.Kicked && p.Connections > 0 {
			out = append(out, p)
		}
	}
	return out
}

// tally counts the current voters' answers and how many yeses are needed.
func (r *Room) tally(v *kickVote) (yes, no, needed int) {
	voters := r.kickVoters(v.target)
	for _, p := range voters {
		switch {
		case v.yes[p.ID]:
			yes++
		case v.no[p.ID]:
			no++
		}
	}
	return yes, no, len(voters)/2 + 1
}

// resolveKicks settles every vote that has passed or can no longer pass.
// Call it after a vote and whenever someone connects or disconnects, since
// that changes the majority.
func (r *Room) resolveKicks() {
	for _, v := range slices.Clone(r.kickVotes) {
		yes, no, needed := r.tally(v)
		voters := len(r.kickVoters(v.target))
		switch {
		case yes >= needed:
			r.dropKickVote(v.target)
			r.kick(r.player(v.target))
		case no > voters-needed:
			r.dropKickVote(v.target)
		}
	}
}

func (r *Room) kick(p *Player) {
	p.Kicked = true
	p.SittingOut = true
	if r.hostID == p.ID {
		r.hostID = r.nextHost()
	}
	switch r.stage {
	case protocol.StageChoosing:
		if r.dealerID == p.ID {
			r.nextHand() // the button moves on and the next dealer chooses
		}
	case protocol.StageHand:
		r.afterHandChange() // plays their turn if the hand is waiting on them
	}
}

// nextHost picks a new host: the first connected player still in the room,
// else the first one still in it.
func (r *Room) nextHost() string {
	for _, q := range r.players {
		if !q.Kicked && q.Connections > 0 {
			return q.ID
		}
	}
	for _, q := range r.players {
		if !q.Kicked {
			return q.ID
		}
	}
	return ""
}

// kickedAwaited is the index of a kicked player the hand is waiting on, or -1.
func (r *Room) kickedAwaited() int {
	for i, id := range r.handIDs {
		if p := r.player(id); p != nil && p.Kicked && r.hand.Legal(i) != (engine.Legal{}) {
			return i
		}
	}
	return -1
}

// seated counts players who haven't been kicked.
func (r *Room) seated() int {
	n := 0
	for _, p := range r.players {
		if !p.Kicked {
			n++
		}
	}
	return n
}

func (r *Room) kickVoteViews() []protocol.KickVote {
	out := []protocol.KickVote{}
	for _, v := range r.kickVotes {
		_, _, needed := r.tally(v)
		kv := protocol.KickVote{Target: v.target, StartedBy: v.startedBy, Yes: []string{}, No: []string{}, Needed: needed}
		// Only votes that count right now: from players still connected.
		for _, p := range r.kickVoters(v.target) {
			switch {
			case v.yes[p.ID]:
				kv.Yes = append(kv.Yes, p.ID)
			case v.no[p.ID]:
				kv.No = append(kv.No, p.ID)
			}
		}
		out = append(out, kv)
	}
	return out
}
