package engine

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

// By default player i holds the i-th strongest pocket pair, so lower index
// means a stronger hand. The board pairs none of them and makes no straight
// or flush with any of them.
var (
	defaultHoles = []string{"Ah Ad", "Kh Kd", "Qh Qd", "Th Td", "8h 8d", "7h 7d", "5h 5d", "4h 4d"}
	defaultBoard = "3c 6d 9h Jc 2s"
)

// scenario is a scripted hand. Actions are "<player> <verb> [amount]".
type scenario struct {
	name      string
	stacks    []int64
	mults     []int64 // optional, default 1
	dealer    int
	sb, bb    int64 // default 10/20
	potLimit  bool
	holes     []string // optional overrides, by player index
	actions   []string
	wantErr   error   // the last action must fail with this
	wantFinal []int64 // final stacks; the hand must be done
	check     func(t *testing.T, h *Hand)
}

func (s scenario) deck() []cards.Card {
	var d []cards.Card
	for i := range s.stacks {
		hole := defaultHoles[i]
		if i < len(s.holes) && s.holes[i] != "" {
			hole = s.holes[i]
		}
		d = append(d, cards.MustParseList(hole)...)
	}
	return append(d, cards.MustParseList(defaultBoard)...)
}

func (s scenario) newHand(t *testing.T) *Hand {
	t.Helper()
	cfg := Config{SmallBlind: 10, BigBlind: 20}
	if s.sb != 0 {
		cfg.SmallBlind, cfg.BigBlind = s.sb, s.bb
	}
	if s.potLimit {
		cfg.Structure = PotLimit
	}
	seats := make([]SeatSpec, len(s.stacks))
	for i, st := range s.stacks {
		seats[i] = SeatSpec{ID: fmt.Sprintf("P%d", i), Stack: st}
		if i < len(s.mults) {
			seats[i].Mult = s.mults[i]
		}
	}
	h, err := NewHand(cfg, Holdem, seats, s.dealer, Options{Deck: s.deck()})
	if err != nil {
		t.Fatal(err)
	}
	mustInvariants(t, h)
	return h
}

func parseAction(t *testing.T, s string) (int, Action) {
	t.Helper()
	f := strings.Fields(s)
	player, err := strconv.Atoi(f[0])
	if err != nil {
		t.Fatalf("bad action %q", s)
	}
	var a Action
	if err := a.Type.UnmarshalText([]byte(f[1])); err != nil {
		t.Fatalf("bad action %q: %v", s, err)
	}
	if len(f) == 3 {
		a.To, _ = strconv.ParseInt(f[2], 10, 64)
	}
	return player, a
}

func mustInvariants(t *testing.T, h *Hand) {
	t.Helper()
	if err := h.CheckInvariants(); err != nil {
		t.Fatalf("invariants broken:\n%v\n--- hand ---\n%s", err, strings.Join(h.Narrate(), "\n"))
	}
}

func stacksOf(h *Hand) []int64 {
	out := make([]int64, len(h.Players))
	for i, p := range h.Players {
		out[i] = p.Stack
	}
	return out
}

func (s scenario) run(t *testing.T) *Hand {
	t.Helper()
	h := s.newHand(t)
	for n, step := range s.actions {
		player, a := parseAction(t, step)
		err := h.Apply(player, a)
		last := n == len(s.actions)-1
		if last && s.wantErr != nil {
			if !errors.Is(err, s.wantErr) {
				t.Fatalf("step %q: err = %v, want %v", step, err, s.wantErr)
			}
			return h
		}
		if err != nil {
			t.Fatalf("step %q: %v\n--- hand ---\n%s", step, err, strings.Join(h.Narrate(), "\n"))
		}
		mustInvariants(t, h)
	}
	if s.wantFinal != nil {
		if !h.Done {
			t.Fatalf("hand not done; %d to act\n%s", h.ToAct, strings.Join(h.Narrate(), "\n"))
		}
		if got := stacksOf(h); !slices.Equal(got, s.wantFinal) {
			t.Fatalf("final stacks = %v, want %v\n--- hand ---\n%s", got, s.wantFinal, strings.Join(h.Narrate(), "\n"))
		}
	}
	if s.check != nil {
		s.check(t, h)
	}
	return h
}

