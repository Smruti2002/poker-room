// Command replay re-runs a hand from its JSON log and prints what happened.
//
//	go run ./cmd/replay sim-failure.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"pokerroom/internal/engine"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: replay <hand-log.json>")
		os.Exit(2)
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var l engine.Log
	if err := json.Unmarshal(b, &l); err != nil {
		fmt.Fprintln(os.Stderr, "parsing log:", err)
		os.Exit(1)
	}

	h, replayErr := engine.Replay(l)
	if h != nil {
		fmt.Println(strings.Join(h.Narrate(), "\n"))
		fmt.Println()
		for i, p := range h.Players {
			fmt.Printf("%-10s stack %d\n", nameOf(p, i), p.Stack)
		}
		if err := h.CheckInvariants(); err != nil {
			fmt.Printf("\nINVARIANTS BROKEN:\n%v\n", err)
		}
	}
	if replayErr != nil {
		fmt.Fprintln(os.Stderr, "\nreplay stopped:", replayErr)
		os.Exit(1)
	}
}

func nameOf(p *engine.Player, i int) string {
	if p.ID != "" {
		return p.ID
	}
	return fmt.Sprintf("P%d", i)
}
