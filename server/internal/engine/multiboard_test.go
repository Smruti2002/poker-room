package engine

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

// Double Texas deals used below (docs/rules.md §4.4). With the 4.4-c
// boards, P0's aces win board 1 and P1's kings make two pair on board 2.
var (
	exBoard1   = "9c Tc 2d 3h Ks"
	exBoard2   = "Ad 4s 4h Jc Qd"
	tieBoard   = "2c 2d 2h 2s Ac" // everyone plays quad twos with the board ace
	exHoles    = []string{"As Ah 7c 8c", "Kd Kh 5c 6d"}
	headsUpRun = seq([]string{"0 call", "1 check"}, checkDown(3, 1, 0))
)

func TestMultiBoardScenarios(t *testing.T) {
	scenarios := []scenario{
		{
			// 4.4-a: A wins board 1, B wins board 2 -> 1 point each -> split.
			name:      "double 4.4-a: one board each splits the pot",
			variant:   DoubleTexas,
			stacks:    []int64{1000, 1000},
			holes:     exHoles,
			boards:    []string{exBoard1, exBoard2},
			actions:   seq(headsUpRun, []string{"0 allocate As Ah 7c 8c", "1 allocate 5c 6d Kd Kh"}),
			wantFinal: []int64{1000, 1000},
			check: func(t *testing.T, h *Hand) {
				if got := showDesc(h, 0); got != "Pair of Aces | Pair of Fours" {
					t.Errorf("P0 = %q", got)
				}
				if got := showDesc(h, 1); got != "King high | Two pair, Kings and Fours" {
					t.Errorf("P1 = %q", got)
				}
			},
		},
		{
			// 4.4-b: a tie on board 1 is a full point each; A also wins
			// board 2: 2 points to 1, so A takes the whole pot.
			name:      "double 4.4-b: a tie on one board plus a win on the other takes it all",
			variant:   DoubleTexas,
			stacks:    []int64{1000, 1000},
			holes:     exHoles,
			boards:    []string{tieBoard, "9d Td Jc Qs 3h"},
			actions:   seq(headsUpRun, []string{"0 allocate 7c 8c As Ah", "1 allocate Kd Kh 5c 6d"}),
			wantFinal: []int64{1020, 980},
		},
		{
			// Three players each win exactly one board with a set: 1-1-1.
			name:    "triple: each player wins one board, three-way split",
			variant: TripleTexas,
			stacks:  thousand3,
			holes:   []string{"9c 9d As Ac Jd Qs", "Tc Td Ah Ad 7h 8c", "Kc Kd 8d 7s 6c 3c"},
			boards:  []string{"9h 2c 5d Jc 3s", "Th 4c 6d Qc 8s", "Kh 7c 2d 4s 6h"},
			actions: seq(
				[]string{"0 call", "1 call", "2 check"}, checkDown(3, 1, 2, 0),
				[]string{"2 allocate 8d 7s 6c 3c Kc Kd", "0 allocate 9c 9d As Ac Jd Qs", "1 allocate Ah Ad Tc Td 7h 8c"},
			),
			wantFinal: []int64{1000, 1000, 1000},
			check: func(t *testing.T, h *Hand) {
				for i, want := range []string{"Three of a kind, Nines", "Three of a kind, Tens", "Three of a kind, Kings"} {
					if got := strings.Split(showDesc(h, i), " | ")[i]; got != want {
						t.Errorf("player %d board %d = %q, want %q", i, i+1, got, want)
					}
				}
			},
		},
		{
			// The short stack sweeps both boards (straight, quad fours) and
			// wins the main pot; the side pot is scored only between the
			// other two, who win a board each and split it.
			name:    "double: side pots are scored among their own players",
			variant: DoubleTexas,
			stacks:  []int64{1000, 1000, 100},
			holes:   []string{"As Ah 7c 8c", "Kd Kh 5c 6d", "Qc Jd 4c 4d"},
			boards:  []string{exBoard1, exBoard2},
			actions: seq(
				[]string{"0 raise 300", "1 call", "2 call"}, checkDown(3, 1, 0),
				[]string{"0 allocate As Ah 7c 8c", "1 allocate 5c 6d Kd Kh", "2 allocate Qc Jd 4c 4d"},
			),
			wantFinal: []int64{900, 900, 300},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{300, []int{0, 1, 2}}, Pot{400, []int{0, 1}})
			},
		},
		{
			name:      "double: a hand won by folds needs no split",
			variant:   DoubleTexas,
			stacks:    []int64{1000, 1000},
			holes:     exHoles,
			boards:    []string{exBoard1, exBoard2},
			actions:   []string{"0 raise 100", "1 fold"},
			wantFinal: []int64{1020, 980},
		},
		{
			name:    "double: after an all-in the boards run out, then players split",
			variant: DoubleTexas,
			stacks:  []int64{1000, 1000},
			holes:   exHoles,
			boards:  []string{exBoard1, exBoard2},
			actions: []string{"0 raise 1000", "1 call"},
			check: func(t *testing.T, h *Hand) {
				if !h.Allocating || h.Done || len(h.Boards[0]) != 5 || len(h.Boards[1]) != 5 {
					t.Fatalf("allocating %v done %v boards %v", h.Allocating, h.Done, h.Boards)
				}
			},
		},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.run(t) })
	}
}

func allocatingHand(t *testing.T) *Hand {
	t.Helper()
	return scenario{variant: DoubleTexas, stacks: []int64{1000, 1000, 1000},
		holes:   []string{"As Ah 7c 8c", "Kd Kh 5c 6d", "Qc Jd 4c 4d"},
		boards:  []string{exBoard1, exBoard2},
		actions: seq([]string{"0 fold", "1 call", "2 check"}, checkDown(3, 1, 2)),
	}.run(t)
}

