package room

import (
	"errors"
	"strings"
	"testing"
	"time"

	"pokerroom/internal/protocol"
)

func TestChat(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	say := func(p *Player, text string) error {
		return h.do(p, protocol.ClientMsg{Type: protocol.MsgChat, Text: text})
	}
	if err := say(h.ps[0], "  hello\x07 table  "); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "   ", "\x00\x01", strings.Repeat("x", chatMaxRunes+1)} {
		if err := say(h.ps[1], bad); err == nil {
			t.Errorf("say(%q) should fail", bad)
		}
	}
	v := h.r.View(h.ps[1].ID)
	if len(v.Chat) != 1 || v.Chat[0].Text != "hello table" || v.Chat[0].Name != "Ann" || v.Chat[0].PlayerID != "p1" {
		t.Fatalf("chat = %+v", v.Chat)
	}
}

func TestChatRateLimitAndCap(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	h := newHarnessOpts(t, Options{}, "Ann", "Bob")
	h.r.opts.Now = func() time.Time { return now }
	ann := h.ps[0]
	for i := range chatBurst {
		if err := h.do(ann, protocol.ClientMsg{Type: protocol.MsgChat, Text: "hi"}); err != nil {
			t.Fatalf("message %d: %v", i, err)
		}
	}
	if err := h.do(ann, protocol.ClientMsg{Type: protocol.MsgChat, Text: "hi"}); !errors.Is(err, errChatTooFast) {
		t.Fatalf("burst: %v", err)
	}
	// Others are not limited by Ann's burst, and Ann can talk again later.
	if err := h.do(h.ps[1], protocol.ClientMsg{Type: protocol.MsgChat, Text: "yo"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(chatWindow)
	for range chatSize {
		now = now.Add(chatWindow / chatBurst)
		if err := h.do(ann, protocol.ClientMsg{Type: protocol.MsgChat, Text: "spam"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(h.r.View(ann.ID).Chat); got != chatSize {
		t.Fatalf("kept %d lines, want %d", got, chatSize)
	}
}

func TestReactions(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	h := newHarnessOpts(t, Options{}, "Ann", "Bob")
	h.r.opts.Now = func() time.Time { return now }
	react := func(p *Player, e string) error {
		return h.do(p, protocol.ClientMsg{Type: protocol.MsgReact, Emoji: e})
	}
	if err := react(h.ps[0], "💩"); err == nil {
		t.Fatal("unknown emoji should be refused")
	}
	if err := react(h.ps[0], "😂"); err != nil {
		t.Fatal(err)
	}
	if err := react(h.ps[0], "🔥"); err != nil { // replaces Ann's first
		t.Fatal(err)
	}
	if err := react(h.ps[1], "👍"); err != nil {
		t.Fatal(err)
	}
	v := h.r.View(h.ps[1].ID)
	if len(v.Reactions) != 2 || v.Reactions[0].Emoji != "🔥" || v.Reactions[1].PlayerID != "p2" {
		t.Fatalf("reactions = %+v", v.Reactions)
	}
	now = now.Add(reactionTTL)
	if got := h.r.View(h.ps[1].ID).Reactions; len(got) != 0 {
		t.Fatalf("expired reactions still shown: %+v", got)
	}
}

// The history shows each viewer their own cards and anything shown, never
// another player's hidden cards; newest hand first; capped.
func TestHistory(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	// Hand 1: Ann (whoever acts first) folds, so her cards are never shown.
	folder := h.toAct()
	h.must(folder, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
	h.playPassively()
	h.sched.fire(t)
	h.playPassively()

	for _, viewer := range h.ps {
		hist := h.r.History(viewer.ID)
		if len(hist) != 2 || hist[0].Number != 2 || hist[1].Number != 1 {
			t.Fatalf("history numbers %+v", hist)
		}
		for _, line := range hist[1].Log {
			if strings.Contains(line, " is dealt ") && !strings.HasPrefix(line, viewer.Name+" ") {
				t.Fatalf("%s's history leaks %q", viewer.Name, line)
			}
		}
		if len(hist[1].Winners) == 0 || len(hist[1].Boards[0]) != 5 {
			t.Fatalf("hand 1 = %+v", hist[1])
		}
	}

	for range historySize + 5 {
		h.sched.fire(t)
		h.playPassively()
	}
	if got := len(h.r.History(h.ps[0].ID)); got != historySize {
		t.Fatalf("kept %d hands, want %d", got, historySize)
	}
	if got := h.r.History("nobody"); len(got[0].Log) == 0 {
		t.Fatal("spectators should still see public events")
	}
}

func TestHints(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	me := h.toAct()
	if hint := h.r.View(me.ID).Hand.Hint; hint != "" {
		t.Fatalf("pre-flop hint %q", hint)
	}
	// Call and check to the flop.
	h.must(me, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "call"}})
	h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "check"}})
	hint := h.r.View(me.ID).Hand.Hint
	if hint == "" {
		t.Fatal("no hint on the flop")
	}
	if h.r.View("nobody").Hand.Hint != "" {
		t.Fatal("spectators get no hint")
	}
	h.r.settings.Hints = false
	if h.r.View(me.ID).Hand.Hint != "" {
		t.Fatal("hints off should hide the hint")
	}
}
