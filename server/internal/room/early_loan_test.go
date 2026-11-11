package room

import (
	"slices"
	"testing"

	"pokerroom/internal/protocol"
)

func canBorrow(h *harness, p *Player) bool {
	v := h.r.View(p.ID)
	return v.Players[slices.IndexFunc(v.Players, func(q protocol.PlayerView) bool { return q.ID == p.ID })].CanBorrow
}

// Players below 500 chips may borrow between hands, before going broke.
func TestEarlyLoan(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	h.playPassively()
	ann, bob := h.ps[0], h.ps[1]
	loan := protocol.ClientMsg{Type: protocol.MsgTakeLoan}

	// Move chips so Ann has exactly the line: not low enough yet.
	total := ann.Stack + bob.Stack
	ann.Stack = EarlyLoanBelow
	bob.Stack = total - ann.Stack
	if canBorrow(h, ann) {
		t.Fatal("500 chips shouldn't offer a loan")
	}
	if err := h.do(ann, loan); err == nil {
		t.Fatal("loan at 500 chips accepted")
	}

	ann.Stack = EarlyLoanBelow - 1
	bob.Stack = total - ann.Stack
	if !canBorrow(h, ann) || canBorrow(h, bob) {
		t.Fatal("only Ann, below 500, should be offered a loan")
	}
	h.must(ann, loan)
	if ann.Stack != 499+2000 || ann.Debt != 3000 || ann.Loans != 1 {
		t.Fatalf("after loan: stack %d debt %d loans %d", ann.Stack, ann.Debt, ann.Loans)
	}
	// Chips are conserved: profits sum to minus the penalties paid.
	if sum := h.r.Profit(ann) + h.r.Profit(bob); sum != -1000 {
		t.Fatalf("profits sum to %d, want -1000", sum)
	}
}

// A player dealt into a hand can't borrow until it's over: their chips are
// in play, and the room only learns their stack when the hand ends.
func TestEarlyLoanNotDuringHand(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	h.playPassively()
	ann, bob := h.ps[0], h.ps[1]
	total := ann.Stack + bob.Stack
	ann.Stack = 300
	bob.Stack = total - ann.Stack
	h.advance() // deal the next hand, with Ann on 300
	if h.r.stage != protocol.StageHand || !slices.Contains(h.r.handIDs, ann.ID) {
		t.Fatalf("expected Ann in a hand; stage %q", h.r.stage)
	}
	if canBorrow(h, ann) {
		t.Fatal("loan offered during a hand")
	}
	if err := h.do(ann, protocol.ClientMsg{Type: protocol.MsgTakeLoan}); err == nil {
		t.Fatal("loan during a hand accepted")
	}
	h.playPassively()
	if ann.Stack < EarlyLoanBelow && !canBorrow(h, ann) {
		t.Fatal("loan should be offered once the hand is over")
	}
}

// Someone sitting out isn't in the hand, so they may borrow while it's played.
func TestEarlyLoanWhileSittingOut(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	h.playPassively()
	cy := h.ps[2]
	h.must(cy, protocol.ClientMsg{Type: protocol.MsgSitOut})
	h.ps[0].Stack += cy.Stack - 200
	cy.Stack = 200
	h.advance()
	if h.r.stage != protocol.StageHand || slices.Contains(h.r.handIDs, cy.ID) {
		t.Fatalf("expected a hand without Cy; stage %q", h.r.stage)
	}
	h.must(cy, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
	if cy.Stack != 2200 {
		t.Fatalf("Cy's stack %d, want 2200", cy.Stack)
	}
}
