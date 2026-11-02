// Package cards defines playing cards, parsing and formatting, and the deck.
package cards

import (
	"fmt"
	"strings"
)

// Rank is a card rank from Two (2) to Ace (14).
type Rank uint8

const (
	Two Rank = iota + 2
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
	Ten
	Jack
	Queen
	King
	Ace
)

// Suit is one of the four suits.
type Suit uint8

const (
	Clubs Suit = iota
	Diamonds
	Hearts
	Spades
)

// Card is one of the 52 cards, encoded as (rank-2)*4 + suit, i.e. 0..51.
type Card uint8

// NumCards is the size of a standard deck.
const NumCards = 52

// New returns the card with the given rank and suit.
func New(r Rank, s Suit) Card {
	return Card((uint8(r)-2)*4 + uint8(s))
}

// Rank returns the card's rank.
func (c Card) Rank() Rank { return Rank(uint8(c)/4 + 2) }

// Suit returns the card's suit.
func (c Card) Suit() Suit { return Suit(uint8(c) % 4) }

// Valid reports whether c is one of the 52 cards.
func (c Card) Valid() bool { return c < NumCards }

const (
	rankChars = "23456789TJQKA"
	suitChars = "cdhs"
)

var suitSymbols = [4]string{"♣", "♦", "♥", "♠"}

// String formats the card as two ASCII characters, e.g. "As", "Td".
func (c Card) String() string {
	if !c.Valid() {
		return "??"
	}
	return string([]byte{rankChars[c.Rank()-2], suitChars[c.Suit()]})
}

// Pretty formats the card with a suit symbol, e.g. "A♠", "T♦".
func (c Card) Pretty() string {
	if !c.Valid() {
		return "??"
	}
	return string(rankChars[c.Rank()-2]) + suitSymbols[c.Suit()]
}

// MarshalText encodes the card as its ASCII form, so cards appear as "As" in JSON.
func (c Card) MarshalText() ([]byte, error) {
	if !c.Valid() {
		return nil, fmt.Errorf("invalid card %d", uint8(c))
	}
	return []byte(c.String()), nil
}

// UnmarshalText parses the ASCII or symbol form of a card.
func (c *Card) UnmarshalText(b []byte) error {
	parsed, err := Parse(string(b))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// Parse parses a card such as "As", "td", "10h" or "K♠".
func Parse(s string) (Card, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "10") {
		s = "T" + s[2:]
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid card %q", s)
	}
	ri := strings.IndexByte(rankChars, upper(s[0]))
	if ri < 0 {
		return 0, fmt.Errorf("invalid rank in card %q", s)
	}
	suit, ok := parseSuit(s[1:])
	if !ok {
		return 0, fmt.Errorf("invalid suit in card %q", s)
	}
	return New(Rank(ri+2), suit), nil
}

func parseSuit(s string) (Suit, bool) {
	switch s {
	case "c", "C", "♣", "♧":
		return Clubs, true
	case "d", "D", "♦", "♢":
		return Diamonds, true
	case "h", "H", "♥", "♡":
		return Hearts, true
	case "s", "S", "♠", "♤":
		return Spades, true
	}
	return 0, false
}

func upper(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

// ParseList parses cards separated by spaces or commas, e.g. "As Kd, 10h".
func ParseList(s string) ([]Card, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
	out := make([]Card, 0, len(fields))
	for _, f := range fields {
		c, err := Parse(f)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// MustParseList is ParseList that panics on error. Intended for tests and fixtures.
func MustParseList(s string) []Card {
	cs, err := ParseList(s)
	if err != nil {
		panic(err)
	}
	return cs
}

// Set is a bitset of cards, used for duplicate detection.
type Set uint64

// Add adds c and reports whether it was newly added.
func (s *Set) Add(c Card) bool {
	bit := Set(1) << c
	if *s&bit != 0 {
		return false
	}
	*s |= bit
	return true
}

// Has reports whether c is in the set.
func (s Set) Has(c Card) bool { return s&(Set(1)<<c) != 0 }