// checkDown is "check" for each listed player on each of the given streets.
func checkDown(streets int, players ...int) []string {
	var out []string
	for range streets {
		for _, p := range players {
			out = append(out, fmt.Sprintf("%d check", p))
		}
	}
	return out
}

func seq(parts ...[]string) []string { return slices.Concat(parts...) }

var thousand3 = []int64{1000, 1000, 1000}

func TestScenarios(t *testing.T) {
	scenarios := []scenario{
		// --- Basics: blinds, order, folds -------------------------------
		{
			name:      "everyone folds to the big blind",
			stacks:    thousand3,
			actions:   []string{"0 fold", "1 fold"},
			wantFinal: []int64{1000, 990, 1010},
		},
		{
			name:      "small blind completes, check down, best hand wins",
			stacks:    thousand3,
			actions:   seq([]string{"0 fold", "1 call", "2 check"}, checkDown(3, 1, 2)),
			wantFinal: []int64{1000, 1020, 980},
		},
		{
			name:      "raise takes the blinds and the uncalled part comes back",
			stacks:    thousand3,
			actions:   []string{"0 raise 60", "1 fold", "2 fold"},
			wantFinal: []int64{1030, 990, 980},
		},
		{
			name:      "heads-up: dealer posts small blind, acts first pre-flop and last after",
			stacks:    []int64{1000, 1000},
			actions:   seq([]string{"0 call", "1 check"}, checkDown(3, 1, 0)),
			wantFinal: []int64{1020, 980},
		},
		{
			name:      "heads-up: big blind raises its option",
			stacks:    []int64{1000, 1000},
			actions:   []string{"0 call", "1 raise 80", "0 call", "1 bet 100", "0 fold"},
			wantFinal: []int64{920, 1080},
		},
		{
			name:      "bet, raise, call on the flop then showdown",
			stacks:    thousand3,
			actions:   seq([]string{"0 call", "1 call", "2 check", "1 bet 40", "2 raise 120", "0 fold", "1 call"}, checkDown(2, 1, 2)),
			wantFinal: []int64{980, 1160, 860},
		},
		{
			name:      "minimum re-raise",
			stacks:    thousand3,
			actions:   []string{"0 raise 60", "1 raise 100", "2 fold", "0 fold"},
			wantFinal: []int64{940, 1080, 980},
		},

		// --- Illegal actions --------------------------------------------
		{name: "acting out of turn", stacks: thousand3, actions: []string{"1 call"}, wantErr: ErrNotYourTurn},
		{name: "checking when facing a bet", stacks: thousand3, actions: []string{"0 check"}, wantErr: ErrIllegalAction},
		{name: "folding when you can check", stacks: thousand3, actions: []string{"0 call", "1 call", "2 fold"}, wantErr: ErrIllegalAction},
		{name: "raise below the minimum", stacks: thousand3, actions: []string{"0 raise 39"}, wantErr: ErrIllegalAction},
		{name: "re-raise below the last raise size", stacks: thousand3, actions: []string{"0 raise 60", "1 raise 99"}, wantErr: ErrIllegalAction},
		{name: "bet below the big blind", stacks: thousand3, actions: []string{"0 call", "1 call", "2 check", "1 bet 10"}, wantErr: ErrIllegalAction},
		{name: "raise more than your stack", stacks: []int64{100, 1000, 1000}, actions: []string{"0 raise 101"}, wantErr: ErrIllegalAction},
		{name: "bet when there is already a bet", stacks: thousand3, actions: []string{"0 bet 40"}, wantErr: ErrIllegalAction},
		{name: "raise when nobody has bet", stacks: thousand3, actions: []string{"0 call", "1 call", "2 check", "1 raise 40"}, wantErr: ErrIllegalAction},
		{name: "acting after the hand is over", stacks: thousand3, actions: []string{"0 fold", "1 fold", "2 check"}, wantErr: ErrHandOver},

		// --- All-ins and side pots --------------------------------------
		{
			name:      "short all-in call wins the main pot, side pot to the next best",
			stacks:    []int64{1000, 1000, 300},
			holes:     []string{"Kh Kd", "Qh Qd", "Ah Ad"},
			actions:   seq([]string{"0 raise 500", "1 call", "2 call"}, checkDown(3, 1, 0)),
			wantFinal: []int64{900, 500, 900},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{900, []int{0, 1, 2}}, Pot{400, []int{0, 1}})
			},
		},
		{
			name:      "all-in and call runs out the board",
			stacks:    []int64{500, 500},
			actions:   []string{"0 raise 500", "1 call"},
			wantFinal: []int64{1000, 0},
			check: func(t *testing.T, h *Hand) {
				if len(h.Boards[0]) != 5 {
					t.Fatalf("board has %d cards", len(h.Boards[0]))
				}
			},
		},
		{
			name:      "uncalled part of an all-in shove is returned",
			stacks:    []int64{1000, 300},
			actions:   []string{"0 raise 1000", "1 call"},
			wantFinal: []int64{1300, 0},
		},
		{
			name:      "three-way all-in: short stack wins main, middle wins side",
			stacks:    []int64{100, 300, 1000},
			actions:   []string{"0 raise 100", "1 raise 300", "2 call"},
			wantFinal: []int64{300, 400, 700},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{300, []int{0, 1, 2}}, Pot{400, []int{1, 2}})
			},
		},
		{
			name:      "three-way all-in: big stack wins every pot",
			stacks:    []int64{100, 300, 1000},
			holes:     []string{"Th Td", "Kh Kd", "Ah Ad"},
			actions:   []string{"0 raise 100", "1 raise 300", "2 call"},
			wantFinal: []int64{0, 0, 1400},
		},
		{
			name:      "short big blind: others still call the full big blind",
			stacks:    []int64{1000, 1000, 5},
			actions:   seq([]string{"0 call", "1 call"}, checkDown(3, 1, 0)),
			wantFinal: []int64{1025, 980, 0},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{15, []int{0, 1, 2}}, Pot{30, []int{0, 1}})
			},
		},
		{
			name:      "short small blind wins the main pot",
			stacks:    []int64{1000, 7, 1000},
			holes:     []string{"Kh Kd", "Ah Ad", "Qh Qd"},
			actions:   seq([]string{"0 call", "2 check"}, checkDown(3, 2, 0)),
			wantFinal: []int64{1006, 21, 980},
		},
		{
			name:      "heads-up: big blind all-in from the blind, dealer just calls",
			stacks:    []int64{1000, 15},
			actions:   []string{"0 call"},
			wantFinal: []int64{1015, 0},
		},
		{
			name:    "no raising when everyone else is all-in",
			stacks:  []int64{1000, 15},
			actions: []string{"0 raise 60"},
			wantErr: ErrIllegalAction,
		},

		// --- Re-opening rules -------------------------------------------
		{
			name:    "short all-in raise doesn't reopen betting for players who acted",
			stacks:  []int64{1000, 1000, 130},
			actions: []string{"0 raise 100", "1 call", "2 raise 130", "0 raise 300"},
			wantErr: ErrIllegalAction,
		},
		{
			name:    "short all-in raise: players who already acted may still call",
			stacks:  []int64{1000, 1000, 130},
			actions: seq([]string{"0 raise 100", "1 call", "2 raise 130", "0 call", "1 call"}, checkDown(3, 1, 0)),
			// P0 AA wins the main pot (390) outright.
			wantFinal: []int64{1260, 870, 0},
		},
		{
			name:   "short all-in raise: players yet to act may raise",
			stacks: []int64{130, 1000, 1000, 1000},
			actions: []string{
				"3 raise 100", "0 raise 130", "1 raise 300", "2 fold", "3 fold",
			},
			wantFinal: []int64{380, 870, 980, 900},
		},
		{
			name:   "short all-ins that add up to a full raise reopen betting",
			stacks: []int64{1000, 150, 200, 1000},
			actions: []string{
				"3 raise 100", "0 call", "1 raise 150", "2 raise 200", "3 raise 400", "0 fold",
			},
			wantFinal: []int64{900, 550, 100, 800},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{550, []int{1, 2, 3}}, Pot{100, []int{2, 3}})
			},
		},

		// --- Split pots and odd chips -----------------------------------
		{
			name:      "identical hands split the pot",
			stacks:    []int64{1000, 1000},
			holes:     []string{"Ac Kc", "Ah Kh"},
			actions:   seq([]string{"0 call", "1 check"}, checkDown(3, 1, 0)),
			wantFinal: []int64{1000, 1000},
		},
		{
			name:      "odd chip goes to the first winner left of the dealer",
			stacks:    thousand3,
			sb:        5,
			bb:        10,
			holes:     []string{"Ac Kc", "8h 7d", "Ah Kh"},
			actions:   seq([]string{"0 call", "1 fold", "2 check"}, checkDown(3, 2, 0)),
			wantFinal: []int64{1002, 995, 1003},
		},
		{
			name:      "odd chip skips folded players and losers; dealer is last in line",
			stacks:    []int64{1000, 1000, 1000, 1000},
			sb:        5,
			bb:        10,
			holes:     []string{"Ac Kc", "8h 7d", "Th 8s", "Ah Kh"},
			actions:   seq([]string{"3 call", "0 call", "1 fold", "2 check"}, checkDown(3, 2, 3, 0)),
			wantFinal: []int64{1007, 995, 990, 1008},
		},

		// --- Pot limit ---------------------------------------------------
		{name: "pot limit: pre-flop raise capped at pot", stacks: thousand3, potLimit: true, actions: []string{"0 raise 71"}, wantErr: ErrIllegalAction},
		{name: "pot limit: re-raise capped at pot", stacks: thousand3, potLimit: true, actions: []string{"0 raise 70", "1 raise 231"}, wantErr: ErrIllegalAction},
		{
			name:      "pot limit: pot-sized raises are allowed",
			stacks:    thousand3,
			potLimit:  true,
			actions:   []string{"0 raise 70", "1 raise 230", "2 raise 760", "0 fold", "1 fold"},
			wantFinal: []int64{930, 770, 1300},
		},

		// --- Multipliers (Dacoit betting, docs/rules.md 4.3-a / 4.3-b) ---
		{
			name:    "dacoit 4.3-a: pre-flop with 6-card players paying double",
			stacks:  []int64{1000, 320, 1000, 1000},
			mults:   []int64{1, 2, 1, 2},
			actions: []string{"3 call", "0 raise 60", "1 call", "2 call", "3 call"},
			check: func(t *testing.T, h *Hand) {
				if h.Street != Flop || h.potChips() != 360 {
					t.Fatalf("street %v pot %d, want flop with 360", h.Street, h.potChips())
				}
				if got := stacksOf(h); !slices.Equal(got, []int64{940, 200, 940, 880}) {
					t.Fatalf("stacks %v", got)
				}
			},
		},
		{
			name:   "dacoit 4.3-b: side pots in units, paid in chips",
			stacks: []int64{1000, 320, 1000, 1000},
			mults:  []int64{1, 2, 1, 2},
			actions: seq(
				[]string{"3 call", "0 raise 60", "1 call", "2 call", "3 call"},
				[]string{"1 check", "2 bet 300", "3 call", "0 fold", "1 call"},
				checkDown(2, 2, 3),
			),
			wantFinal: []int64{940, 860, 1240, 280},
			check: func(t *testing.T, h *Hand) {
				wantPots(t, h, Pot{860, []int{1, 2, 3}}, Pot{600, []int{2, 3}})
			},
		},
		{
			name:      "double-pay player with an odd stack is all-in with one chip left",
			stacks:    []int64{41, 1000},
			mults:     []int64{2, 1},
			actions:   []string{"0 call"},
			wantFinal: []int64{61, 980},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.run(t) })
	}
}

func wantPots(t *testing.T, h *Hand, want ...Pot) {
	t.Helper()
	if len(h.Pots) != len(want) {
		t.Fatalf("pots = %+v, want %+v", h.Pots, want)
	}
	for i := range want {
		if h.Pots[i].Chips != want[i].Chips || !slices.Equal(h.Pots[i].Eligible, want[i].Eligible) {
			t.Fatalf("pots = %+v, want %+v", h.Pots, want)
		}
	}
}
