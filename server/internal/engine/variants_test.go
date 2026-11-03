package engine

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

func TestOmahaAndShowTwoScenarios(t *testing.T) {
	scenarios := []scenario{
		// --- Omaha (docs/rules.md §4.2) ----------------------------------
		{
			// P0's four hearts would be a flush in Hold'em, but Omaha uses
			// exactly two hole cards, so it's a pair of kings; P1's two fours
			// make trips with the board four.
			name:      "omaha: exactly two hole cards decide the hand",
			variant:   Omaha,
			stacks:    []int64{1000, 1000},
			holes:     []string{"Ah Kh Qh Jh", "4d 4h 5c 6s"},
			board:     "9h 4c 7d 2s Kc",
			actions:   seq([]string{"0 call", "1 check"}, checkDown(3, 1, 0)),
			wantFinal: []int64{980, 1020},
			check: func(t *testing.T, h *Hand) {
				for i, want := range []string{"Pair of Kings", "Three of a kind, Fours"} {
					if got := showDesc(h, i); got != want {
						t.Errorf("player %d shows %q, want %q", i, got, want)
					}
				}
			},
		},
		{
			name:     "omaha: pot-limit raise capped at the pot",
			variant:  Omaha,
			stacks:   thousand3,
			holes:    []string{"Ah Kh Qh Jh", "4d 4h 5c 6s", "Ts 9s 8s 7s"},
			board:    "9h 4c 7d 2s Kc",
			potLimit: true,
			actions:  []string{"0 raise 71"},
			wantErr:  ErrIllegalAction,
		},

		// --- Show 2 Play 2 (docs/rules.md §4.6) --------------------------
		{
			// Example 4.6-a: P0 shows the aces, keeps K♣ 5♥ hidden. One shown
			// + one hidden + three board makes kings full of aces; aces full
			// would need both shown aces.
			name:    "show2: rules example 4.6-a played out",
			variant: ShowTwo,
			stacks:  []int64{1000, 1000},
			holes:   []string{"As Ad Kc 5h", "Qh Qd 9h 3c"},
			board:   "Ac Kd Kh 9s 2c",
			// Heads-up, dealer 0: P1 (left of the dealer) shows first, then
			// P0; then betting starts as usual with the small blind.
			actions:   seq([]string{"1 show Qh Qd", "0 show As Ad", "0 call", "1 check"}, checkDown(3, 1, 0)),
			wantFinal: []int64{1020, 980},
			check: func(t *testing.T, h *Hand) {
				if got := showDesc(h, 0); got != "Full house, Kings full of Aces" {
					t.Errorf("P0 shows %q", got)
				}
				if got := showDesc(h, 1); got != "Two pair, Kings and Nines" {
					t.Errorf("P1 shows %q", got)
				}
			},
		},
		{
			name:    "show2: three players show in seat order from the dealer's left, then bet",
			variant: ShowTwo,
			stacks:  thousand3,
			holes:   []string{"As Ad Kc 5h", "Qh Qd 9h 3c", "Js Jd 8h 7h"},
			board:   "Ac Kd Kh 9s 2c",
			actions: []string{"1 show Qh Qd", "2 show Js Jd", "0 show As Ad", "0 fold", "1 call", "2 check"},
			check: func(t *testing.T, h *Hand) {
				if h.Showing || h.Street != Flop {
					t.Fatalf("showing %v street %v", h.Showing, h.Street)
				}
			},
		},
		{name: "show2: showing out of turn", variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c",
			actions: []string{"0 show As Ad"}, wantErr: ErrNotYourTurn},
		{name: "show2: betting before everyone has shown", variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c",
			actions: []string{"1 call"}, wantErr: ErrIllegalAction},
		{name: "show2: showing one card", variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c",
			actions: []string{"1 show Qh"}, wantErr: ErrIllegalAction},
		{name: "show2: showing three cards", variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c",
			actions: []string{"1 show Qh Qd 9h"}, wantErr: ErrIllegalAction},
		{name: "show2: showing a card you don't hold", variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c",
			actions: []string{"1 show Qh As"}, wantErr: ErrIllegalAction},
		{name: "show2: showing the same card twice", variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c",
			actions: []string{"1 show Qh Qh"}, wantErr: ErrIllegalAction},
		{
			name:      "show2: a player all-in on the blind still shows, then the board runs out",
			variant:   ShowTwo,
			stacks:    []int64{1000, 15},
			holes:     []string{"As Ad Kc 5h", "Qh Qd 9h 3c"},
			board:     "Ac Kd Kh 9s 2c",
			actions:   []string{"1 show Qh Qd", "0 show As Ad", "0 call"},
			wantFinal: []int64{1015, 0},
		},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.run(t) })
	}
}

