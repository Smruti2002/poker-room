package sim

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"pokerroom/internal/engine"
)

// simHands is the number of hands per configuration. CI runs the default;
// the nightly job sets SIM_HANDS much higher.
func simHands(t *testing.T) int {
	if s := os.Getenv("SIM_HANDS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			t.Fatalf("bad SIM_HANDS: %v", err)
		}
		return n
	}
	if testing.Short() {
		return 2_000
	}
	return 20_000
}

func TestSimulation(t *testing.T) {
	configs := []Config{
		{Players: 2, StartStack: 2000, SmallBlind: 10, BigBlind: 20},
		{Players: 6, StartStack: 2000, SmallBlind: 10, BigBlind: 20},
		{Players: 10, StartStack: 2000, SmallBlind: 10, BigBlind: 20},
		{Players: 6, StartStack: 2000, SmallBlind: 10, BigBlind: 20, Structure: engine.PotLimit},
		{Players: 6, StartStack: 2000, SmallBlind: 10, BigBlind: 20, DoubleChance: 0.4},
		{Players: 8, StartStack: 150, SmallBlind: 5, BigBlind: 10, DoubleChance: 0.3}, // short stacks: lots of all-ins
		{Players: 6, StartStack: 2000, SmallBlind: 10, BigBlind: 20, Variant: "omaha", Structure: engine.PotLimit},
		{Players: 10, StartStack: 500, SmallBlind: 5, BigBlind: 10, Variant: "omaha", DoubleChance: 0.3},
		{Players: 6, StartStack: 2000, SmallBlind: 10, BigBlind: 20, Variant: "show2"},
		{Players: 10, StartStack: 300, SmallBlind: 5, BigBlind: 10, Variant: "show2", Structure: engine.PotLimit},
		{Players: 4, StartStack: 2000, SmallBlind: 10, BigBlind: 20, Variant: "dacoit"},
		{Players: 10, StartStack: 300, SmallBlind: 5, BigBlind: 10, Variant: "dacoit", Structure: engine.PotLimit},
		{Players: 6, StartStack: 2000, SmallBlind: 10, BigBlind: 20, Variant: "double"},
		{Players: 10, StartStack: 300, SmallBlind: 5, BigBlind: 10, Variant: "double", DoubleChance: 0.3},
		{Players: 6, StartStack: 500, SmallBlind: 5, BigBlind: 10, Variant: "triple"},
	}
	hands := simHands(t)
	for i, cfg := range configs {
		cfg.Hands = hands
		if cfg.Variant == "triple" {
			// Working out default splits is slow; the nightly run does the volume.
			cfg.Hands = max(hands/10, 200)
		}
		cfg.Seed = uint64(i + 1)
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			st, err := Run(cfg)
			if f, ok := errors.AsType[*Failure](err); ok {
				t.Fatalf("%v\nreplay this log with cmd/replay: %+v", f, f.Log)
			}
			if err != nil {
				t.Fatal(err)
			}
			if st.Hands != cfg.Hands || st.Showdowns == 0 || st.FoldWins == 0 {
				t.Fatalf("suspicious stats: %+v", st)
			}
			// Total chips = starting stacks + rebuys.
			var total int64
			for _, s := range st.Stacks {
				total += s
			}
			if want := int64(cfg.Players+st.Rebuys) * cfg.StartStack; total != want {
				t.Fatalf("chips at the end %d, want %d", total, want)
			}
			t.Logf("%+v", st)
		})
	}
}

func TestSimulationIsDeterministic(t *testing.T) {
	cfg := Config{Players: 5, Hands: 200, StartStack: 1000, SmallBlind: 5, BigBlind: 10, DoubleChance: 0.2, Seed: 42}
	a, err := Run(cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Run(cfg)
	if a.Actions != b.Actions || a.Showdowns != b.Showdowns {
		t.Fatalf("runs differ: %+v vs %+v", a, b)
	}
}

func TestOnHandCallback(t *testing.T) {
	calls := 0
	_, err := Run(Config{Players: 3, Hands: 10, StartStack: 500, SmallBlind: 5, BigBlind: 10,
		OnHand: func(h *engine.Hand) {
			if !h.Done {
				t.Error("callback got an unfinished hand")
			}
			calls++
		}})
	if err != nil || calls != 10 {
		t.Fatalf("calls = %d, err = %v", calls, err)
	}
}

func TestUnknownVariant(t *testing.T) {
	if _, err := Run(Config{Players: 2, Hands: 1, StartStack: 100, SmallBlind: 1, BigBlind: 2, Variant: "go-fish"}); err == nil {
		t.Fatal("expected an error for an unknown variant")
	}
}
