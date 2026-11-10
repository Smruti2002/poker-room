package room

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pokerroom/internal/history"
	"pokerroom/internal/protocol"
)

// Client is one connection's outbox. The WebSocket layer drains Send and
// stops when Done closes (the room dropped a client that fell behind).
type Client struct {
	send     chan []byte
	done     chan struct{}
	once     sync.Once
	playerID string
}

// NewClient returns a client with a small outbound buffer.
func NewClient() *Client {
	return &Client{send: make(chan []byte, 64), done: make(chan struct{})}
}

// Send yields encoded server messages, in order.
func (c *Client) Send() <-chan []byte { return c.send }

// Done closes when the room stops sending to this client.
func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) close() { c.once.Do(func() { close(c.done) }) }

// ErrRoomClosed is returned for a room that has been shut down.
var ErrRoomClosed = errors.New("this room has closed")

// Actor owns a Room on a single goroutine. Every change to the room runs on
// that goroutine, so the room needs no locks; after each change, every
// connected client gets a fresh snapshot.
type Actor struct {
	room     *Room
	inbox    chan func()
	quit     chan struct{}
	stopOnce sync.Once
	clients  map[*Client]bool
	mgr      *Manager // for opening a new room after a game; nil in some tests
	log      *slog.Logger
	now      func() time.Time
	lastSeen time.Time // last connect, disconnect or message, for idle cleanup
}

func newActor(r *Room, log *slog.Logger) *Actor {
	a := &Actor{
		room:    r,
		inbox:   make(chan func(), 256),
		quit:    make(chan struct{}),
		clients: map[*Client]bool{},
		log:     log.With("room", r.Code),
		now:     r.opts.Now,
	}
	a.lastSeen = a.now()
	go a.loop()
	return a
}

func (a *Actor) loop() {
	for {
		select {
		case f := <-a.inbox:
			a.run(f)
			a.broadcast()
		case <-a.quit:
			for c := range a.clients {
				c.close()
			}
			return
		}
	}
}

// stop shuts the room's goroutine down and disconnects everyone.
func (a *Actor) stop() { a.stopOnce.Do(func() { close(a.quit) }) }

// run executes f, containing any panic so one bad message can't take the
// whole server down.
func (a *Actor) run(f func()) {
	defer func() {
		if err := recover(); err != nil {
			a.log.Error("room panic", "err", err)
		}
	}()
	f()
}

// post queues f for the room goroutine; false if the room has closed.
func (a *Actor) post(f func()) bool {
	select {
	case a.inbox <- f:
		return true
	case <-a.quit:
		return false
	}
}

// call runs f on the room goroutine and waits for it. It reports false,
// without running f, if the room has closed. f either runs to completion
// before call returns, or never runs: a room closing mid-call can't leave
// f running behind the caller's back.
func (a *Actor) call(f func()) bool {
	const (
		pending = iota
		running
		abandoned
	)
	var state atomic.Int32
	done := make(chan struct{})
	if !a.post(func() {
		if !state.CompareAndSwap(pending, running) {
			return // the caller gave up; don't touch its variables
		}
		defer close(done)
		f()
	}) {
		return false
	}
	select {
	case <-done:
		return true
	case <-a.quit:
		if state.CompareAndSwap(pending, abandoned) {
			return false
		}
		<-done // already running: let it finish
		return true
	}
}

// Inspect runs f on the room goroutine; for tests and diagnostics.
func (a *Actor) Inspect(f func(*Room)) { a.call(func() { f(a.room) }) }

// Connect handles a connection's first message (join or resume) and
// registers the client. The welcome message is queued before any snapshot.
func (a *Actor) Connect(c *Client, m protocol.ClientMsg) error {
	err := ErrRoomClosed
	a.call(func() {
		var p *Player
		switch m.Type {
		case protocol.MsgJoin:
			p, err = a.room.Join(m.Name)
		case protocol.MsgResume:
			err = nil
			if p = a.room.PlayerByToken(m.Token); p == nil {
				err = errors.New("unknown session, please join again")
			} else if p.Kicked {
				err = ErrKicked
			}
		default:
			err = errors.New("first message must be join or resume")
		}
		if err != nil {
			return
		}
		c.playerID = p.ID
		p.Connections++
		a.clients[c] = true
		a.room.resolveKicks() // one more voter changes the majority
		a.lastSeen = a.now()
		a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgWelcome, Token: p.Token, PlayerID: p.ID})
	})
	return err
}

