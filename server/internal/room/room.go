// Package room runs a poker room: the lobby, seats and settings, dealer
// rotation and variant choice, turn timers, loans and the leaderboard.
//
// Room is a synchronous state machine with no goroutines or I/O, so it can
// be tested directly. Actor (actor.go) owns a Room on a single goroutine and
// connects it to WebSocket clients.
package room

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

// Scheduler runs fire after d. Inside an Actor, fire runs on the room's
// goroutine; tests use a fake that fires on demand.
type Scheduler interface {
	After(d time.Duration, fire func())
}

// Options configure a room. Zero values get sensible defaults.
type Options struct {
	Variants    map[string]engine.Variant // default: every engine variant
	Now         func() time.Time
	Sched       Scheduler
	Rand        *mrand.Rand       // picks the first dealer
	NewSeed     func() cards.Seed // shuffle seed per hand
	ChooseDelay time.Duration     // dealer's time to pick a variant
	ResultDelay time.Duration     // pause after a hand won by folds
	// ShowdownDelay is the longer pause after a showdown, so everyone can
	// see who won and with what.
	ShowdownDelay time.Duration
}

// Player is a person in the room, kept across hands.
type Player struct {
	ID          string
	Name        string
	Token       string
	Stack       int64 // chips, updated when each hand ends
	Debt        int64
	Loans       int
	SittingOut  bool
	Connections int
}

// Room is the state of one poker room.
type Room struct {
	Code string

	opts         Options
	variantNames []string
	settings     protocol.Settings
	players      []*Player
	hostID       string
	phase        string
	stage        string
	nextID       int

	hand     *engine.Hand // current or last hand
	handIDs  []string     // engine player index -> player ID
	handNo   int
	variant  string
	dealerID string
	options  []string // variants the dealer may choose from

	deadline     time.Time
	timerGen     int
	endAfterHand bool
}

// User-facing errors.
var (
	ErrNotHost     = errors.New("only the host can do that")
	ErrRoomFull    = errors.New("the room is full")
	ErrGameOver    = errors.New("the game has ended")
	ErrUnknownMsg  = errors.New("unknown message type")
	ErrNotYourTurn = errors.New("it's not your turn")
)

// potLimitVariants use pot-limit when the host enables it (docs/rules.md §2).
var potLimitVariants = []string{"omaha", "dacoit", "show2"}

