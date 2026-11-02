package engine

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"pokerroom/internal/cards"
)

func TestBuildPots(t *testing.T) {
	type pl struct {
		total, mult int64
		folded      bool
	}
	tests := []struct {
		name    string
		players []pl
		want    []Pot
	}{
		{
			name:    "everyone equal: one pot",
			players: []pl{{100, 1, false}, {100, 1, false}, {100, 1, false}},
			want:    []Pot{{300, []int{0, 1, 2}}},
		},
		{
			name:    "folded money stays in the pot but can't win",
			players: []pl{{50, 1, true}, {100, 1, false}, {100, 1, false}},
			want:    []Pot{{250, []int{1, 2}}},
		},
		{
			name:    "one short all-in",
			players: []pl{{40, 1, false}, {100, 1, false}, {100, 1, false}},
			want:    []Pot{{120, []int{0, 1, 2}}, {120, []int{1, 2}}},
		},
		{
			name:    "two all-ins at different levels",
			players: []pl{{40, 1, false}, {70, 1, false}, {100, 1, false}, {100, 1, false}},
			want:    []Pot{{160, []int{0, 1, 2, 3}}, {90, []int{1, 2, 3}}, {60, []int{2, 3}}},
		},
		{
			name: "rules doc 4.3-b with multipliers",
			// D 60u x1 folded, A 160u x2, B 360u x1, C 360u x2
			players: []pl{{60, 1, true}, {160, 2, false}, {360, 1, false}, {360, 2, false}},
			want:    []Pot{{860, []int{1, 2, 3}}, {600, []int{2, 3}}},
		},
		{
			name:    "folded player over the top live level goes to the last pot",
			players: []pl{{300, 1, true}, {100, 1, false}, {200, 1, false}},
			want:    []Pot{{300, []int{1, 2}}, {300, []int{2}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ps []*Player
			var in int64
			for _, p := range tt.players {
				ps = append(ps, &Player{Total: p.total, Mult: p.mult, Folded: p.folded})
				in += p.total * p.mult
			}
			got := buildPots(ps)
			var out int64
			for _, p := range got {
				out += p.Chips
			}
			if out != in {
				t.Fatalf("pots hold %d chips, players put in %d", out, in)
			}
			if !slices.EqualFunc(got, tt.want, func(a, b Pot) bool {
				return a.Chips == b.Chips && slices.Equal(a.Eligible, b.Eligible)
			}) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSplitPot(t *testing.T) {
	tests := []struct {
		chips   int64
		winners []int
		dealer  int
		want    map[int]int64
	}{
		{100, []int{2}, 0, map[int]int64{2: 100}},
		{101, []int{0, 2}, 0, map[int]int64{0: 50, 2: 51}},               // 2 is closer to dealer's left
		{101, []int{0, 2}, 1, map[int]int64{0: 50, 2: 51}},               // order from seat 2: 2, 3, 0
		{102, []int{0, 1, 3}, 3, map[int]int64{0: 34, 1: 34, 3: 34}},     // even
		{5, []int{0, 1, 2, 3}, 1, map[int]int64{2: 2, 3: 1, 0: 1, 1: 1}}, // 1 odd chip to seat 2
	}
	for _, tt := range tests {
		got := splitPot(tt.chips, tt.winners, tt.dealer, 4)
		if len(got) != len(tt.want) {
			t.Fatalf("splitPot(%d, %v, d=%d) = %v, want %v", tt.chips, tt.winners, tt.dealer, got, tt.want)
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Fatalf("splitPot(%d, %v, d=%d) = %v, want %v", tt.chips, tt.winners, tt.dealer, got, tt.want)
			}
		}
	}
}

func TestNewHandValidation(t *testing.T) {
	ok := Config{SmallBlind: 10, BigBlind: 20}
	two := []SeatSpec{{Stack: 100}, {Stack: 100}}
	tests := []struct {
		name   string
		cfg    Config
		seats  []SeatSpec
		dealer int
		opts   Options
	}{
		{"one player", ok, []SeatSpec{{Stack: 100}}, 0, Options{}},
		{"eleven players", ok, make([]SeatSpec, 11), 0, Options{}},
		{"zero small blind", Config{SmallBlind: 0, BigBlind: 20}, two, 0, Options{}},
		{"big blind below small", Config{SmallBlind: 20, BigBlind: 10}, two, 0, Options{}},
		{"unknown structure", Config{SmallBlind: 10, BigBlind: 20, Structure: 9}, two, 0, Options{}},
		{"dealer out of range", ok, two, 2, Options{}},
		{"empty stack", ok, []SeatSpec{{Stack: 100}, {Stack: 0}}, 0, Options{}},
		{"multiplier 3", ok, []SeatSpec{{Stack: 100}, {Stack: 100, Mult: 3}}, 0, Options{}},
		{"double-pay player with 1 chip", ok, []SeatSpec{{Stack: 100}, {Stack: 1, Mult: 2}}, 0, Options{}},
		{"deck too short", ok, two, 0, Options{Deck: cards.MustParseList("As Ks Qs Js Ts 9s 8s 7s")}},
		{"duplicate card in deck", ok, two, 0, Options{Deck: cards.MustParseList("As As Qs Js Ts 9s 8s 7s 6s")}},
	}
	for _, tt := range tests {
		if _, err := NewHand(tt.cfg, Holdem, tt.seats, tt.dealer, tt.opts); err == nil {
			t.Errorf("%s: expected an error", tt.name)
		}
	}
}

func TestLegal(t *testing.T) {
	s := scenario{stacks: []int64{1000, 1000, 1000}}
	h := s.newHand(t)

	if got := h.Legal(1); got != (Legal{}) {
		t.Fatalf("not-to-act player has legal actions: %+v", got)
	}
	got := h.Legal(0)
	want := Legal{Fold: true, Call: true, CallUnits: 20, Raise: true, MinTo: 40, MaxTo: 1000}
	if got != want {
		t.Fatalf("Legal(0) = %+v, want %+v", got, want)
	}

	// Big blind's option after the small blind completes.
	for _, a := range []string{"0 call", "1 call"} {
		p, act := parseAction(t, a)
		if err := h.Apply(p, act); err != nil {
			t.Fatal(err)
		}
	}
	want = Legal{Check: true, Raise: true, MinTo: 40, MaxTo: 1000}
	if got := h.Legal(2); got != want {
		t.Fatalf("BB option = %+v, want %+v", got, want)
	}
}

func TestLegalAllInForLess(t *testing.T) {
	s := scenario{stacks: []int64{1000, 1000, 1000, 30}}
	h := s.run(t)
	// Dealer 0, SB 1, BB 2; player 3 acts first and can only shove 30,
	// short of the minimum raise to 40.
	got := h.Legal(3)
	want := Legal{Fold: true, Call: true, CallUnits: 20, Raise: true, MinTo: 30, MaxTo: 30}
	if got != want {
		t.Fatalf("Legal(3) = %+v, want %+v", got, want)
	}
	// Facing a bet bigger than the stack: call is all-in for less, no raise.
	h = scenario{stacks: []int64{1000, 1000, 1000, 30}, actions: []string{"3 call", "0 raise 300", "1 fold", "2 fold"}}.run(t)
	got = h.Legal(3)
	want = Legal{Fold: true, Call: true, CallUnits: 10}
	if got != want {
		t.Fatalf("Legal(3) facing 300 = %+v, want %+v", got, want)
	}
}

func TestUnknownActionRejected(t *testing.T) {
	h := scenario{stacks: thousand3}.newHand(t)
	if err := h.Apply(0, Action{Type: 99}); !errors.Is(err, ErrIllegalAction) {
		t.Fatalf("err = %v", err)
	}
}

func TestActionJSON(t *testing.T) {
	b, err := json.Marshal(Action{Type: Raise, To: 60})
	if err != nil || string(b) != `{"type":"raise","to":60}` {
		t.Fatalf("json = %s, %v", b, err)
	}
	var a Action
	if err := json.Unmarshal([]byte(`{"type":"check"}`), &a); err != nil || a != (Action{Type: Check}) {
		t.Fatalf("got %+v, %v", a, err)
	}
	if err := json.Unmarshal([]byte(`{"type":"shove"}`), &a); err == nil {
		t.Fatal("expected error for unknown action")
	}
	if _, err := json.Marshal(Action{Type: 42}); err == nil {
		t.Fatal("expected error for invalid action type")
	}
	if ActionType(42).String() != "ActionType(42)" || Street(9).String() != "Street(9)" {
		t.Fatal("unexpected fallback names")
	}
	if (Action{Type: Bet, To: 40}).String() != "bet 40" || (Action{Type: Fold}).String() != "fold" {
		t.Fatal("unexpected action strings")
	}
}

// A hand replayed from its JSON log ends exactly the same way.
func TestLogReplay(t *testing.T) {
	seats := []SeatSpec{{ID: "ann", Stack: 1000}, {ID: "bob", Stack: 600, Mult: 2}, {ID: "cy", Stack: 800}}
	h, err := NewHand(Config{SmallBlind: 10, BigBlind: 20}, Holdem, seats, 1, Options{Seed: cards.SeedFromUint64(7)})
	if err != nil {
		t.Fatal(err)
	}
	// Dealer 1: SB 2, BB 0, first to act 1.
	for _, a := range []LoggedAction{
		{1, Action{Type: Raise, To: 60}}, {2, Action{Type: Call}}, {0, Action{Type: Call}},
		{2, Action{Type: Bet, To: 100}}, {0, Action{Type: Fold}}, {1, Action{Type: Call}},
		{2, Action{Type: Check}}, {1, Action{Type: Bet, To: 140}}, {2, Action{Type: Call}},
	} {
		if err := h.Apply(a.Player, a.Action); err != nil {
			t.Fatalf("%+v: %v\n%s", a, err, strings.Join(h.Narrate(), "\n"))
		}
	}
	if !h.Done {
		t.Fatal("hand should be done")
	}

	b, err := json.Marshal(h.Log())
	if err != nil {
		t.Fatal(err)
	}
	var l Log
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	r, err := Replay(l)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stacksOf(r), stacksOf(h)) || !slices.Equal(r.Narrate(), h.Narrate()) {
		t.Fatalf("replay differs:\n%s\n---\n%s", strings.Join(r.Narrate(), "\n"), strings.Join(h.Narrate(), "\n"))
	}

	// A log with a bad action reports which step failed.
	l.Actions = append(l.Actions, LoggedAction{0, Action{Type: Check}})
	if _, err := Replay(l); !errors.Is(err, ErrHandOver) {
		t.Fatalf("err = %v", err)
	}
	if _, err := Replay(Log{Variant: "nope"}); err == nil {
		t.Fatal("expected unknown variant error")
	}
	if _, err := Replay(Log{Variant: "holdem"}); err == nil {
		t.Fatal("expected setup error for empty log")
	}
}

func TestNarrateCoversEveryEvent(t *testing.T) {
	h := scenario{
		stacks:  []int64{1000, 300, 1000},
		holes:   []string{"Kh Kd", "Ah Ad", "Qh Qd"},
		actions: seq([]string{"0 raise 500", "1 call", "2 fold"}),
	}.run(t)
	text := strings.Join(h.Narrate(), "\n")
	for _, want := range []string{
		"P1 posts 10", "P0 is dealt K♥ K♦", "P0 raises to 500", "P1 calls (300 in)", "P2 folds",
		"200 uncalled returned to P0", "Flop: 3♣ 6♦ 9♥", "Turn: J♣", "River: 2♠",
		"P1 shows A♥ A♦ (Pair of Aces)", "P1 wins 620 from pot 1 with Pair of Aces",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("narration missing %q:\n%s", want, text)
		}
	}
}

func TestCheckInvariantsDetectsCorruption(t *testing.T) {
	h := scenario{stacks: thousand3}.newHand(t)
	h.Players[0].Stack += 5
	h.Players[1].Hole[0] = h.Players[2].Hole[0]
	h.ToAct = 7
	err := h.CheckInvariants()
	if err == nil {
		t.Fatal("expected invariant violations")
	}
	for _, want := range []string{"chips not conserved", "dealt twice", "ToAct"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

func TestVariantRegistry(t *testing.T) {
	v, err := VariantByName("holdem")
	if err != nil || v != Holdem {
		t.Fatalf("got %v, %v", v, err)
	}
	if _, err := VariantByName("nope"); err == nil {
		t.Fatal("expected error")
	}
}
