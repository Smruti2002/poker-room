// Command sim plays bot games to stress-test the engine.
//
//	go run ./cmd/sim -players 6 -hands 100000
//
// On an invariant failure it writes the hand log to sim-failure.json; replay
// it with: go run ./cmd/replay sim-failure.json
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"pokerroom/internal/engine"
	"pokerroom/internal/sim"
)

func main() {
	var (
		cfg      sim.Config
		potLimit bool
		verbose  bool
		out      string
	)
	flag.IntVar(&cfg.Players, "players", 6, "players at the table (2-10)")
	flag.IntVar(&cfg.Hands, "hands", 10_000, "hands to play")
	flag.Int64Var(&cfg.StartStack, "stack", 2000, "starting stack and rebuy amount")
	flag.Int64Var(&cfg.SmallBlind, "sb", 10, "small blind")
	flag.Int64Var(&cfg.BigBlind, "bb", 20, "big blind")
	flag.BoolVar(&potLimit, "pl", false, "pot-limit instead of no-limit")
	flag.Float64Var(&cfg.DoubleChance, "double", 0, "chance a player pays double per unit (Dacoit pot math)")
	flag.Uint64Var(&cfg.Seed, "seed", uint64(time.Now().UnixNano()), "random seed")
	flag.BoolVar(&verbose, "v", false, "print every hand")
	flag.StringVar(&out, "failure-log", "sim-failure.json", "where to write a failing hand's log")
	flag.Parse()

	if potLimit {
		cfg.Structure = engine.PotLimit
	}
	if verbose {
		n := 0
		cfg.OnHand = func(h *engine.Hand) {
			n++
			fmt.Printf("=== Hand %d ===\n%s\n\n", n, strings.Join(h.Narrate(), "\n"))
		}
	}

	start := time.Now()
	st, err := sim.Run(cfg)
	if f, ok := errors.AsType[*sim.Failure](err); ok {
		b, _ := json.MarshalIndent(f.Log, "", "  ")
		if werr := os.WriteFile(out, b, 0o644); werr != nil {
			fmt.Fprintln(os.Stderr, "writing failure log:", werr)
		}
		fmt.Fprintf(os.Stderr, "FAIL (seed %d): %v\nhand log written to %s\n", cfg.Seed, f, out)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("seed %d: %d hands, %d actions in %v (%.0f hands/s)\n",
		cfg.Seed, st.Hands, st.Actions, elapsed.Round(time.Millisecond), float64(st.Hands)/elapsed.Seconds())
	fmt.Printf("showdowns %d, fold wins %d, hands with side pots %d, split pots %d, rebuys %d\n",
		st.Showdowns, st.FoldWins, st.SidePots, st.SplitPots, st.Rebuys)
	fmt.Println("all invariants held")
}