// New creates a room in the lobby.
func New(code string, opts Options) *Room {
	if opts.Variants == nil {
		opts.Variants = engine.AllVariants()
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Rand == nil {
		opts.Rand = mrand.New(mrand.NewPCG(mrand.Uint64(), mrand.Uint64()))
	}
	if opts.NewSeed == nil {
		opts.NewSeed = cards.NewSeed
	}
	if opts.ChooseDelay == 0 {
		opts.ChooseDelay = 30 * time.Second
	}
	if opts.ResultDelay == 0 {
		opts.ResultDelay = 4 * time.Second
	}
	if opts.ShowdownDelay == 0 {
		opts.ShowdownDelay = 9 * time.Second
	}
	r := &Room{Code: code, opts: opts, phase: protocol.PhaseLobby}
	for name := range opts.Variants {
		r.variantNames = append(r.variantNames, name)
	}
	slices.Sort(r.variantNames)
	r.settings = DefaultSettings(r.variantNames)
	return r
}

// Join adds a new player. The first player becomes the host.
func (r *Room) Join(name string) (*Player, error) {
	name, err := cleanName(name)
	if err != nil {
		return nil, err
	}
	if r.phase == protocol.PhaseEnded {
		return nil, ErrGameOver
	}
	if len(r.players) >= r.settings.MaxPlayers {
		return nil, ErrRoomFull
	}
	for _, p := range r.players {
		if strings.EqualFold(p.Name, name) {
			return nil, fmt.Errorf("the name %q is taken", name)
		}
	}
	r.nextID++
	p := &Player{
		ID:    fmt.Sprintf("p%d", r.nextID),
		Name:  name,
		Token: newToken(),
		Stack: r.settings.StartingStack,
	}
	r.players = append(r.players, p)
	if r.hostID == "" {
		r.hostID = p.ID
	}
	if r.phase == protocol.PhasePlaying && r.stage == protocol.StageWaiting {
		r.nextHand()
	}
	return p, nil
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n == 0 || n > 20 {
		return "", errors.New("name must be 1-20 characters")
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return "", errors.New("name contains invalid characters")
	}
	return name, nil
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// PlayerByToken finds a player for a reconnecting client.
func (r *Room) PlayerByToken(token string) *Player {
	for _, p := range r.players {
		if token != "" && p.Token == token {
			return p
		}
	}
	return nil
}

func (r *Room) player(id string) *Player {
	for _, p := range r.players {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// Handle applies a message from player pid. Join and resume are handled by
// the caller before a player exists.
func (r *Room) Handle(pid string, m protocol.ClientMsg) error {
	p := r.player(pid)
	if p == nil {
		return errors.New("unknown player")
	}
	switch m.Type {
	case protocol.MsgUpdateSettings:
		return r.updateSettings(p, m.Settings)
	case protocol.MsgStart:
		return r.start(p)
	case protocol.MsgChooseVariant:
		return r.chooseVariant(p, m.Variant)
	case protocol.MsgAction:
		return r.act(p, m.Action)
	case protocol.MsgTakeLoan:
		return r.takeLoan(p)
	case protocol.MsgSitOut:
		p.SittingOut = true
		return nil
	case protocol.MsgSitIn:
		p.SittingOut = false
		if r.phase == protocol.PhasePlaying && r.stage == protocol.StageWaiting {
			r.nextHand()
		}
		return nil
	case protocol.MsgEndGame:
		return r.endGame(p)
	}
	return ErrUnknownMsg
}

func (r *Room) updateSettings(p *Player, s *protocol.Settings) error {
	switch {
	case p.ID != r.hostID:
		return ErrNotHost
	case r.phase != protocol.PhaseLobby:
		return errors.New("settings are locked once the game starts")
	case s == nil:
		return errors.New("missing settings")
	}
	if err := validateSettings(*s, r.variantNames); err != nil {
		return err
	}
	if s.MaxPlayers < len(r.players) {
		return fmt.Errorf("%d players have already joined", len(r.players))
	}
	r.settings = *s
	r.settings.Variants = slices.Clone(s.Variants)
	for _, q := range r.players {
		q.Stack = s.StartingStack
	}
	return nil
}

func (r *Room) start(p *Player) error {
	switch {
	case p.ID != r.hostID:
		return ErrNotHost
	case r.phase != protocol.PhaseLobby:
		return errors.New("the game has already started")
	case len(r.players) < 2:
		return errors.New("need at least 2 players to start")
	}
	r.phase = protocol.PhasePlaying
	r.nextHand()
	return nil
}

func (r *Room) endGame(p *Player) error {
	if p.ID != r.hostID {
		return ErrNotHost
	}
	if r.phase == protocol.PhaseEnded {
		return ErrGameOver
	}
	// Never cut a hand short: chips are in the pot.
	if r.stage == protocol.StageHand {
		r.endAfterHand = true
		return nil
	}
	r.end()
	return nil
}

func (r *Room) end() {
	r.phase = protocol.PhaseEnded
	r.stage = ""
	r.cancelTimer()
}

// nextHand picks the next dealer and either asks them for a variant or deals
// straight away when only one variant is possible.
func (r *Room) nextHand() {
	if r.endAfterHand {
		r.end()
		return
	}
	r.cancelTimer()
	eligible := r.eligible()
	if len(eligible) < 2 {
		r.stage = protocol.StageWaiting
		return
	}

	r.dealerID = r.nextDealer(eligible)
	r.options = nil
	for _, name := range r.settings.Variants {
		v, ok := r.opts.Variants[name]
		if ok && len(eligible)*v.HoleCards()+5*v.Boards() <= cards.NumCards {
			r.options = append(r.options, name)
		}
	}
	switch len(r.options) {
	case 0:
		r.stage = protocol.StageWaiting
	case 1:
		r.deal(r.options[0])
	default:
		r.stage = protocol.StageChoosing
		r.schedule(r.opts.ChooseDelay)
	}
}

// eligible lists, in seat order, players who will be dealt in.
func (r *Room) eligible() []*Player {
	var out []*Player
	for _, p := range r.players {
		if !p.SittingOut && p.Stack > 0 {
			out = append(out, p)
		}
	}
	return out
}

// nextDealer moves the button clockwise to the next eligible player.
func (r *Room) nextDealer(eligible []*Player) string {
	prev := slices.IndexFunc(r.players, func(p *Player) bool { return p.ID == r.dealerID })
	if prev < 0 {
		return eligible[r.opts.Rand.IntN(len(eligible))].ID
	}
	for i := 1; i <= len(r.players); i++ {
		p := r.players[(prev+i)%len(r.players)]
		if slices.Contains(eligible, p) {
			return p.ID
		}
	}
	return eligible[0].ID // unreachable: eligible is non-empty
}

func (r *Room) chooseVariant(p *Player, name string) error {
	switch {
	case r.stage != protocol.StageChoosing:
		return errors.New("not choosing a variant now")
	case p.ID != r.dealerID:
		return errors.New("only the dealer picks the variant")
	case !slices.Contains(r.options, name):
		return fmt.Errorf("%q can't be played now", name)
	}
	r.deal(name)
	return nil
}

func (r *Room) deal(variant string) {
	eligible := r.eligible()
	// Players may have sat out (or in) while the dealer was choosing. If the
	// table no longer fits this hand, or the dealer left, start over.
	v := r.opts.Variants[variant]
	stillDealer := slices.ContainsFunc(eligible, func(p *Player) bool { return p.ID == r.dealerID })
	if len(eligible) < 2 || len(eligible)*v.HoleCards()+5*v.Boards() > cards.NumCards || !stillDealer {
		r.nextHand()
		return
	}
	seats := make([]engine.SeatSpec, len(eligible))
	r.handIDs = make([]string, len(eligible))
	dealer := 0
	for i, p := range eligible {
		seats[i] = engine.SeatSpec{ID: p.Name, Stack: p.Stack}
		r.handIDs[i] = p.ID
		if p.ID == r.dealerID {
			dealer = i
		}
	}
	cfg := engine.Config{SmallBlind: r.settings.SmallBlind, BigBlind: r.settings.BigBlind}
	if r.settings.PotLimit && slices.Contains(potLimitVariants, variant) {
		cfg.Structure = engine.PotLimit
	}
	h, err := engine.NewHand(cfg, v, seats, dealer, engine.Options{Seed: r.opts.NewSeed()})
	if err != nil {
		// Settings and eligibility are validated, so this is a bug.
		panic(fmt.Sprintf("room %s: dealing %s: %v", r.Code, variant, err))
	}
	r.hand = h
	r.handNo++
	r.variant = variant
	r.stage = protocol.StageHand
	r.afterHandChange()
}

// afterHandChange settles a finished hand or starts the next turn timer.
func (r *Room) afterHandChange() {
	if !r.hand.Done {
		r.schedule(time.Duration(r.settings.TurnSeconds) * time.Second)
		return
	}
	for i, id := range r.handIDs {
		r.player(id).Stack = r.hand.Players[i].Stack
	}
	r.stage = protocol.StageResult
	delay := r.opts.ResultDelay
	if slices.ContainsFunc(r.hand.Events, func(e engine.Event) bool { return e.Kind == engine.EvShow }) {
		delay = r.opts.ShowdownDelay
	}
	r.schedule(delay)
}

func (r *Room) act(p *Player, a *protocol.Action) error {
	if r.stage != protocol.StageHand || r.hand.Done {
		return errors.New("no hand in progress")
	}
	if a == nil {
		return errors.New("missing action")
	}
	idx := slices.Index(r.handIDs, p.ID)
	if idx < 0 || idx != r.hand.ToAct {
		return ErrNotYourTurn
	}
	var typ engine.ActionType
	if err := typ.UnmarshalText([]byte(a.Type)); err != nil {
		return err
	}
	ea := engine.Action{Type: typ, To: a.To, Count: a.Count}
	for _, s := range a.Cards {
		c, err := cards.Parse(s)
		if err != nil {
			return err
		}
		ea.Cards = append(ea.Cards, c)
	}
	if err := r.hand.Apply(idx, ea); err != nil {
		return err
	}
	r.afterHandChange()
	return nil
}

func (r *Room) takeLoan(p *Player) error {
	switch {
	case r.phase == protocol.PhaseEnded:
		return ErrGameOver
	case p.Stack > 0:
		return errors.New("you can only borrow when you're out of chips")
	}
	p.Loans++
	p.Stack += r.settings.LoanAmount
	p.Debt += r.settings.LoanAmount
	if p.Loans <= r.settings.PenalizedLoans {
		p.Debt += r.settings.LoanPenalty
	}
	if r.phase == protocol.PhasePlaying && r.stage == protocol.StageWaiting {
		r.nextHand()
	}
	return nil
}

// Profit is the player's score: stack − starting stack − debt.
func (r *Room) Profit(p *Player) int64 {
	return p.Stack - r.settings.StartingStack - p.Debt
}

func (r *Room) schedule(d time.Duration) {
	r.timerGen++
	gen := r.timerGen
	r.deadline = r.opts.Now().Add(d)
	if r.opts.Sched != nil {
		r.opts.Sched.After(d, func() { r.onTimer(gen) })
	}
}

func (r *Room) cancelTimer() {
	r.timerGen++
	r.deadline = time.Time{}
}

// onTimer handles an expired deadline; stale timers are ignored.
func (r *Room) onTimer(gen int) {
	if gen != r.timerGen {
		return
	}
	switch r.stage {
	case protocol.StageChoosing:
		// Keep last hand's variant if possible, else the first option.
		v := r.options[0]
		if slices.Contains(r.options, r.variant) {
			v = r.variant
		}
		r.deal(v)
	case protocol.StageHand:
		// Timeout: take 4 cards in a Dacoit choice, show the lowest cards
		// during a reveal; otherwise check if possible, else fold.
		a := engine.Action{Type: engine.Fold}
		switch l := r.hand.Legal(r.hand.ToAct); {
		case l.Choose:
			a = r.hand.DefaultChoose()
		case l.Show:
			a = r.hand.DefaultShow(r.hand.ToAct)
		case l.Check:
			a.Type = engine.Check
		}
		if err := r.hand.Apply(r.hand.ToAct, a); err != nil {
			panic(fmt.Sprintf("room %s: timeout action: %v", r.Code, err))
		}
		r.afterHandChange()
	case protocol.StageResult:
		r.nextHand()
	}
}

// Players returns the players in seat order.
func (r *Room) Players() []*Player { return r.players }
