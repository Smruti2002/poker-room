package room

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

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

// Actor owns a Room on a single goroutine. Every change to the room runs on
// that goroutine, so the room needs no locks; after each change, every
// connected client gets a fresh snapshot.
type Actor struct {
	room    *Room
	inbox   chan func()
	clients map[*Client]bool
	log     *slog.Logger
}

type actorSched struct{ a *Actor }

func (s actorSched) After(d time.Duration, fire func()) {
	time.AfterFunc(d, func() { s.a.inbox <- fire })
}

func newActor(code string, opts Options, log *slog.Logger) *Actor {
	a := &Actor{inbox: make(chan func(), 256), clients: map[*Client]bool{}, log: log.With("room", code)}
	opts.Sched = actorSched{a}
	a.room = New(code, opts)
	go a.loop()
	return a
}

func (a *Actor) loop() {
	for f := range a.inbox {
		a.run(f)
		a.broadcast()
	}
}

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

// call runs f on the room goroutine and waits for it.
func (a *Actor) call(f func()) {
	done := make(chan struct{})
	a.inbox <- func() {
		defer close(done)
		f()
	}
	<-done
}

// Inspect runs f on the room goroutine; for tests and diagnostics.
func (a *Actor) Inspect(f func(*Room)) { a.call(func() { f(a.room) }) }

// Connect handles a connection's first message (join or resume) and
// registers the client. The welcome message is queued before any snapshot.
func (a *Actor) Connect(c *Client, m protocol.ClientMsg) error {
	var err error
	a.call(func() {
		var p *Player
		switch m.Type {
		case protocol.MsgJoin:
			p, err = a.room.Join(m.Name)
		case protocol.MsgResume:
			if p = a.room.PlayerByToken(m.Token); p == nil {
				err = errors.New("unknown session, please join again")
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
		a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgWelcome, Token: p.Token, PlayerID: p.ID})
	})
	return err
}

// Submit queues a message from a connected client.
func (a *Actor) Submit(c *Client, m protocol.ClientMsg) {
	a.inbox <- func() {
		if !a.clients[c] {
			return
		}
		if err := a.room.Handle(c.playerID, m); err != nil {
			a.sendTo(c, protocol.ServerMsg{Type: protocol.MsgError, Error: err.Error()})
		}
	}
}

// Disconnect unregisters a client.
func (a *Actor) Disconnect(c *Client) {
	a.inbox <- func() { a.drop(c) }
}

func (a *Actor) drop(c *Client) {
	if !a.clients[c] {
		return
	}
	delete(a.clients, c)
	if p := a.room.player(c.playerID); p != nil {
		p.Connections--
	}
	c.close()
}

func (a *Actor) broadcast() {
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

// Manager creates and finds rooms.
type Manager struct {
	mu    sync.Mutex
	rooms map[string]*Actor
	opts  Options
	log   *slog.Logger
}

// NewManager returns an empty registry; opts apply to every room.
func NewManager(opts Options, log *slog.Logger) *Manager {
	return &Manager{rooms: map[string]*Actor{}, opts: opts, log: log}
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
	m.mu.Lock()
	for code == "" || m.rooms[code] != nil {
		code = newCode()
	}
	a := newActor(code, m.opts, m.log)
	m.rooms[code] = a
	m.mu.Unlock()

	a.call(func() {
		var p *Player
		if p, err = a.room.Join(hostName); err == nil {
			token = p.Token
		}
	})
	if err != nil {
		return "", "", fmt.Errorf("creating room: %w", err)
	}
	m.log.Info("room created", "room", code)
	return code, token, nil
}

// Get finds a room by code, ignoring case.
func (m *Manager) Get(code string) *Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rooms[strings.ToUpper(code)]
}
