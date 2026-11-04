package room

import (
	"errors"
	"slices"
	"testing"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

func online(ps ...*Player) {
	for _, p := range ps {
		p.Connections = 1
	}
}

func voteKick(target *Player, yes bool) protocol.ClientMsg {
	return protocol.ClientMsg{Type: protocol.MsgVoteKick, Target: target.ID, Yes: yes}
}

func TestKickVotePassesWithMajority(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	ann, bob, cy, dee := h.ps[0], h.ps[1], h.ps[2], h.ps[3]
	online(h.ps...)

	if err := h.do(dee, voteKick(dee, true)); err == nil {
		t.Fatal("nobody can vote on themselves")
	}
	if err := h.do(ann, voteKick(dee, false)); err == nil {
		t.Fatal("a vote can't be started with a no")
	}
	h.must(ann, voteKick(dee, true))
	if err := h.do(dee, voteKick(dee, false)); err == nil {
		t.Fatal("the target doesn't vote")
	}
	// Three voters (everyone but Dee): two yeses needed.
	v := h.r.View(bob.ID).KickVotes
	if len(v) != 1 || v[0].Target != dee.ID || v[0].StartedBy != ann.ID || v[0].Needed != 2 || !slices.Equal(v[0].Yes, []string{ann.ID}) {
		t.Fatalf("votes %+v", v)
	}
	h.must(bob, voteKick(dee, false))
	if dee.Kicked {
		t.Fatal("one yes of two needed")
	}
	h.must(bob, voteKick(dee, true)) // changed their mind
	if !dee.Kicked {
		t.Fatal("two yeses should remove Dee")
	}
	if len(h.r.kickVotes) != 0 {
		t.Fatal("the vote is over")
	}

	// Dee is out: can't act, can't come back under that name, and isn't
	// counted towards the room's size.
	if err := h.do(dee, protocol.ClientMsg{Type: protocol.MsgChat, Text: "hi"}); !errors.Is(err, ErrKicked) {
		t.Fatalf("kicked player's message: %v", err)
	}
	if _, err := h.r.Join("Dee"); err == nil {
		t.Fatal("the name stays taken")
	}
	if h.r.seated() != 3 {
		t.Fatalf("seated %d", h.r.seated())
	}
	_ = cy
}

func TestKickVoteEndsWhenItCannotPass(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	ann, bob, cy, dee := h.ps[0], h.ps[1], h.ps[2], h.ps[3]
	online(h.ps...)
	h.must(ann, voteKick(dee, true))
	h.must(bob, voteKick(dee, false))
	h.must(cy, voteKick(dee, false)) // 1 yes of 3 voters: 2 can't be reached
	if dee.Kicked || len(h.r.kickVotes) != 0 {
		t.Fatalf("kicked %v, votes %d", dee.Kicked, len(h.r.kickVotes))
	}
}

func TestKickMajorityCountsOnlyConnectedPlayers(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee", "Eve")
	ann, bob, cy, dee, eve := h.ps[0], h.ps[1], h.ps[2], h.ps[3], h.ps[4]
	online(h.ps...)
	h.must(ann, voteKick(eve, true))
	h.must(bob, voteKick(eve, true))
	if eve.Kicked {
		t.Fatal("2 of 4 voters isn't a majority")
	}
	// Cy and Dee leave: now 2 of 2.
	cy.Connections, dee.Connections = 0, 0
	h.r.resolveKicks()
	if !eve.Kicked {
		t.Fatal("players who left shouldn't block the vote")
	}

	// Alone at the table, one yes is a majority.
	h2 := newHarness(t, "Ann", "Bob")
	online(h2.ps[0])
	h2.must(h2.ps[0], voteKick(h2.ps[1], true))
	if !h2.ps[1].Kicked {
		t.Fatal("the only voter decides")
	}
}

func TestCancelKick(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	ann, bob, _, dee := h.ps[0], h.ps[1], h.ps[2], h.ps[3]
	online(h.ps...)
	h.must(ann, voteKick(dee, true))
	if err := h.do(bob, protocol.ClientMsg{Type: protocol.MsgCancelKick, Target: dee.ID}); err == nil {
		t.Fatal("only the starter cancels")
	}
	h.must(ann, protocol.ClientMsg{Type: protocol.MsgCancelKick, Target: dee.ID})
	if len(h.r.kickVotes) != 0 || dee.Kicked {
		t.Fatal("vote should be gone")
	}
}

func TestKickingTheHostPassesHosting(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	ann, bob, cy := h.ps[0], h.ps[1], h.ps[2]
	online(h.ps...)
	bob.Connections = 0 // away: Cy is the only voter, and the only one here
	h.must(cy, voteKick(ann, true))
	if !ann.Kicked {
		t.Fatal("Cy is the only voter, so one yes is enough")
	}
	if h.r.hostID != cy.ID {
		t.Fatalf("host is %s, want Cy (the first player still connected)", h.r.player(h.r.hostID).Name)
	}
}

func TestKickMidHand(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	online(h.ps...)
	h.start()

	// Kick the player whose turn it is: they get the default move at once.
	target := h.toAct()
	var voters []*Player
	for _, p := range h.ps {
		if p != target {
			voters = append(voters, p)
		}
	}
	h.must(voters[0], voteKick(target, true))
	h.must(voters[1], voteKick(target, true))
	if !target.Kicked {
		t.Fatal("not kicked")
	}
	i := slices.Index(h.r.handIDs, target.ID)
	if h.r.hand.ToAct == i && !h.r.hand.Done {
		t.Fatal("the hand should have moved past the kicked player")
	}

	// The hand plays on to the end; chips are all accounted for.
	h.playPassively()
	if got := h.totalStacks(); got != 8000 {
		t.Fatalf("chips %d", got)
	}
	// The kicked player is never dealt in again.
	h.advance()
	if slices.Contains(h.r.handIDs, target.ID) {
		t.Fatal("kicked player dealt in")
	}
	// The result shows them on the leaderboard, marked.
	lb := h.r.View(voters[0].ID).Leaderboard
	if j := slices.IndexFunc(lb, func(s protocol.Standing) bool { return s.ID == target.ID }); j < 0 || !lb[j].Kicked {
		t.Fatalf("leaderboard %+v", lb)
	}
}

func TestKickedPlayerLaterInTheHandIsSkipped(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	online(h.ps...)
	h.start()
	// Someone who isn't to act now.
	actor := h.toAct()
	var target *Player
	for _, p := range h.ps {
		if p != actor {
			target = p
			break
		}
	}
	var n int
	for _, p := range h.ps {
		if p != target && n < 2 {
			h.must(p, voteKick(target, true))
			n++
		}
	}
	if !target.Kicked {
		t.Fatal("not kicked")
	}
	// Nobody ever has to wait on them.
	for h.r.stage == protocol.StageHand && !h.r.hand.Done {
		if p := h.toAct(); p == target {
			t.Fatal("the hand waited on a kicked player")
		}
		h.playPassively()
	}
	if got := h.totalStacks(); got != 8000 {
		t.Fatalf("chips %d", got)
	}
}

func TestKickWhileTheDealerIsChoosing(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: engine.AllVariants()}, "Ann", "Bob", "Cy") // the dealer chooses
	online(h.ps...)
	h.start()
	if h.r.stage != protocol.StageChoosing {
		t.Fatalf("stage %q", h.r.stage)
	}
	dealer := h.r.player(h.r.dealerID)
	for _, p := range h.ps {
		if p != dealer {
			h.must(p, voteKick(dealer, true))
		}
	}
	// The button moves on: someone still here picks the game.
	if !dealer.Kicked || h.r.stage != protocol.StageChoosing || h.r.dealerID == dealer.ID {
		t.Fatalf("kicked %v, stage %q, dealer %s", dealer.Kicked, h.r.stage, h.r.dealerID)
	}
	h.advance()
	if slices.Contains(h.r.handIDs, dealer.ID) {
		t.Fatal("the kicked dealer shouldn't be dealt in")
	}
}