func TestAllocationPhase(t *testing.T) {
	h := allocatingHand(t)
	if !h.Allocating || h.ToAct != -1 || !slices.Equal(h.Pending(), []int{1, 2}) {
		t.Fatalf("allocating %v toAct %d pending %v", h.Allocating, h.ToAct, h.Pending())
	}
	if h.Legal(1) != (Legal{Allocate: true}) || h.Legal(0) != (Legal{}) || h.Legal(9) != (Legal{}) {
		t.Fatal("only pending players may allocate")
	}

	bad := []struct {
		player int
		action Action
		want   error
	}{
		{1, Action{Type: Call}, ErrIllegalAction},
		{1, Action{Type: Allocate, Cards: cards.MustParseList("Kd Kh 5c")}, ErrIllegalAction},
		{1, Action{Type: Allocate, Cards: cards.MustParseList("Kd Kh 5c As")}, ErrIllegalAction},
		{1, Action{Type: Allocate, Cards: cards.MustParseList("Kd Kd 5c 6d")}, ErrIllegalAction},
		{0, Action{Type: Allocate, Cards: cards.MustParseList("As Ah 7c 8c")}, ErrNotYourTurn}, // folded
		{-1, Action{Type: Allocate}, ErrNotYourTurn},
	}
	for _, b := range bad {
		if err := h.Apply(b.player, b.action); !errors.Is(err, b.want) {
			t.Errorf("player %d %v: err %v, want %v", b.player, b.action, err, b.want)
		}
	}

	// A locked-in split stays hidden from others until the showdown.
	if err := h.Apply(1, Action{Type: Allocate, Cards: cards.MustParseList("5c 6d Kd Kh")}); err != nil {
		t.Fatal(err)
	}
	if err := h.Apply(1, Action{Type: Allocate, Cards: cards.MustParseList("5c 6d Kd Kh")}); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("second allocation: %v", err)
	}
	mustInvariants(t, h)
	text := strings.Join(h.NarrateFor(2), "\n")
	if !strings.Contains(text, "P1 locked in their split") || strings.Contains(text, "K♦") {
		t.Fatalf("P2's view leaks P1's split:\n%s", text)
	}

	if err := h.Apply(2, h.DefaultSplit(2)); err != nil {
		t.Fatal(err)
	}
	if !h.Done || h.Allocating {
		t.Fatal("hand should finish once everyone has split")
	}
	mustInvariants(t, h)
}

// docs/rules.md 4.4-c: aces on board 1 make a pair, 7-8 on board 2 play the
// board's fours; the other way round leaves king high on board 1. The
// default split maximises the weakest board, so it's aces on board 1.
func TestDefaultSplitMaximisesTheWeakestBoard(t *testing.T) {
	h := scenario{variant: DoubleTexas, stacks: []int64{1000, 1000}, holes: exHoles,
		boards: []string{exBoard1, exBoard2}, actions: headsUpRun}.run(t)
	if got := h.DefaultSplit(0).Cards; !slices.Equal(got, cards.MustParseList("As Ah 7c 8c")) {
		t.Fatalf("default split = %v", got)
	}
}

func TestTripleTexasNeedsSixOrFewer(t *testing.T) {
	seats := make([]SeatSpec, 7)
	for i := range seats {
		seats[i] = SeatSpec{Stack: 1000}
	}
	if _, err := NewHand(Config{SmallBlind: 10, BigBlind: 20}, TripleTexas, seats, 0, Options{}); err == nil {
		t.Fatal("7 players need 57 cards; Triple Texas must refuse")
	}
	if _, err := NewHand(Config{SmallBlind: 10, BigBlind: 20}, TripleTexas, seats[:6], 0, Options{}); err != nil {
		t.Fatalf("6 players should fit: %v", err)
	}
	if cardsNeeded(DoubleTexas, 10) > cards.NumCards {
		t.Fatal("Double Texas should fit 10 players")
	}
}

func TestMultiBoardRankNeedsSplit(t *testing.T) {
	if _, err := DoubleTexas.Rank(&Player{}, make([][]cards.Card, 2)); err == nil {
		t.Fatal("ranking without a split should fail")
	}
}

func TestHints(t *testing.T) {
	// Hold'em: nothing before the flop, a hand name after it, nothing once folded.
	h := scenario{stacks: thousand3}.newHand(t)
	if h.Hint(0) != "" {
		t.Fatal("pre-flop hint")
	}
	for _, a := range []string{"0 call", "1 call", "2 check"} {
		p, act := parseAction(t, a)
		if err := h.Apply(p, act); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.Hint(0); got != "Pair of Aces" {
		t.Fatalf("hint = %q", got)
	}

	// Show 2 Play 2 needs the reveal; multi-board hands need the split.
	s2 := scenario{variant: ShowTwo, stacks: []int64{1000, 1000}, holes: []string{"As Ad Kc 5h", "Qh Qd 9h 3c"}, board: "Ac Kd Kh 9s 2c"}.newHand(t)
	if s2.Hint(0) != "" {
		t.Fatal("hint before the reveal")
	}
	mb := allocatingHand(t)
	if mb.Hint(1) != "" {
		t.Fatal("multi-board hint before the split")
	}
	if err := mb.Apply(1, Action{Type: Allocate, Cards: cards.MustParseList("5c 6d Kd Kh")}); err != nil {
		t.Fatal(err)
	}
	if got := mb.Hint(1); got != "King high | Two pair, Kings and Fours" {
		t.Fatalf("multi-board hint = %q", got)
	}
	if mb.Hint(0) != "" {
		t.Fatal("folded players get no hint")
	}
}
