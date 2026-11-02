package eval

import (
	"testing"

	"pokerroom/internal/cards"
)

func hand5(s string) [5]cards.Card {
	cs := cards.MustParseList(s)
	if len(cs) != 5 {
		panic("hand5 needs 5 cards: " + s)
	}
	return [5]cards.Card(cs)
}

func TestEval5Categories(t *testing.T) {
	tests := []struct {
		hand string
		cat  Category
		desc string
	}{
		{"As Ks Qs Js Ts", StraightFlush, "Royal flush"},
		{"9h 8h 7h 6h 5h", StraightFlush, "Straight flush, Nine high"},
		{"5d 4d 3d 2d Ad", StraightFlush, "Straight flush, Five high"}, // steel wheel
		{"Qc Qd Qh Qs 2c", FourOfAKind, "Four of a kind, Queens"},
		{"Kc Kd Kh Ac As", FullHouse, "Full house, Kings full of Aces"},
		{"Ah Jh 8h 4h 2h", Flush, "Flush, Ace high"},
		{"Tc 9d 8h 7s 6c", Straight, "Straight, Ten high"},
		{"Ac Kd Qh Js Tc", Straight, "Straight, Ace high"},
		{"5c 4d 3h 2s Ac", Straight, "Straight, Five high"}, // wheel
		{"7c 7d 7h Ks 2c", ThreeOfAKind, "Three of a kind, Sevens"},
		{"Jc Jd 4h 4s Ac", TwoPair, "Two pair, Jacks and Fours"},
		{"Tc Td 8h 4s 2c", OnePair, "Pair of Tens"},
		{"Ac Jd 8h 4s 2c", HighCard, "Ace high"},
		{"Kc Ad 2h 3s 4c", HighCard, "Ace high"}, // no wrap-around straight
	}
	for _, tt := range tests {
		t.Run(tt.hand, func(t *testing.T) {
			r := Eval5(hand5(tt.hand))
			if r.Category() != tt.cat {
				t.Fatalf("category = %v, want %v", r.Category(), tt.cat)
			}
			if r.String() != tt.desc {
				t.Fatalf("String() = %q, want %q", r.String(), tt.desc)
			}
		})
	}
}

func TestEval5Ordering(t *testing.T) {
	// Each hand must beat the next one.
	ladder := []string{
		"As Ks Qs Js Ts", // royal
		"6h 5h 4h 3h 2h", // 6-high straight flush
		"5d 4d 3d 2d Ad", // steel wheel
		"Ac Ad Ah As Kc", // quad aces
		"Ac Ad Ah As 2c", // quad aces, lower kicker
		"2c 2d 2h 2s Ac", // quad deuces
		"Ac Ad Ah 2s 2c", // aces full
		"Kc Kd Kh As Ac", // kings full of aces
		"Kc Kd Kh Qs Qc", // kings full of queens
		"Ah Kh Qh Jh 9h", // ace-high flush
		"Ah Kh Qh Jh 8h", // lower 5th card
		"Ac Kd Qh Js Tc", // broadway
		"6c 5d 4h 3s 2c", // six-high straight
		"5c 4d 3h 2s Ac", // wheel
		"Ac Ad Ah Ks Qc",
		"Ac Ad Ah Ks Jc",
		"Ac Ad Kh Ks Qc",
		"Ac Ad Kh Ks Jc",
		"Ac Ad Qh Qs Kc",
		"Ac Ad Kh Qs Jc",
		"Ac Ad Kh Qs Tc",
		"Kc Kd Ah Qs Jc",
		"Ac Kd Qh Js 9c",
		"Ac Kd Qh Js 8c",
		"7c 5d 4h 3s 2c",
	}
	for i := 0; i+1 < len(ladder); i++ {
		a, b := Eval5(hand5(ladder[i])), Eval5(hand5(ladder[i+1]))
		if a <= b {
			t.Errorf("%s (%v) should beat %s (%v)", ladder[i], a, ladder[i+1], b)
		}
	}
}

func TestEval5SuitsNeverBreakTies(t *testing.T) {
	a := Eval5(hand5("Ac Kc Qc Jc 9c"))
	b := Eval5(hand5("As Ks Qs Js 9s"))
	c := Eval5(hand5("Ad Kh Qs Jc 9c"))
	d := Eval5(hand5("Ah Kd Qc Js 9s"))
	if a != b || c != d {
		t.Fatal("suit changed the hand rank")
	}
}

func TestEval5OrderIndependent(t *testing.T) {
	h := hand5("Kc Kd 7h 7s 2c")
	want := Eval5(h)
	for _, p := range [][5]int{{4, 3, 2, 1, 0}, {2, 0, 4, 1, 3}, {1, 4, 0, 3, 2}} {
		var shuffled [5]cards.Card
		for i, j := range p {
			shuffled[i] = h[j]
		}
		if Eval5(shuffled) != want {
			t.Fatalf("order %v changed the rank", p)
		}
	}
}

func TestCategoryString(t *testing.T) {
	if StraightFlush.String() != "Straight flush" || Category(42).String() != "Category(42)" {
		t.Fatal("unexpected category names")
	}
	if rankName(0) != "?" {
		t.Fatal("rankName(0) should be ?")
	}
}

// TestEval5Exhaustive ranks all C(52,5) = 2,598,960 hands and checks the
// well-known category counts and the number of distinct hand values (7462).
// Every hand's category and tie-breaks are also checked against refEval.
func TestEval5Exhaustive(t *testing.T) {
	if testing.Short() {
		t.Skip("exhaustive check skipped in -short mode")
	}
	want := map[Category]int{
		StraightFlush: 40,
		FourOfAKind:   624,
		FullHouse:     3744,
		Flush:         5108,
		Straight:      10200,
		ThreeOfAKind:  54912,
		TwoPair:       123552,
		OnePair:       1098240,
		HighCard:      1302540,
	}
	got := map[Category]int{}
	distinct := map[HandRank]bool{}
	royals := 0
	var h [5]cards.Card
	for a := cards.Card(0); a < 52; a++ {
		h[0] = a
		for b := a + 1; b < 52; b++ {
			h[1] = b
			for c := b + 1; c < 52; c++ {
				h[2] = c
				for d := c + 1; d < 52; d++ {
					h[3] = d
					for e := d + 1; e < 52; e++ {
						h[4] = e
						r := Eval5(h)
						got[r.Category()]++
						distinct[r] = true
						if r.String() == "Royal flush" {
							royals++
						}
					}
				}
			}
		}
	}
	for cat, n := range want {
		if got[cat] != n {
			t.Errorf("%v: got %d hands, want %d", cat, got[cat], n)
		}
	}
	if len(distinct) != 7462 {
		t.Errorf("distinct hand values = %d, want 7462", len(distinct))
	}
	if royals != 4 {
		t.Errorf("royal flushes = %d, want 4", royals)
	}
}
