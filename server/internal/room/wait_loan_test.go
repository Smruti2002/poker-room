package room

import (
	"slices"
	"testing"

	"pokerroom/internal/protocol"
)

// bust moves all of loser's chips to winner between hands, and marks both
// online, as players at a real table are.
func bust(h *harness, loser, winner *Player) {
	winner.Stack += loser.Stack
	loser.Stack = 0
	loser.Connections, winner.Connections = 1, 1
}

func waitingIDs(h *harness, viewer *Player) []string {
	return h.r.View(viewer.ID).WaitingOnLoan
}

// Next hand waits for an online broke player to borrow or sit out.
func TestNextHandWaitsForLoan(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	h.playPassively()
	ann, bob, cy := h.ps[0], h.ps[1], h.ps[2]
	bust(h, bob, ann)
	cy.Connections = 1

	if got := waitingIDs(h, ann); !slices.Equal(got, []string{bob.ID}) {
		t.Fatalf("waiting on %v, want Bob", got)
	}
	if err := h.do(cy, protocol.ClientMsg{Type: protocol.MsgNextHand}); err == nil {
		t.Fatal("Next hand started while Bob was deciding")
	}
	h.must(bob, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
	if got := waitingIDs(h, ann); len(got) != 0 {
		t.Fatalf("still waiting on %v after the loan", got)
	}
	h.must(cy, protocol.ClientMsg{Type: protocol.MsgNextHand})
	if !slices.Contains(h.r.handIDs, bob.ID) {
		t.Fatal("Bob borrowed but wasn't dealt in")
	}
}

// Sitting out instead of borrowing lets the table go on.
func TestNextHandBrokePlayerSitsOut(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	h.playPassively()
	ann, bob := h.ps[0], h.ps[1]
	bust(h, bob, ann)
	h.must(bob, protocol.ClientMsg{Type: protocol.MsgSitOut})
	h.must(ann, protocol.ClientMsg{Type: protocol.MsgNextHand})
	if slices.Contains(h.r.handIDs, bob.ID) {
		t.Fatal("Bob sat out but was dealt in")
	}
}

// An offline broke player doesn't hold the table up.
func TestNextHandIgnoresOfflineBrokePlayer(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	h.playPassively()
	ann, bob := h.ps[0], h.ps[1]
	bust(h, bob, ann)
	bob.Connections = 0
	h.must(ann, protocol.ClientMsg{Type: protocol.MsgNextHand})
}

// The host can deal without a broke player; a loan later sits them back in.
func TestHostDealsWithoutBrokePlayer(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	h.playPassively()
	ann, bob, cy := h.ps[0], h.ps[1], h.ps[2]
	bust(h, bob, ann)
	skip := protocol.ClientMsg{Type: protocol.MsgSkip, Target: bob.ID}

	if err := h.do(cy, skip); err == nil {
		t.Fatal("a non-host dealt without Bob")
	}
	h.must(ann, skip) // ann is the host
	if h.r.stage == protocol.StageResult || slices.Contains(h.r.handIDs, bob.ID) {
		t.Fatalf("expected a hand without Bob; stage %q", h.r.stage)
	}
	if !bob.SittingOut {
		t.Fatal("Bob should be sitting out")
	}
	h.must(bob, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
	if bob.SittingOut {
		t.Fatal("borrowing when broke should sit Bob back in")
	}
	h.playPassively()
	h.advance()
	if !slices.Contains(h.r.handIDs, bob.ID) {
		t.Fatal("Bob should be dealt into the next hand")
	}
}
