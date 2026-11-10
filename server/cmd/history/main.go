// Command history turns a room's hand archive into readable JSON for
// analysis: every hand with every player's cards, every action, the boards
// and who won. It works on the archive file alone, so it still works after
// the room itself has closed.
//
//	fly ssh sftp get /data/history/EEEVKK.jsonl EEEVKK.jsonl --app poker-room-smruti
//	go run ./cmd/history EEEVKK.jsonl > EEEVKK.json
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"pokerroom/internal/history"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: history ROOM.jsonl > ROOM.json")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "history:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	records, err := history.ReadFile(path)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("%s has no hands", path)
	}
	hands, err := history.Replay(records)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Room  string         `json:"room"`
		Hands []history.Hand `json:"hands"`
	}{records[0].Room, hands})
}
