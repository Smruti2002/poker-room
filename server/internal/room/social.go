package room

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

const (
	historySize  = 20  // finished hands kept for the history panel
	chatSize     = 50  // chat lines kept
	chatMaxRunes = 200 // longest chat message
	chatBurst    = 5   // messages allowed per chatWindow
	chatWindow   = 10 * time.Second
	reactionTTL  = 4 * time.Second // how long a reaction stays over a seat
)

// Emojis are the quick reactions on offer.
var Emojis = []string{"👍", "😂", "😮", "😭", "😡", "🔥", "🎉", "🤑"}

var errChatTooFast = errors.New("slow down: too many messages")

// say adds a chat message, trimmed, with control characters removed.
func (r *Room) say(p *Player, text string) error {
	text = strings.TrimSpace(strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return -1
		}
		return c
	}, text))
	if text == "" {
		return errors.New("empty message")
	}
	if utf8.RuneCountInString(text) > chatMaxRunes {
		return errors.New("message too long")
	}
	now := r.opts.Now()
	if r.chatTimes == nil {
		r.chatTimes = map[string][]time.Time{}
	}
	recent := slices.DeleteFunc(r.chatTimes[p.ID], func(t time.Time) bool { return now.Sub(t) >= chatWindow })
	if len(recent) >= chatBurst {
		r.chatTimes[p.ID] = recent
		return errChatTooFast
	}
	r.chatTimes[p.ID] = append(recent, now)

	r.chatID++
	r.chat = append(r.chat, protocol.ChatLine{ID: r.chatID, PlayerID: p.ID, Name: p.Name, Text: text, At: now.UnixMilli()})
	if len(r.chat) > chatSize {
		r.chat = r.chat[len(r.chat)-chatSize:]
	}
	return nil
}

// react shows an emoji over the player's seat for a few seconds.
func (r *Room) react(p *Player, emoji string) error {
	if !slices.Contains(Emojis, emoji) {
		return errors.New("unknown reaction")
	}
	now := r.opts.Now()
	r.pruneReactions(now)
	// One live reaction per player: a new one replaces the old.
	r.reactions = slices.DeleteFunc(r.reactions, func(x protocol.Reaction) bool { return x.PlayerID == p.ID })
	r.chatID++
	r.reactions = append(r.reactions, protocol.Reaction{ID: r.chatID, PlayerID: p.ID, Emoji: emoji, At: now.UnixMilli()})
	return nil
}

func (r *Room) pruneReactions(now time.Time) {
	r.reactions = slices.DeleteFunc(r.reactions, func(x protocol.Reaction) bool {
		return now.Sub(time.UnixMilli(x.At)) >= reactionTTL
	})
}

// History is the recent hands as player `viewer` may see them: their own
// hole cards and whatever was shown, never anyone else's hidden cards.
func (r *Room) History(viewer string) []protocol.HistoryHand {
	out := make([]protocol.HistoryHand, 0, len(r.past))
	for i := len(r.past) - 1; i >= 0; i-- { // newest first
		ph := r.past[i]
		hh := protocol.HistoryHand{
			Number:  ph.number,
			Variant: ph.variant,
			Winners: winnersOf(ph.hand, ph.ids),
			Log:     ph.hand.NarrateFor(slices.Index(ph.ids, viewer)),
		}
		for _, b := range ph.hand.Boards {
			hh.Boards = append(hh.Boards, cardStrings(b))
		}
		if hh.Log == nil {
			hh.Log = []string{}
		}
		out = append(out, hh)
	}
	return out
}

// hint is the viewer's current best hand, if hints are on.
func (r *Room) hint(h *engine.Hand, viewer string) string {
	i := slices.Index(r.handIDs, viewer)
	if !r.settings.Hints || i < 0 || h.Done {
		return ""
	}
	return h.Hint(i)
}