func TestKickedPlayersStayOutOfARematch(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	online(h.ps...)
	h.must(h.ps[0], voteKick(h.ps[2], true))
	h.must(h.ps[1], voteKick(h.ps[2], true))
	_, seats := h.r.rematchSeats()
	if len(seats) != 2 || slices.Contains(seats, h.ps[2]) {
		t.Fatalf("seats %v", seats)
	}
}

func TestKickStateSurvivesRestart(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee", "Eve")
	online(h.ps...)
	h.start()
	h.must(h.ps[0], voteKick(h.ps[4], true))
	h.must(h.ps[1], voteKick(h.ps[4], true))
	h.must(h.ps[2], voteKick(h.ps[4], true)) // Eve is out
	h.must(h.ps[0], voteKick(h.ps[3], true)) // a vote on Dee is open
	h.must(h.ps[1], voteKick(h.ps[3], false))

	back := roundTrip(t, h.r)
	for _, p := range h.r.players {
		back.player(p.ID).Connections = p.Connections
	}
	for _, p := range h.ps[:4] {
		if got, want := viewJSON(t, back, p.ID), viewJSON(t, h.r, p.ID); got != want {
			t.Fatalf("%s's view after restore\n got: %s\nwant: %s", p.Name, got, want)
		}
	}
	if !back.player(h.ps[4].ID).Kicked || len(back.kickVotes) != 1 {
		t.Fatal("kick state lost")
	}
}

