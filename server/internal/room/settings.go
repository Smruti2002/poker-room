package room

import (
	"fmt"
	"slices"

	"pokerroom/internal/protocol"
)

// DefaultSettings are the lobby defaults (docs/rules.md §2).
func DefaultSettings(variants []string) protocol.Settings {
	return protocol.Settings{
		StartingStack:  2000,
		LoanAmount:     2000,
		LoanPenalty:    1000,
		PenalizedLoans: 3,
		SmallBlind:     10,
		BigBlind:       20,
		TurnSeconds:    30,
		Variants:       slices.Clone(variants),
		MaxPlayers:     8,
		Hints:          true,
	}
}

// validateSettings checks the host's settings against the allowed ranges
// and the variants this server knows.
func validateSettings(s protocol.Settings, known []string) error {
	between := func(name string, v, lo, hi int64) error {
		if v < lo || v > hi {
			return fmt.Errorf("%s must be between %d and %d", name, lo, hi)
		}
		return nil
	}
	checks := []error{
		between("starting stack", s.StartingStack, 100, 1_000_000),
		between("loan amount", s.LoanAmount, 100, 1_000_000),
		between("loan penalty", s.LoanPenalty, 0, 1_000_000),
		between("penalized loans", int64(s.PenalizedLoans), 0, 100),
		between("small blind", s.SmallBlind, 1, s.StartingStack),
		between("big blind", s.BigBlind, s.SmallBlind, s.StartingStack),
		between("turn timer", int64(s.TurnSeconds), 10, 120),
		between("max players", int64(s.MaxPlayers), 2, 10),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	if len(s.Variants) == 0 {
		return fmt.Errorf("allow at least one variant")
	}
	for i, v := range s.Variants {
		if !slices.Contains(known, v) {
			return fmt.Errorf("unknown variant %q", v)
		}
		if slices.Contains(s.Variants[:i], v) {
			return fmt.Errorf("variant %q listed twice", v)
		}
	}
	return nil
}
