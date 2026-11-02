package eval

import (
	"testing"

	"pokerroom/internal/cards"
)

func iterations(t *testing.T, full, short int) int {
	if testing.Short() {
		return short
	}
	return full
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}

func dealN(d *cards.Deck, n int) []cards.Card {
	c, err := d.Deal(n)
	if err != nil {
		panic(err)
	}
	return c
}

// TestEval5MatchesReference compares Eval5 with the naive reference on random
// pairs of hands: categories must match and the two must order hands the same.
func TestEval5MatchesReference(t *testing.T) {
	n := iterations(t, 500_000, 20_000)
	r := cards.SeedFromUint64(1).Rand()
	for i := range n {
		d := cards.NewDeck()
		d.Shuffle(r)
		a, b := [5]cards.Card(dealN(d, 5)), [5]cards.Card(dealN(d, 5))
		ra, rb := Eval5(a), Eval5(b)
		ka, kb := refEval(a[:]), refEval(b[:])
		if ra.Category() != ka.cat {
			t.Fatalf("iter %d: %v category %v, reference %v", i, a, ra.Category(), ka.cat)
		}
		var cmp int
		switch {
		case ra > rb:
			cmp = 1
		case ra < rb:
			cmp = -1
		}
		if cmp != sign(refCompare(ka, kb)) {
			t.Fatalf("iter %d: %v (%v) vs %v (%v): eval says %d, reference %d",
				i, a, ra, b, rb, cmp, sign(refCompare(ka, kb)))
		}
	}
}

// TestHoldemMatchesBruteForce checks BestOf against brute force over all 21
// five-card subsets of a random 7-card Hold'em hand.
func TestHoldemMatchesBruteForce(t *testing.T) {
	n := iterations(t, 100_000, 5_000)
	r := cards.SeedFromUint64(2).Rand()
	for i := range n {
		d := cards.NewDeck()
		d.Shuffle(r)
		hole, board := dealN(d, 2), dealN(d, 5)
		got, err := Holdem(hole, board)
		if err != nil {
			t.Fatal(err)
		}
		checkHandConsistent(t, got)
		want := refBest(append(append([]cards.Card{}, hole...), board...))
		if refCompare(refEval(got.Cards[:]), want) != 0 {
			t.Fatalf("iter %d: hole %v board %v: got %v, reference best %+v", i, hole, board, got.Rank, want)
		}
	}
}

// TestOmahaMatchesBruteForce covers both Omaha (4 hole) and Dacoit (6 hole).
func TestOmahaMatchesBruteForce(t *testing.T) {
	n := iterations(t, 50_000, 3_000)
	r := cards.SeedFromUint64(3).Rand()
	for i := range n {
		holeSize := 4
		if i%2 == 1 {
			holeSize = 6
		}
		d := cards.NewDeck()
		d.Shuffle(r)
		hole, board := dealN(d, holeSize), dealN(d, 5)
		got, err := Omaha(hole, board)
		if err != nil {
			t.Fatal(err)
		}
		checkHandConsistent(t, got)
		checkUses(t, got, hole, 2)
		if refCompare(refEval(got.Cards[:]), refOmaha(hole, board)) != 0 {
			t.Fatalf("iter %d: hole %v board %v: got %v", i, hole, board, got.Rank)
		}
	}
}

func TestShowTwoMatchesBruteForce(t *testing.T) {
	n := iterations(t, 50_000, 3_000)
	r := cards.SeedFromUint64(4).Rand()
	for i := range n {
		d := cards.NewDeck()
		d.Shuffle(r)
		shown, hidden, board := dealN(d, 2), dealN(d, 2), dealN(d, 5)
		got, err := ShowTwo(shown, hidden, board)
		if err != nil {
			t.Fatal(err)
		}
		checkHandConsistent(t, got)
		checkUses(t, got, shown, 1)
		checkUses(t, got, hidden, 1)

		// Brute force: one shown x one hidden x three board.
		var best refKey
		first := true
		for _, s := range shown {
			for _, h := range hidden {
				k := refOmaha([]cards.Card{s, h}, board)
				if first || refCompare(k, best) > 0 {
					best, first = k, false
				}
			}
		}
		if refCompare(refEval(got.Cards[:]), best) != 0 {
			t.Fatalf("iter %d: shown %v hidden %v board %v: got %v", i, shown, hidden, board, got.Rank)
		}
	}
}

// checkHandConsistent verifies the returned cards really make the returned rank.
func checkHandConsistent(t *testing.T, h Hand) {
	t.Helper()
	if Eval5(h.Cards) != h.Rank {
		t.Fatalf("hand cards %v rank %v, but reported %v", h.Cards, Eval5(h.Cards), h.Rank)
	}
}

// checkUses verifies exactly n of the hand's cards come from pool.
func checkUses(t *testing.T, h Hand, pool []cards.Card, n int) {
	t.Helper()
	var set cards.Set
	for _, c := range pool {
		set.Add(c)
	}
	used := 0
	for _, c := range h.Cards {
		if set.Has(c) {
			used++
		}
	}
	if used != n {
		t.Fatalf("hand %v uses %d cards from %v, want %d", h.Cards, used, pool, n)
	}
}