// Through the actor: the kicked player's connection is told and closed, and
// their token no longer gets them back in.
func TestKickedClientIsRemoved(t *testing.T) {
	m := newTestManager()
	code, annToken, _ := m.Create("Ann")
	a := m.Get(code)
	tokens := map[string]string{"Ann": annToken}
	a.Inspect(func(r *Room) {
		for _, n := range []string{"Bob", "Cy"} {
			p, _ := r.Join(n)
			tokens[n] = p.Token
		}
	})
	clients := map[string]*Client{}
	for _, n := range []string{"Ann", "Bob", "Cy"} {
		clients[n] = NewClient()
		if err := a.Connect(clients[n], protocol.ClientMsg{Type: protocol.MsgResume, Token: tokens[n]}); err != nil {
			t.Fatal(err)
		}
	}
	var cyID string
	a.Inspect(func(r *Room) { cyID = r.PlayerByToken(tokens["Cy"]).ID })
	a.Submit(clients["Ann"], protocol.ClientMsg{Type: protocol.MsgVoteKick, Target: cyID, Yes: true})
	a.Submit(clients["Bob"], protocol.ClientMsg{Type: protocol.MsgVoteKick, Target: cyID, Yes: true})
	a.Inspect(func(*Room) {})

	select {
	case <-clients["Cy"].Done():
	default:
		t.Fatal("Cy's connection should be closed")
	}
	msgs := drain(t, clients["Cy"])
	if last := msgs[len(msgs)-1]; last.Type != protocol.MsgRemoved {
		t.Fatalf("Cy's last message %+v", last)
	}
	if err := a.Connect(NewClient(), protocol.ClientMsg{Type: protocol.MsgResume, Token: tokens["Cy"]}); !errors.Is(err, ErrKicked) {
		t.Fatalf("Cy reconnecting: %v", err)
	}
	// Everyone else still sees Cy on the leaderboard.
	a.Inspect(func(r *Room) {
		if lb := r.View(r.PlayerByToken(tokens["Ann"]).ID).Leaderboard; len(lb) != 3 {
			t.Fatalf("leaderboard %+v", lb)
		}
	})
}
