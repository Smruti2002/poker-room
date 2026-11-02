package cards

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
)

// ErrDeckEmpty is returned when dealing more cards than remain.
var ErrDeckEmpty = errors.New("not enough cards left in the deck")

// Seed fully determines a shuffle. Each hand gets a fresh Seed from
// NewSeed; logging it with the hand's actions makes the hand replayable.
type Seed [32]byte

// NewSeed returns a cryptographically random seed.
func NewSeed() Seed {
	var s Seed
	_, _ = crand.Read(s[:]) // crypto/rand.Read never fails on supported platforms
	return s
}

// SeedFromUint64 returns a deterministic seed for tests and simulations.
func SeedFromUint64(n uint64) Seed {
	var s Seed
	binary.LittleEndian.PutUint64(s[:], n)
	return s
}

// MarshalText encodes the seed as hex.
func (s Seed) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(s[:])), nil
}

// UnmarshalText decodes a hex seed.
func (s *Seed) UnmarshalText(b []byte) error {
	var out Seed
	if len(b) != hex.EncodedLen(len(out)) {
		return fmt.Errorf("invalid seed %q: want %d hex characters", b, hex.EncodedLen(len(out)))
	}
	if _, err := hex.Decode(out[:], b); err != nil {
		return fmt.Errorf("invalid seed %q: %w", b, err)
	}
	*s = out
	return nil
}

// Rand returns a ChaCha8-backed generator for the seed. ChaCha8 is a
// cryptographically strong PRNG, so seeded shuffles are unpredictable as long
// as the seed stays secret.
func (s Seed) Rand() *rand.Rand {
	return rand.New(rand.NewChaCha8(s))
}

// Deck is an ordered stack of cards dealt from the top.
type Deck struct {
	cards []Card
	next  int
}

// NewDeck returns a full deck in a fixed order.
func NewDeck() *Deck {
	d := &Deck{cards: make([]Card, NumCards)}
	for i := range d.cards {
		d.cards[i] = Card(i)
	}
	return d
}

// NewDeckOf returns a deck that deals exactly these cards in order.
func NewDeckOf(cs []Card) *Deck {
	return &Deck{cards: slices.Clone(cs)}
}

// NewShuffledDeck returns a full deck shuffled with the given seed.
func NewShuffledDeck(seed Seed) *Deck {
	d := NewDeck()
	d.Shuffle(seed.Rand())
	return d
}

// Shuffle shuffles the undealt cards with an unbiased Fisher-Yates shuffle.
func (d *Deck) Shuffle(r *rand.Rand) {
	rest := d.cards[d.next:]
	r.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
}

// Remaining returns the number of undealt cards.
func (d *Deck) Remaining() int { return len(d.cards) - d.next }

// Deal removes n cards from the top of the deck.
func (d *Deck) Deal(n int) ([]Card, error) {
	if n < 0 || n > d.Remaining() {
		return nil, ErrDeckEmpty
	}
	out := make([]Card, n)
	copy(out, d.cards[d.next:d.next+n])
	d.next += n
	return out, nil
}
