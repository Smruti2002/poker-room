package eval

import (
	"errors"
	"slices"
	"testing"

	"pokerroom/internal/cards"
)

var mp = cards.MustParseList

// sameCards reports whether two hands use the same cards in any order.
func sameCards(got [5]cards.Card, want []cards.Card) bool {
	g := slices.Clone(got[:])
	w := slices.Clone(want)
	slices.Sort(g)
	slices.Sort(w)
	return slices.Equal(g, w)
}

// The worked examples from docs/rules.md, section 4.
func TestRulesDocExamples(t *testing.T) {
	tests := []struct {
		name      string
		eval      func() (Hand, error)
		wantDesc  string
		wantCards string
	}{
		{
			name:      "4.1-a holdem royal flush",
			eval:      func() (Hand, error) { return Holdem(mp("K♠ Q♠"), mp("A♠ J♠ T♠ 2♦ 3♣")) },
			wantDesc:  "Royal flush",
			wantCards: "A♠ K♠ Q♠ J♠ T♠",
		},
		{
			name:      "4.1-b holdem playing the board",
			eval:      func() (Hand, error) { return Holdem(mp("2♣ 3♦"), mp("A♠ K♠ Q♠ J♠ T♠")) },
			wantDesc:  "Royal flush",
			wantCards: "A♠ K♠ Q♠ J♠ T♠",
		},
		{
			name:      "4.2-a omaha flush trap",
			eval:      func() (Hand, error) { return Omaha(mp("A♥ K♥ Q♥ J♥"), mp("9♥ 4♣ 7♦ 2♠ K♣")) },
			wantDesc:  "Pair of Kings",
			wantCards: "K♥ K♣ A♥ 9♥ 7♦",
		},
		{
			name:      "4.2-b omaha board full house trap",
			eval:      func() (Hand, error) { return Omaha(mp("A♠ 2♣ 3♦ 4♥"), mp("K♣ K♦ K♥ Q♠ Q♦")) },
			wantDesc:  "Three of a kind, Kings",
			wantCards: "K♣ K♦ K♥ A♠ 4♥",
		},
		{
			name: "4.4-c split 1, board 1 gets aces",
			eval: func() (Hand, error) { return Holdem(mp("A♠ A♥"), mp("9♣ T♣ 2♦ 3♥ K♠")) },
			// A A K T 9
			wantDesc:  "Pair of Aces",
			wantCards: "A♠ A♥ K♠ T♣ 9♣",
		},
		{
			name:      "4.4-c split 1, board 2 gets 7-8",
			eval:      func() (Hand, error) { return Holdem(mp("7♣ 8♣"), mp("A♦ 4♠ 4♥ J♣ Q♦")) },
			wantDesc:  "Pair of Fours",
			wantCards: "4♠ 4♥ A♦ Q♦ J♣",
		},
		{
			name:      "4.4-c split 2, board 1 gets 7-8",
			eval:      func() (Hand, error) { return Holdem(mp("7♣ 8♣"), mp("9♣ T♣ 2♦ 3♥ K♠")) },
			wantDesc:  "King high",
			wantCards: "K♠ T♣ 9♣ 8♣ 7♣",
		},
		{
			name:      "4.4-c split 2, board 2 gets aces",
			eval:      func() (Hand, error) { return Holdem(mp("A♠ A♥"), mp("A♦ 4♠ 4♥ J♣ Q♦")) },
			wantDesc:  "Full house, Aces full of Fours",
			wantCards: "A♠ A♥ A♦ 4♠ 4♥",
		},
		{
			name:      "4.6-a show two play two",
			eval:      func() (Hand, error) { return ShowTwo(mp("A♠ A♦"), mp("K♣ 5♥"), mp("A♣ K♦ K♥ 9♠ 2♣")) },
			wantDesc:  "Full house, Kings full of Aces",
			wantCards: "K♣ K♦ K♥ A♠ A♣",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := tt.eval()
			if err != nil {
				t.Fatal(err)
			}
			if h.Rank.String() != tt.wantDesc {
				t.Errorf("rank = %q, want %q", h.Rank, tt.wantDesc)
			}
			if !sameCards(h.Cards, mp(tt.wantCards)) {
				t.Errorf("cards = %v, want %s", h.Cards, tt.wantCards)
			}
		})
	}
}

// In 4.1-b everyone plays the board, so different hole cards tie.
func TestPlayingTheBoardTies(t *testing.T) {
	board := mp("A♠ K♠ Q♠ J♠ T♠")
	a, _ := Holdem(mp("2♣ 3♦"), board)
	b, _ := Holdem(mp("9♥ 9♦"), board)
	if a.Rank != b.Rank {
		t.Fatalf("%v vs %v should tie", a.Rank, b.Rank)
	}
}

func TestHoldemCanUseZeroOneOrTwoHoleCards(t *testing.T) {
	// One hole card: the A♦ completes a broadway straight.
	h, _ := Holdem(mp("A♦ 2♣"), mp("K♠ Q♥ J♣ T♦ 5♠"))
	if h.Rank.String() != "Straight, Ace high" {
		t.Fatalf("got %v", h.Rank)
	}
}

func TestBestOfErrors(t *testing.T) {
	tests := []struct {
		name   string
		groups []Group
	}{
		{"too few cards in group", []Group{{Cards: mp("As Ks"), Take: 3}, {Cards: mp("2c 3c"), Take: 2}}},
		{"negative take", []Group{{Cards: mp("As Ks Qs Js Ts"), Take: -1}}},
		{"takes 4 not 5", []Group{{Cards: mp("As Ks Qs Js"), Take: 4}}},
		{"takes 6 not 5", []Group{{Cards: mp("As Ks Qs Js Ts 9s"), Take: 6}}},
		{"invalid card", []Group{{Cards: []cards.Card{1, 2, 3, 4, 77}, Take: 5}}},
	}
	for _, tt := range tests {
		if _, err := BestOf(tt.groups...); err == nil {
			t.Errorf("%s: expected error", tt.name)
		}
	}

	_, err := Omaha(mp("As Ks Qs Js"), mp("As 2c 3c 4c 5c"))
	if !errors.Is(err, ErrDuplicateCard) {
		t.Fatalf("err = %v, want ErrDuplicateCard", err)
	}
}

func TestBestOfZeroTakeGroups(t *testing.T) {
	// Groups that take nothing are allowed anywhere in the list.
	h, err := BestOf(
		Group{Cards: mp("2c 3c"), Take: 0},
		Group{Cards: mp("As Ks"), Take: 2},
		Group{Cards: nil, Take: 0},
		Group{Cards: mp("Qs Js Ts 4d"), Take: 3},
		Group{Cards: mp("7h"), Take: 0},
	)
	if err != nil {
		t.Fatal(err)
	}
	if h.Rank.String() != "Royal flush" {
		t.Fatalf("got %v", h.Rank)
	}
}