// Submit queues a message from a connected client.
func (a *Actor) Submit(c *Client, m protocol.ClientMsg) {
	a.post(func() {
		if !a.clients[c] {
			return
		}
		a.lastSeen = a.now()
		if m.Type == protocol.MsgRematch {
			if err := a.rematch(c.playerID); err != nil {
				a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgError, Error: err.Error()})
			}
			return
		}
		if m.Type == protocol.MsgHistory {
			// Only for the asker; nothing in the room changes.
			a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgHistoryReply, History: a.room.History(c.playerID)})
			return
		}
		if err := a.room.Handle(c.playerID, m); err != nil {
			a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgError, Error: err.Error()})
		}
	})
}

// Disconnect unregisters a client.
func (a *Actor) Disconnect(c *Client) {
	a.post(func() {
		a.drop(c)
		a.lastSeen = a.now()
	})
}

func (a *Actor) drop(c *Client) {
	if !a.clients[c] {
		return
	}
	delete(a.clients, c)
	if p := a.room.player(c.playerID); p != nil {
		p.Connections--
		a.room.resolveKicks() // one fewer voter changes the majority
	}
	c.close()
}

func (a *Actor) broadcast() {
	// Disconnect anyone voted out first. Dropping them can settle another
	// vote, so repeat until nobody is left to remove.
	for removed := true; removed; {
		removed = false
		for c := range a.clients {
			if p := a.room.player(c.playerID); p != nil && p.Kicked {
				a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgRemoved, Error: ErrKicked.Error()})
				a.drop(c)
				removed = true
			}
		}
	}
	for c := range a.clients {
		s := a.room.View(c.playerID)
		a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgState, State: &s})
	}
}

// sendTo queues a message; a client whose buffer is full is dropped rather
// than stalling the room for everyone.
func (a *Actor) sendTo(c *Client, m protocol.ServerMsg) {
	b, err := json.Marshal(m)
	if err != nil {
		a.log.Error("encode", "err", err)
		return
	}
	select {
	case c.send <- b:
	default:
		a.log.Warn("dropping slow client", "player", c.playerID)
		a.drop(c)
	}
}

// Summary is public information about a room, for the join page.
type Summary struct {
	Code    string `json:"code"`
	Phase   string `json:"phase"`
	Players int    `json:"players"`
	Max     int    `json:"maxPlayers"`
}

// Summary returns the room's public summary.
func (a *Actor) Summary() Summary {
	var s Summary
	a.call(func() {
		s = Summary{Code: a.room.Code, Phase: a.room.phase, Players: len(a.room.players), Max: a.room.settings.MaxPlayers}
	})
	return s
}

// Manager creates, finds, saves and cleans up rooms.
type Manager struct {
	mu    sync.Mutex
	rooms map[string]*Actor
	opts  Options
	log   *slog.Logger
	// MaxRooms bounds memory use; creating more fails.
	MaxRooms int
	// History, if set, is where every finished hand is archived (see
	// Options.Archive), for downloading whole games.
	History *history.Store
}

// ErrTooManyRooms is returned when the server is full.
var ErrTooManyRooms = errors.New("the server is full right now, please try again later")

// NewManager returns an empty registry; opts apply to every room.
func NewManager(opts Options, log *slog.Logger) *Manager {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Manager{rooms: map[string]*Actor{}, opts: opts, log: log, MaxRooms: 2000}
}

// KeepHistory archives every finished hand of every room in store, so
// whole games can be downloaded. Call it before creating or loading rooms.
func (m *Manager) KeepHistory(store *history.Store) {
	m.History = store
	m.opts.Archive = func(r history.Record) {
		if err := store.Append(r); err != nil {
			m.log.Error("archiving hand", "room", r.Room, "hand", r.Number, "err", err)
		}
	}
}

// codeAlphabet avoids look-alike characters (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// Create makes a room with hostName as its host and returns the room code
// and the host's session token.
func (m *Manager) Create(hostName string) (code, token string, err error) {
	if _, err := cleanName(hostName); err != nil {
		return "", "", err
	}
	code, tokens, err := m.create(nil, []string{hostName})
	if err != nil {
		return "", "", err
	}
	return code, tokens[0], nil
}

