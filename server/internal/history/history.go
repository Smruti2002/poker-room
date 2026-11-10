// Package history keeps every finished hand of every room, so a whole game
// can be downloaded and analysed afterwards. Rooms themselves only keep the
// last few hands (for the in-game history panel); this archive keeps them
// all, one JSON line per hand in a file per room.
package history

import (
	"bufio"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"pokerroom/internal/engine"
)

// Player is a seat in an archived hand, in engine seat order.
type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Record is one finished hand: enough to replay it exactly.
type Record struct {
	Room     string     `json:"room"`
	Number   int        `json:"number"`
	Variant  string     `json:"variant"`
	Players  []Player   `json:"players"`
	Log      engine.Log `json:"log"`
	Finished int64      `json:"finished"` // unix ms
}

// Store appends records to <dir>/<ROOM>.jsonl.
type Store struct {
	dir string
	mu  sync.Mutex // rooms run on their own goroutines
}

// NewStore keeps archives in dir, creating it if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

var codeRE = regexp.MustCompile(`^[A-Z0-9]{1,16}$`)

func (s *Store) path(room string) (string, error) {
	if !codeRE.MatchString(room) {
		return "", fmt.Errorf("bad room code %q", room)
	}
	return filepath.Join(s.dir, room+".jsonl"), nil
}

// Append adds one finished hand to its room's archive.
func (s *Store) Append(r Record) error {
	p, err := s.path(r.Room)
	if err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// Load returns a room's archived hands in order; none if it has no archive.
func (s *Store) Load(room string) ([]Record, error) {
	p, err := s.path(room)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return ReadFile(p)
}

// ReadFile reads an archive file. If a hand number appears more than once
// (a crash between archiving a hand and saving the room can make it be
// played out again), the last one wins.
func ReadFile(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	byNumber := map[int]Record{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for line := 1; sc.Scan(); line++ {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		byNumber[r.Number] = r
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(byNumber))
	for _, r := range byNumber {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Record) int { return cmp.Compare(a.Number, b.Number) })
	return out, nil
}

// Prune deletes archives not written to for keep, and returns how many.
func (s *Store) Prune(keep time.Duration, now time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := filepath.Glob(filepath.Join(s.dir, "*.jsonl"))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range files {
		info, err := os.Stat(p)
		if err != nil || now.Sub(info.ModTime()) < keep {
			continue
		}
		if os.Remove(p) == nil {
			n++
		}
	}
	return n, nil
}
