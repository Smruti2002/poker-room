package room

import (
	"fmt"
	"maps"
	"slices"

	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// snapshotVersion changes when the snapshot format does; older snapshots are
// skipped on restore rather than misread.
const snapshotVersion = 1

// Snapshot is everything needed to rebuild a room after a restart. Hands
// are stored as engine logs (seed plus actions) and replayed on restore, so
// even a hand in progress comes back exactly as it was.
type Snapshot struct {
	Version      int                 `json:"version"`
	Code         string              `json:"code"`
	Settings     protocol.Settings   `json:"settings"`
	Players      []PlayerSnapshot    `json:"players"`
	HostID       string              `json:"hostId"`
	Phase        string              `json:"phase"`
	Stage        string              `json:"stage"`
	NextID       int                 `json:"nextId"`
	HandNo       int                 `json:"handNo"`
	Variant      string              `json:"variant"`
	DealerID     string              `json:"dealerId"`
	Options      []string            `json:"options,omitempty"`
	EndAfterHand bool                `json:"endAfterHand,omitempty"`
	Hand         *engine.Log         `json:"hand,omitempty"`
	HandIDs      []string            `json:"handIds,omitempty"`
	Chat         []protocol.ChatLine `json:"chat,omitempty"`
	ChatID       int                 `json:"chatId"`
	Past         []PastSnapshot      `json:"past,omitempty"`
	NextCode     string              `json:"nextCode,omitempty"`
	NextTokens   map[string]string   `json:"nextTokens,omitempty"`
}

// PlayerSnapshot is a player as saved; the session token lets them resume.
type PlayerSnapshot struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Token      string `json:"token"`
	Stack      int64  `json:"stack"`
	Debt       int64  `json:"debt"`
	Loans      int    `json:"loans"`
	SittingOut bool   `json:"sittingOut,omitempty"`
}

// PastSnapshot is a finished hand kept for the history panel.
type PastSnapshot struct {
	Number  int        `json:"number"`
	Variant string     `json:"variant"`
	Log     engine.Log `json:"log"`
	IDs     []string   `json:"ids"`
}

// Snapshot captures the room. Reactions and rate-limit counters are
// short-lived and not saved; connections come back on their own.
func (r *Room) Snapshot() Snapshot {
	s := Snapshot{
		Version:      snapshotVersion,
		Code:         r.Code,
		Settings:     r.settings,
		HostID:       r.hostID,
		Phase:        r.phase,
		Stage:        r.stage,
		NextID:       r.nextID,
		HandNo:       r.handNo,
		Variant:      r.variant,
		DealerID:     r.dealerID,
		Options:      slices.Clone(r.options),
		EndAfterHand: r.endAfterHand,
		HandIDs:      slices.Clone(r.handIDs),
		Chat:         slices.Clone(r.chat),
		ChatID:       r.chatID,
		NextCode:     r.nextCode,
		NextTokens:   maps.Clone(r.nextTokens),
	}
	s.Settings.Variants = slices.Clone(r.settings.Variants)
	for _, p := range r.players {
		s.Players = append(s.Players, PlayerSnapshot{
			ID: p.ID, Name: p.Name, Token: p.Token, Stack: p.Stack,
			Debt: p.Debt, Loans: p.Loans, SittingOut: p.SittingOut,
		})
	}
	if r.hand != nil {
		l := r.hand.Log()
		s.Hand = &l
	}
	for _, ph := range r.past {
		s.Past = append(s.Past, PastSnapshot{Number: ph.number, Variant: ph.variant, Log: ph.hand.Log(), IDs: slices.Clone(ph.ids)})
	}
	return s
}

// Restore rebuilds a room from a snapshot, replaying its hands.
func Restore(s Snapshot, opts Options) (*Room, error) {
	if s.Version != snapshotVersion {
		return nil, fmt.Errorf("room %s: snapshot version %d, want %d", s.Code, s.Version, snapshotVersion)
	}
	r := New(s.Code, opts)
	r.settings = s.Settings
	r.hostID = s.HostID
	r.phase = s.Phase
	r.stage = s.Stage
	r.nextID = s.NextID
	r.handNo = s.HandNo
	r.variant = s.Variant
	r.dealerID = s.DealerID
	r.options = s.Options
	r.endAfterHand = s.EndAfterHand
	r.handIDs = s.HandIDs
	r.chat = s.Chat
	r.chatID = s.ChatID
	r.nextCode = s.NextCode
	r.nextTokens = s.NextTokens
	for _, p := range s.Players {
		r.players = append(r.players, &Player{
			ID: p.ID, Name: p.Name, Token: p.Token, Stack: p.Stack,
			Debt: p.Debt, Loans: p.Loans, SittingOut: p.SittingOut,
		})
	}
	if s.Hand != nil {
		h, err := engine.Replay(*s.Hand)
		if err != nil {
			return nil, fmt.Errorf("room %s: replaying hand %d: %w", s.Code, s.HandNo, err)
		}
		r.hand = h
	}
	for _, ps := range s.Past {
		h, err := engine.Replay(ps.Log)
		if err != nil {
			return nil, fmt.Errorf("room %s: replaying past hand %d: %w", s.Code, ps.Number, err)
		}
		r.past = append(r.past, pastHand{number: ps.Number, variant: ps.Variant, hand: h, ids: ps.IDs})
	}
	return r, nil
}
