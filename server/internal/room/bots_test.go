package room

import (
	mrand "math/rand/v2"
	"slices"
	"testing"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

func addBot(h *harness) {
	h.t.Helper()
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgAddBot})
}

func bots(r *Room) []*Player {
	var out []*Player
	for _, p := range r.players {
		if p.Bot {
			out = append(out, p)
		}
	}
	return out
}

func TestHostAddsAndRemovesBots(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	if err := h.do(h.ps[1], protocol.ClientMsg{Type: protocol.MsgAddBot}); err == nil {
		t.Fatal("only the host adds bots")
	}
	addBot(h)
	addBot(h)
	b := bots(h.r)
	if len(b) != 2 || b[0].Name != "Bot 1" || b[1].Name != "Bot 2" {
		t.Fatalf("bots %v", b)
	}
	if v := h.r.View(h.ps[0].ID); !v.Players[2].Bot || !v.Players[2].Connected {
		t.Fatalf("a bot shows as a bot, never offline: %+v", v.Players[2])
	}
	// In the lobby, removing a bot frees its seat and its name.
	if err := h.do(h.ps[1], protocol.ClientMsg{Type: protocol.MsgRemoveBot, Target: b[0].ID}); err == nil {
		t.Fatal("only the host removes bots")
	}
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgRemoveBot, Target: h.ps[1].ID}); err == nil {
		t.Fatal("people aren't bots")
	}
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgRemoveBot, Target: b[0].ID})
	addBot(h)
	if names := []string{h.r.players[2].Name, h.r.players[3].Name}; !slices.Contains(names, "Bot 1") || len(h.r.players) != 4 {
		t.Fatalf("players %v", names)
	}
	h.r.settings.MaxPlayers = 4
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgAddBot}); err == nil {
		t.Fatal("the room is full")
	}
}

// One person and three bots play many hands of every variant: the game
// only ever waits on the person, bots pick games when they deal and borrow
// when they're broke, and the chips add up.
func TestBotsPlayWholeGames(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: engine.AllVariants()}, "Ann")
	for range 3 {
		addBot(h)
	}
	h.r.settings.SmallBlind, h.r.settings.BigBlind = 50, 100 // so bots go broke
	for _, p := range h.r.players {
		p.Stack = 500
	}
	h.r.settings.StartingStack = 500
	h.start()
	ann := h.ps[0]
	rng := mrand.New(mrand.NewPCG(5, 6))
	hands, variants := 0, map[string]bool{}
	for hands < 120 {
		switch h.r.stage {
		case protocol.StageResult:
			hands++
			variants[h.r.variant] = true
			h.must(ann, protocol.ClientMsg{Type: protocol.MsgNextHand})
		case protocol.StageChoosing:
			if d := h.r.player(h.r.dealerID); d.Bot {
				t.Fatal("a bot dealer should pick the game straight away")
			}
			h.must(ann, protocol.ClientMsg{Type: protocol.MsgChooseVariant, Variant: h.r.options[hands%len(h.r.options)]})
		case protocol.StageWaiting:
			if ann.Stack == 0 {
				h.must(ann, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
				continue
			}
			t.Fatalf("stuck waiting: %+v", h.r.players)
		case protocol.StageHand:
			// The game is waiting on Ann, never on a bot.
			hand := h.r.hand
			i := slices.Index(h.r.handIDs, ann.ID)
			if hand.Legal(i) == (engine.Legal{}) {
				t.Fatalf("hand %d waits on a bot (to act %d, drawing %v, allocating %v)", hands, hand.ToAct, hand.Drawing, hand.Allocating)
			}
			if err := h.tryAdvanceWithMoves(rng); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(variants) < 4 {
		t.Fatalf("only played %v", variants)
	}
	var loans int64
	for _, p := range h.r.players {
		loans += int64(p.Loans) * h.r.settings.LoanAmount
	}
	if loans == 0 {
		t.Fatal("with big blinds and small stacks someone should have borrowed")
	}
	if got := h.totalStacks(); got != 4*500+loans {
		t.Fatalf("chips %d, want %d", got, 4*500+loans)
	}
}

func TestRemovingABotMidGame(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	addBot(h)
	h.start()
	b := bots(h.r)[0]
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgRemoveBot, Target: b.ID})
	if !b.Kicked {
		t.Fatal("mid-game a removed bot leaves like a kicked player")
	}
	h.playPassively()
	h.advance()
	if slices.Contains(h.r.handIDs, b.ID) {
		t.Fatal("a removed bot isn't dealt in")
	}
}

// Bots don't count as voters, and hosting never passes to one.
func TestBotsNeverHostOrVote(t *testing.T) {
	h := newHarness(t, "Ann")
	addBot(h) // seated before Bob
	bob, err := h.r.Join("Bob")
	if err != nil {
		t.Fatal(err)
	}
	online(h.ps[0], bob)
	// Bob is the only voter (the bot doesn't count), so his yes is enough.
	h.must(bob, voteKick(h.ps[0], true))
	if !h.ps[0].Kicked {
		t.Fatal("Bob alone is a majority: bots don't vote")
	}
	if h.r.hostID != bob.ID {
		t.Fatalf("host is %s, want Bob, not the bot", h.r.player(h.r.hostID).Name)
	}
}

// A new game with the same players keeps bots as bots.
func TestBotsComeAlongToARematch(t *testing.T) {
	m := newTestManager()
	code, token, _ := m.Create("Ann")
	a := m.Get(code)
	var annID string
	a.Inspect(func(r *Room) {
		annID = r.PlayerByToken(token).ID
		_ = r.Handle(annID, protocol.ClientMsg{Type: protocol.MsgAddBot})
		_ = r.Handle(annID, protocol.ClientMsg{Type: protocol.MsgStart})
		_ = r.Handle(annID, protocol.ClientMsg{Type: protocol.MsgEndGame})
		for r.phase != protocol.PhaseEnded {
			_ = r.Handle(annID, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
		}
	})
	ann := NewClient()
	if err := a.Connect(ann, protocol.ClientMsg{Type: protocol.MsgResume, Token: token}); err != nil {
		t.Fatal(err)
	}
	a.Submit(ann, protocol.ClientMsg{Type: protocol.MsgRematch})
	var next string
	a.Inspect(func(r *Room) { next = r.nextCode })
	m.Get(next).Inspect(func(r *Room) {
		if len(r.players) != 2 || !r.players[1].Bot || r.players[1].Name != "Bot 1" || r.players[0].Bot {
			t.Fatalf("new room's players %+v %+v", r.players[0], r.players[1])
		}
	})
}