// create opens a room with the given settings (nil for the defaults) and
// seats names in order, the first as host. It returns each player's token.
func (m *Manager) create(settings *protocol.Settings, names []string) (code string, tokens []string, err error) {
	m.mu.Lock()
	if len(m.rooms) >= m.MaxRooms {
		m.mu.Unlock()
		return "", nil, ErrTooManyRooms
	}
	for code == "" || m.rooms[code] != nil {
		code = newCode()
	}
	r := New(code, m.opts)
	if settings != nil {
		r.settings = *settings
	}
	a := newActor(r, m.log)
	a.mgr = m
	m.rooms[code] = a
	m.mu.Unlock()

	a.call(func() {
		for _, name := range names {
			var p *Player
			if p, err = a.room.Join(name); err != nil {
				return
			}
			tokens = append(tokens, p.Token)
		}
	})
	if err != nil {
		m.mu.Lock()
		delete(m.rooms, code)
		m.mu.Unlock()
		a.stop()
		return "", nil, fmt.Errorf("creating room: %w", err)
	}
	m.log.Info("room created", "room", code)
	return code, tokens, nil
}

// Get finds a room by code, ignoring case.
func (m *Manager) Get(code string) *Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rooms[strings.ToUpper(code)]
}

func (m *Manager) actors() []*Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Actor, 0, len(m.rooms))
	for _, a := range m.rooms {
		out = append(out, a)
	}
	return out
}

// Sweep closes rooms nobody has been connected to for at least idle, and
// every room past its maximum age (Options.MaxAge) whether or not anyone is
// in it: hosts forget to end games, and an open tab would keep a room alive
// for ever. It returns how many it removed. It's housekeeping, not a game
// timer.
func (m *Manager) Sweep(idle time.Duration) int {
	now := m.opts.Now()
	removed := 0
	for _, a := range m.actors() {
		stale, old := false, false
		a.call(func() {
			closes := a.room.ClosesAt()
			old = !closes.IsZero() && !now.Before(closes)
			stale = old || (len(a.clients) == 0 && now.Sub(a.lastSeen) >= idle)
		})
		if !stale {
			continue
		}
		m.mu.Lock()
		delete(m.rooms, a.room.Code)
		m.mu.Unlock()
		a.stop()
		removed++
		if old {
			m.log.Info("room closed: reached its maximum age", "room", a.room.Code)
		} else {
			m.log.Info("room closed after being idle", "room", a.room.Code)
		}
	}
	return removed
}

// Stats counts rooms and connections, for /healthz.
type Stats struct {
	Rooms       int `json:"rooms"`
	Connections int `json:"connections"`
}

// Stats reports current load.
func (m *Manager) Stats() Stats {
	var s Stats
	for _, a := range m.actors() {
		n := 0
		if a.call(func() { n = len(a.clients) }) {
			s.Rooms++
			s.Connections += n
		}
	}
	return s
}

// SaveAll writes every room to path, atomically: a crash mid-write leaves
// the previous file intact.
func (m *Manager) SaveAll(path string) error {
	var snaps []Snapshot
	for _, a := range m.actors() {
		var s Snapshot
		if a.call(func() { s = a.room.Snapshot() }) {
			snaps = append(snaps, s)
		}
	}
	slices.SortFunc(snaps, func(x, y Snapshot) int { return strings.Compare(x.Code, y.Code) })
	b, err := json.Marshal(snaps)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rooms-*.json")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// LoadAll restores rooms saved by SaveAll. A missing file is not an error;
// a room that can't be restored is logged and skipped.
func (m *Manager) LoadAll(path string) (int, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var snaps []Snapshot
	if err := json.Unmarshal(b, &snaps); err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	loaded := 0
	for _, s := range snaps {
		r, err := Restore(s, m.opts)
		if err != nil {
			m.log.Error("skipping room", "room", s.Code, "err", err)
			continue
		}
		m.mu.Lock()
		if m.rooms[r.Code] == nil {
			a := newActor(r, m.log)
			a.mgr = m
			m.rooms[r.Code] = a
			loaded++
		}
		m.mu.Unlock()
	}
	return loaded, nil
}
