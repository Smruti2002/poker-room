package room

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/protocol"
)

func seatOf(v protocol.State, id string) protocol.PlayerView {
	return v.Players[slices.IndexFunc(v.Players, func(p protocol.PlayerView) bool { return p.ID == id })]
}

// everyoneFolds plays a hand where all but one player fold; it returns the
// winner and the first player who folded.
func everyoneFolds(h *harness) (winner, folder *Player) {
	h.t.Helper()
	folder = h.toAct()
	for h.r.stage == protocol.StageHand && !h.r.hand.Done {
		h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
	}
	for _, p := range h.ps {
		if i := slices.Index(h.r.handIDs, p.ID); i >= 0 && !h.r.hand.Players[i].Folded {
			winner = p
		}
	}
	return winner, folder
}

func TestShowCardsAfterEveryoneFolds(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	winner, folder := everyoneFolds(h)
	var other *Player
	for _, p := range h.ps {
		if p != winner && p != folder {
			other = p
		}
	}
	show := protocol.ClientMsg{Type: protocol.MsgShowCards}

	// Nobody has seen the winner's cards.
	if seatOf(h.r.View(other.ID), winner.ID).Hole != nil {
		t.Fatal("winner's cards visible before they chose to show")
	}
	if !seatOf(h.r.View(winner.ID), winner.ID).CanShowCards || seatOf(h.r.View(other.ID), winner.ID).CanShowCards {
		t.Fatal("CanShowCards should be set only in the winner's own view")
	}

	h.must(winner, show)
	v := h.r.View(other.ID)
	seat := seatOf(v, winner.ID)
	if len(seat.Hole) != 2 || !seat.ShowedCards {
		t.Fatalf("after showing: hole %v showed %v", seat.Hole, seat.ShowedCards)
	}
	if !strings.Contains(v.Log[len(v.Log)-1], " shows ") {
		t.Fatalf("no log line: %q", v.Log[len(v.Log)-1])
	}
	if err := h.do(winner, show); err == nil {
		t.Fatal("showed twice")
	}
	// The folder's cards stay hidden unless they show too, which they may.
	if seatOf(v, folder.ID).Hole != nil {
		t.Fatal("folder's cards visible")
	}
	h.must(folder, show)
	if len(seatOf(h.r.View(other.ID), folder.ID).Hole) != 2 {
		t.Fatal("folder showed, but their cards aren't visible")
	}

	// It shows in the hand history too, and survives a restart.
	hist := h.r.History(other.ID)
	if n := len(hist[0].Log); !strings.Contains(hist[0].Log[n-1], " shows ") {
		t.Fatalf("history log %v", hist[0].Log)
	}
	b, _ := json.Marshal(h.r.Snapshot())
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	r, err := Restore(s, h.r.opts)
	if err != nil {
		t.Fatal(err)
	}
	if !seatOf(r.View(other.ID), winner.ID).ShowedCards || len(seatOf(r.View(other.ID), winner.ID).Hole) != 2 {
		t.Fatal("shown cards lost after a restart")
	}

	// The next hand starts with everything hidden again.
	h.advance()
	if seatOf(h.r.View(other.ID), winner.ID).ShowedCards || h.do(winner, show) == nil {
		t.Fatal("showing should end when the next hand starts")
	}
}

func TestShowCardsOnlyWhenAllowed(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	h.must(h.ps[3], protocol.ClientMsg{Type: protocol.MsgSitOut})
	h.start()
	show := protocol.ClientMsg{Type: protocol.MsgShowCards}
	if err := h.do(h.toAct(), show); err == nil {
		t.Fatal("showed during the hand")
	}
	h.playPassively() // a showdown: the cards there are already public
	for i, id := range h.r.handIDs {
		if shownAtShowdown(h.r.hand, i) {
			if err := h.do(h.r.player(id), show); err == nil {
				t.Fatal("showed cards already shown at the showdown")
			}
		}
	}
	if err := h.do(h.ps[3], show); err == nil {
		t.Fatal("a player who sat out showed cards")
	}
}
