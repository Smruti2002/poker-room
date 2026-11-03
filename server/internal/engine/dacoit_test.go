package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// The deal for docs/rules.md examples 4.3-a and 4.3-b. Seats: D (dealer,
// 4 cards), A (6), B (4), C (6). On this board A's 8♣ T♠ make a straight,
// B's kings beat C's queens.
var (
	dacoitHoles = []string{
		"8h 8d Th Td",       // D
		"As Ad 4h 5h 8c Ts", // A
		"Kh Kd 6c 6s",       // B
		"Qh Qd 4c 4d 5s 2h", // C
	}
	dacoitBoard  = "2c 7d 9h Jc 3s"
	dacoitStacks = []int64{1000, 320, 1000, 1000}
	dacoitPicks  = []string{"1 choose 6", "2 choose 4", "3 choose 6"}
	dacoitPre    = []string{"3 call", "0 raise 60", "1 call", "2 call", "3 call"}
)

func TestDacoitScenarios(t *testing.T) {
	scenarios := []scenario{
		{
			name:    "4.3-a: 6-card players pay double pre-flop",
			variant: Dacoit,
			stacks:  dacoitStacks,
			holes:   dacoitHoles,
			board:   dacoitBoard,
			actions: seq(dacoitPicks, dacoitPre),
			check: func(t *testing.T, h *Hand) {
				if h.Street != Flop || h.potChips() != 360 {
					t.Fatalf("street %v pot %d, want flop with 360", h.Street, h.potChips())
				}
				if got := stacksOf(h); !slices.Equal(got, []int64{940, 200, 940, 880}) {
					t.Fatalf("stacks %v", got)
				}
				for i, want := range []int64{1, 2, 1, 2} {
					if h.Players[i].Mult != want || len(h.Players[i].Hole) != int(2+2*want) {
						t.Fatalf("player %d: mult %d, %d cards", i, h.Players[i].Mult, len(h.Players[i].Hole))
					}
				}
			},
		},
		{
			name:    "4.3-b: side pots in units, paid in chips, won with Omaha hands",
			variant: Dacoit,
			stacks:  dacoitStacks,
			holes:   dacoitHoles,
			board:   dacoitBoard,
			actions: seq(dacoitPicks, dacoitPre,
				[]string{"1 check", "2 bet 300", "3 call", "0 fold", "1 call"},
				checkDown(2, 2, 3)),
			wantFinal: []int64{940, 860, 1240, 280},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{860, []int{1, 2, 3}}, Pot{600, []int{2, 3}})
				for i, want := range map[int]string{1: "Straight, Jack high", 2: "Pair of Kings", 3: "Pair of Queens"} {
					if got := showDesc(h, i); got != want {
						t.Errorf("player %d shows %q, want %q", i, got, want)
					}
				}
			},
		},
		{
			name:    "heads-up: one choice, then the 6-card big blind posts double",
			variant: Dacoit,
			stacks:  []int64{1000, 1000},
			holes:   []string{"8h 8d Th Td", "As Ad 4h 5h 8c Ts"},
			board:   dacoitBoard,
			actions: []string{"1 choose 6"},
			check: func(t *testing.T, h *Hand) {
				if h.Picking || h.ToAct != 0 {
					t.Fatalf("picking %v, to act %d", h.Picking, h.ToAct)
				}
				if got := stacksOf(h); !slices.Equal(got, []int64{990, 960}) {
					t.Fatalf("blinds: stacks %v, want 990/960", got)
				}
			},
		},
		{name: "betting before choosing", variant: Dacoit, stacks: thousand3, holes: dacoitHoles[:3], board: dacoitBoard,
			actions: []string{"1 call"}, wantErr: ErrIllegalAction},
		{name: "choosing out of turn", variant: Dacoit, stacks: thousand3, holes: dacoitHoles[:3], board: dacoitBoard,
			actions: []string{"2 choose 4"}, wantErr: ErrNotYourTurn},
		{name: "the dealer never chooses", variant: Dacoit, stacks: thousand3, holes: dacoitHoles[:3], board: dacoitBoard,
			actions: []string{"1 choose 4", "2 choose 4", "0 choose 6"}, wantErr: ErrIllegalAction},
		{name: "five cards isn't a choice", variant: Dacoit, stacks: thousand3, holes: dacoitHoles[:3], board: dacoitBoard,
			actions: []string{"1 choose 5"}, wantErr: ErrIllegalAction},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.run(t) })
	}
}

func TestDacoitLegalAndNarration(t *testing.T) {
	h := scenario{variant: Dacoit, stacks: dacoitStacks, holes: dacoitHoles, board: dacoitBoard}.newHand(t)
	if !h.Picking || h.ToAct != 1 || h.PotChips() != 0 {
		t.Fatalf("picking %v to act %d pot %d", h.Picking, h.ToAct, h.PotChips())
	}
	if got := h.Legal(1); got != (Legal{Choose: true, ChooseMax: 6}) {
		t.Fatalf("legal = %+v", got)
	}
	if got := h.DefaultChoose(); got.Type != Choose || got.Count != 4 {
		t.Fatalf("default = %v", got)
	}
	for _, a := range dacoitPicks {
		p, act := parseAction(t, a)
		if err := h.Apply(p, act); err != nil {
			t.Fatal(err)
		}
	}
	text := strings.Join(h.NarrateFor(0), "\n")
	for _, want := range []string{"P1 takes 6 cards (pays 2x)", "P2 takes 4 cards", "P0 takes 4 cards", "P1 posts 20", "P2 posts 20"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

// docs/rules.md §4.3: "6" is refused once the deck can't cover everyone.
// Ten players need 45 cards with 4 each, so only three can take 6.
func TestDacoitDeckLimit(t *testing.T) {
	stacks := make([]int64, 10)
	for i := range stacks {
		stacks[i] = 1000
	}
	seats := make([]SeatSpec, 10)
	for i := range seats {
		seats[i] = SeatSpec{Stack: 1000}
	}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20}, Dacoit, seats, 0, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for p := 1; p <= 3; p++ {
		if err := h.Apply(p, Action{Type: Choose, Count: 6}); err != nil {
			t.Fatalf("player %d taking 6: %v", p, err)
		}
	}
	if got := h.Legal(4); got.ChooseMax != 4 {
		t.Fatalf("fourth 6-card player allowed: %+v", got)
	}
	if err := h.Apply(4, Action{Type: Choose, Count: 6}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("err = %v", err)
	}
	for p := 4; p <= 9; p++ {
		if err := h.Apply(p, Action{Type: Choose, Count: 4}); err != nil {
			t.Fatal(err)
		}
	}
	if h.Picking {
		t.Fatal("still picking")
	}
	if err := h.CheckInvariants(); err != nil {
		t.Fatal(err)
	}
}

// A player who can't afford one unit at double price can only take 4.
func TestDacoitShortStackCantTakeSix(t *testing.T) {
	h := scenario{variant: Dacoit, stacks: []int64{1000, 1, 1000}, holes: dacoitHoles[:3], board: dacoitBoard}.newHand(t)
	if got := h.Legal(1); got.ChooseMax != 4 {
		t.Fatalf("legal = %+v", got)
	}
	if err := h.Apply(1, Action{Type: Choose, Count: 6}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("err = %v", err)
	}
}