func showDesc(h *Hand, player int) string {
	for _, e := range h.Events {
		if e.Kind == EvShow && e.Player == player {
			return e.Desc
		}
	}
	return ""
}

func TestShowTwoRevealIsPublicAndLegal(t *testing.T) {
	s := scenario{variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c"}
	h := s.newHand(t)
	if !h.Showing || h.ToAct != 1 {
		t.Fatalf("showing %v, to act %d", h.Showing, h.ToAct)
	}
	if got := h.Legal(1); got != (Legal{Show: true}) {
		t.Fatalf("legal during reveal = %+v", got)
	}
	if err := h.Apply(1, Action{Type: Show, Cards: cards.MustParseList("Qh Qd")}); err != nil {
		t.Fatal(err)
	}
	// Everyone, including the other player, sees the shown cards, but not
	// the hidden ones.
	text := strings.Join(h.NarrateFor(0), "\n")
	if !strings.Contains(text, "P1 shows Q♥ Q♦") || strings.Contains(text, "9♥") {
		t.Fatalf("P0's view:\n%s", text)
	}
	if hidden := h.Players[1].Hidden(); !slices.Equal(hidden, cards.MustParseList("9h 3c")) {
		t.Fatalf("hidden = %v", hidden)
	}
}

func TestDefaultShowRevealsLowestCards(t *testing.T) {
	s := scenario{variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c"}
	h := s.newHand(t)
	a := h.DefaultShow(1)
	if a.Type != Show || !slices.Equal(a.Cards, cards.MustParseList("3c 9h")) {
		t.Fatalf("default show = %v", a)
	}
	if err := h.Apply(1, a); err != nil {
		t.Fatal(err)
	}
	if got := h.DefaultShow(0).Cards; !slices.Equal(got, cards.MustParseList("5h Kc")) {
		t.Fatalf("default show for P0 = %v", got)
	}
}

func TestShowTwoLogReplays(t *testing.T) {
	h := scenario{
		variant: ShowTwo,
		stacks:  []int64{1000, 1000},
		holes:   []string{"As Ad Kc 5h", "Qh Qd 9h 3c"},
		board:   "Ac Kd Kh 9s 2c",
		actions: seq([]string{"1 show Qh Qd", "0 show As Ad", "0 call", "1 check"}, checkDown(3, 1, 0)),
	}.run(t)
	b, err := json.Marshal(h.Log())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"cards":["Qh","Qd"]`) {
		t.Fatalf("log missing shown cards: %s", b)
	}
	var l Log
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	r, err := Replay(l)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stacksOf(r), stacksOf(h)) {
		t.Fatalf("replay %v != %v", stacksOf(r), stacksOf(h))
	}
}

func TestNewVariantsRegistered(t *testing.T) {
	for _, name := range []string{"holdem", "omaha", "show2"} {
		if _, err := VariantByName(name); err != nil {
			t.Error(err)
		}
	}
	// Ten players fit every variant so far: 10*4 + 5 = 45 cards.
	if cardsNeeded(ShowTwo, 10) > cards.NumCards || cardsNeeded(Omaha, 10) > cards.NumCards {
		t.Fatal("4-card variants should fit 10 players")
	}
}
