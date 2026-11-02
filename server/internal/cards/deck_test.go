package cards

import (
	"errors"
	"slices"
	"testing"
)

func TestNewDeckHasAll52Cards(t *testing.T) {
	d := NewDeck()
	all, err := d.Deal(NumCards)
	if err != nil {
		t.Fatal(err)
	}
	var seen Set
	for _, c := range all {
		if !c.Valid() || !seen.Add(c) {
			t.Fatalf("bad or duplicate card %v", c)
		}
	}
}

func TestDeal(t *testing.T) {
	d := NewShuffledDeck(SeedFromUint64(1))
	a, _ := d.Deal(5)
	b, _ := d.Deal(47)
	if d.Remaining() != 0 || len(a) != 5 || len(b) != 47 {
		t.Fatalf("remaining=%d", d.Remaining())
	}
	if _, err := d.Deal(1); !errors.Is(err, ErrDeckEmpty) {
		t.Fatalf("err = %v, want ErrDeckEmpty", err)
	}
	if _, err := NewDeck().Deal(-1); !errors.Is(err, ErrDeckEmpty) {
		t.Fatal("negative deal should fail")
	}
	// Dealt slices must not alias the deck.
	a[0] = 99
	if slices.Contains(b, 99) {
		t.Fatal("dealt slices alias")
	}
}

func TestSeededShuffleIsDeterministic(t *testing.T) {
	deal := func(seed Seed) []Card {
		c, _ := NewShuffledDeck(seed).Deal(NumCards)
		return c
	}
	a, b := deal(SeedFromUint64(42)), deal(SeedFromUint64(42))
	if !slices.Equal(a, b) {
		t.Fatal("same seed produced different decks")
	}
	if slices.Equal(a, deal(SeedFromUint64(43))) {
		t.Fatal("different seeds produced the same deck")
	}
	if slices.Equal(deal(NewSeed()), deal(NewSeed())) {
		t.Fatal("two random seeds produced the same deck")
	}
}

func TestShuffleOnlyTouchesUndealtCards(t *testing.T) {
	d := NewDeck()
	top, _ := d.Deal(3)
	d.Shuffle(SeedFromUint64(7).Rand())
	if !slices.Equal(top, []Card{0, 1, 2}) || d.Remaining() != 49 {
		t.Fatal("shuffle disturbed dealt cards")
	}
}

// TestShuffleIsRoughlyUniform checks that every card lands on top of the
// deck about equally often. With 52,000 shuffles each card expects 1000 hits;
// a chi-square statistic far above 51 degrees of freedom indicates bias.
func TestShuffleIsRoughlyUniform(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	const trials = 52_000
	var counts [NumCards]int
	r := SeedFromUint64(2026).Rand()
	for range trials {
		d := NewDeck()
		d.Shuffle(r)
		top, _ := d.Deal(1)
		counts[top[0]]++
	}
	expected := float64(trials) / NumCards
	chi2 := 0.0
	for _, c := range counts {
		diff := float64(c) - expected
		chi2 += diff * diff / expected
	}
	// p ≈ 0.0001 critical value for 51 degrees of freedom is about 96.
	if chi2 > 96 {
		t.Fatalf("chi-square %.1f suggests a biased shuffle", chi2)
	}
}

func TestNewDeckOf(t *testing.T) {
	in := MustParseList("As Kd 2c")
	d := NewDeckOf(in)
	in[0] = New(Two, Hearts) // the deck keeps its own copy
	got, _ := d.Deal(3)
	if !slices.Equal(got, MustParseList("As Kd 2c")) {
		t.Fatalf("dealt %v", got)
	}
}

func TestSeedText(t *testing.T) {
	s := SeedFromUint64(0xabcdef)
	b, err := s.MarshalText()
	if err != nil || len(b) != 64 {
		t.Fatalf("MarshalText = %s, %v", b, err)
	}
	var back Seed
	if err := back.UnmarshalText(b); err != nil || back != s {
		t.Fatalf("round trip = %x, %v", back, err)
	}
	for _, bad := range []string{"", "zz", "abcd", string(b) + "00"} {
		if err := back.UnmarshalText([]byte(bad)); err == nil {
			t.Errorf("UnmarshalText(%q) should fail", bad)
		}
	}
}
