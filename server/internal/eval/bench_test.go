package eval

import (
	"testing"

	"pokerroom/internal/cards"
)

func benchDeals(b *testing.B, holeSize int) (holes, boards [][]cards.Card) {
	b.Helper()
	r := cards.SeedFromUint64(99).Rand()
	for range 1024 {
		d := cards.NewDeck()
		d.Shuffle(r)
		holes = append(holes, dealN(d, holeSize))
		boards = append(boards, dealN(d, 5))
	}
	return holes, boards
}

func BenchmarkEval5(b *testing.B) {
	_, boards := benchDeals(b, 0)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		Eval5([5]cards.Card(boards[i%len(boards)]))
	}
}

func BenchmarkHoldem(b *testing.B) { // 21 combinations
	holes, boards := benchDeals(b, 2)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, _ = Holdem(holes[i%1024], boards[i%1024])
	}
}

func BenchmarkOmaha(b *testing.B) { // 60 combinations
	holes, boards := benchDeals(b, 4)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, _ = Omaha(holes[i%1024], boards[i%1024])
	}
}

func BenchmarkDacoit(b *testing.B) { // 150 combinations; plan target < 5µs
	holes, boards := benchDeals(b, 6)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, _ = Omaha(holes[i%1024], boards[i%1024])
	}
}
