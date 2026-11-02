// Package engine runs a single poker hand: blinds, dealing, betting rounds,
// side pots and showdown. It is pure game logic with no I/O or clock; the
// room layer feeds it actions and reads its state and events.
//
// All betting is in units. A player pays Mult chips per unit (1 normally,
// 2 for a Dacoit 6-card player), so pots are built from units but paid out
// in chips. See docs/rules.md §4.0 and §4.3.
package engine

import (
	"fmt"
	"slices"

	"pokerroom/internal/cards"
	"pokerroom/internal/eval"
)

// MaxPlayers is the most players a hand can have.
const MaxPlayers = 10

// Player is one participant's state within a hand.
type Player struct {
	ID    string       `json:"id"`
	Stack int64        `json:"stack"` // chips behind
	Mult  int64        `json:"mult"`  // chips per unit
	Hole  []cards.Card `json:"hole"`

	Folded bool  `json:"folded"`
	AllIn  bool  `json:"allIn"`
	Street int64 `json:"street"` // units committed on the current street
	Total  int64 `json:"total"`  // units committed this hand
	Won    int64 `json:"won"`    // chips won when the hand ends

	acted bool  // has acted on the current street
	faced int64 // the current bet when this player last acted
}

func (p *Player) maxUnits() int64 { return p.Stack / p.Mult }
func (p *Player) canAct() bool    { return !p.Folded && !p.AllIn }

// commit moves units from the player's stack into the pot.
func (p *Player) commit(units int64) {
	p.Street += units
	p.Total += units
	p.Stack -= units * p.Mult
	p.AllIn = p.Stack < p.Mult
}

// Options control how a hand is dealt.
type Options struct {
	// Seed drives the shuffle. Log it to make the hand replayable.
	Seed cards.Seed
	// Deck, if set, is dealt in this exact order instead of shuffling:
	// each player's hole cards in player order, then each street's board
	// cards. Used by tests.
	Deck []cards.Card
}

// Hand is one hand of poker in progress or finished.
type Hand struct {
	Config  Config
	Variant Variant
	Players []*Player

	Dealer     int
	SmallBlind int
	BigBlind   int

	Boards [][]cards.Card
	Street Street
	// ToAct is the index of the player to act, or -1.
	ToAct int
	// CurrentBet is the street total (units) players must match.
	CurrentBet int64
	// MinRaise is the size of the last full bet or raise on this street.
	MinRaise int64

	Done bool
	// Pots is filled in when the hand ends.
	Pots   []Pot
	Events []Event

	deck  *cards.Deck
	log   Log
	chips int64 // total chips in play, for invariant checks
	ranks [][]eval.Hand
}

// NewHand validates the seats, posts blinds and deals hole cards. Players
// are listed clockwise; dealer is the index of the dealer button.
func NewHand(cfg Config, v Variant, seats []SeatSpec, dealer int, opts Options) (*Hand, error) {
	n := len(seats)
	switch {
	case n < 2 || n > MaxPlayers:
		return nil, fmt.Errorf("need 2-%d players, got %d", MaxPlayers, n)
	case cardsNeeded(v, n) > cards.NumCards:
		return nil, fmt.Errorf("%s needs %d cards for %d players", v.Name(), cardsNeeded(v, n), n)
	case cfg.SmallBlind < 1 || cfg.BigBlind < cfg.SmallBlind:
		return nil, fmt.Errorf("invalid blinds %d/%d", cfg.SmallBlind, cfg.BigBlind)
	case cfg.Structure != NoLimit && cfg.Structure != PotLimit:
		return nil, fmt.Errorf("invalid betting structure %d", cfg.Structure)
	case dealer < 0 || dealer >= n:
		return nil, fmt.Errorf("dealer %d out of range", dealer)
	}

	h := &Hand{
		Config:  cfg,
		Variant: v,
		Dealer:  dealer,
		Boards:  make([][]cards.Card, v.Boards()),
		log: Log{
			Variant: v.Name(), Config: cfg, Seats: slices.Clone(seats),
			Dealer: dealer, Seed: opts.Seed, Deck: opts.Deck,
		},
	}
	for i, s := range seats {
		mult := s.Mult
		if mult == 0 {
			mult = 1
		}
		if mult != 1 && mult != 2 {
			return nil, fmt.Errorf("player %d: invalid multiplier %d", i, mult)
		}
		if s.Stack < mult {
			return nil, fmt.Errorf("player %d: stack %d is too small to play", i, s.Stack)
		}
		h.Players = append(h.Players, &Player{ID: s.ID, Stack: s.Stack, Mult: mult})
		h.chips += s.Stack
	}

	if opts.Deck != nil {
		var seen cards.Set
		for _, c := range opts.Deck {
			if !c.Valid() || !seen.Add(c) {
				return nil, fmt.Errorf("invalid or duplicate card %v in deck", c)
			}
		}
		if len(opts.Deck) < cardsNeeded(v, n) {
			return nil, fmt.Errorf("deck has %d cards, need %d", len(opts.Deck), cardsNeeded(v, n))
		}
		h.deck = cards.NewDeckOf(opts.Deck)
	} else {
		h.deck = cards.NewShuffledDeck(opts.Seed)
	}

	// Heads-up, the dealer posts the small blind.
	if n == 2 {
		h.SmallBlind, h.BigBlind = dealer, h.next(dealer)
	} else {
		h.SmallBlind = h.next(dealer)
		h.BigBlind = h.next(h.SmallBlind)
	}
	h.postBlind(h.SmallBlind, cfg.SmallBlind)
	h.postBlind(h.BigBlind, cfg.BigBlind)

	for i, p := range h.Players {
		p.Hole = h.deal(v.HoleCards())
		h.emit(Event{Kind: EvHole, Player: i, Cards: p.Hole})
	}

	// Players must match the full big blind even if the big blind is short.
	h.Street = Preflop
	h.CurrentBet = cfg.BigBlind
	h.MinRaise = cfg.BigBlind
	if err := h.advance(h.BigBlind); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *Hand) next(i int) int { return (i + 1) % len(h.Players) }

func (h *Hand) deal(n int) []cards.Card {
	c, err := h.deck.Deal(n)
	if err != nil {
		panic("engine: deck ran out; NewHand checks capacity") // unreachable
	}
	return c
}

func (h *Hand) postBlind(i int, units int64) {
	p := h.Players[i]
	units = min(units, p.maxUnits())
	p.commit(units)
	h.emit(Event{Kind: EvBlind, Player: i, Units: units, Chips: units * p.Mult})
}

// Legal returns what player i may do now. It is the zero value unless it
// is player i's turn.
func (h *Hand) Legal(i int) Legal {
	if h.Done || i != h.ToAct {
		return Legal{}
	}
	p := h.Players[i]
	var l Legal
	toCall := h.CurrentBet - p.Street
	if toCall > 0 {
		l.Fold, l.Call = true, true
		l.CallUnits = min(toCall, p.maxUnits())
	} else {
		l.Check = true
	}

	maxTo := p.Street + p.maxUnits()
	if maxTo <= h.CurrentBet || !h.mayRaise(i) {
		return l
	}
	if h.Config.Structure == PotLimit {
		// The largest raise (in units) is the pot in chips after calling.
		maxTo = min(maxTo, h.CurrentBet+h.potChips()+toCall*p.Mult)
	}
	l.MinTo = min(h.CurrentBet+h.MinRaise, maxTo) // all-in for less is allowed
	l.MaxTo = maxTo
	if h.CurrentBet == 0 {
		l.Bet = true
	} else {
		l.Raise = true
	}
	return l
}

// mayRaise reports whether player i can raise: someone else must still be
// able to act, and an all-in for less than a full raise doesn't reopen the
// betting for players who already acted (docs/rules.md §4.0). Several short
// all-ins that add up to a full raise do reopen it.
func (h *Hand) mayRaise(i int) bool {
	others := false
	for j, q := range h.Players {
		if j != i && q.canAct() {
			others = true
		}
	}
	p := h.Players[i]
	return others && (!p.acted || h.CurrentBet-p.faced >= h.MinRaise)
}

// PotChips is the total chips committed this hand, including current bets.
func (h *Hand) PotChips() int64 { return h.potChips() }

func (h *Hand) potChips() int64 {
	var sum int64
	for _, p := range h.Players {
		sum += p.Total * p.Mult
	}
	return sum
}

// Apply performs player i's action.
func (h *Hand) Apply(i int, a Action) error {
	if h.Done {
		return ErrHandOver
	}
	if i != h.ToAct {
		return ErrNotYourTurn
	}
	p := h.Players[i]
	l := h.Legal(i)
	illegal := func(why string) error { return fmt.Errorf("%w: %v: %s", ErrIllegalAction, a, why) }

	switch a.Type {
	case Fold:
		if !l.Fold {
			return illegal("nothing to call, check instead")
		}
		p.Folded = true
	case Check:
		if !l.Check {
			return illegal(fmt.Sprintf("facing a bet of %d", h.CurrentBet))
		}
	case Call:
		if !l.Call {
			return illegal("nothing to call")
		}
		p.commit(l.CallUnits)
	case Bet, Raise:
		if (a.Type == Bet && !l.Bet) || (a.Type == Raise && !l.Raise) {
			return illegal("not allowed now")
		}
		if a.To < l.MinTo || a.To > l.MaxTo {
			return illegal(fmt.Sprintf("amount must be %d-%d", l.MinTo, l.MaxTo))
		}
		if increase := a.To - h.CurrentBet; increase >= h.MinRaise {
			h.MinRaise = increase // a full raise
		}
		p.commit(a.To - p.Street)
		h.CurrentBet = a.To
	default:
		return illegal("unknown action")
	}

	p.acted = true
	p.faced = h.CurrentBet
	h.log.Actions = append(h.log.Actions, LoggedAction{Player: i, Action: a})
	h.emit(Event{Kind: EvAction, Player: i, Action: a, Units: p.Street, Chips: p.Street * p.Mult})
	return h.advance(i)
}

// advance moves play on after player `from` acted (or the blinds were posted).
func (h *Hand) advance(from int) error {
	if h.liveCount() == 1 {
		return h.finish()
	}
	if !h.roundOver() {
		h.ToAct = h.nextToAct(from)
		return nil
	}
	return h.endStreet()
}

func (h *Hand) liveCount() int {
	n := 0
	for _, p := range h.Players {
		if !p.Folded {
			n++
		}
	}
	return n
}

func (h *Hand) needsToAct(p *Player) bool {
	return p.canAct() && (!p.acted || p.Street < h.CurrentBet)
}

// roundOver reports whether the current betting round is complete.
func (h *Hand) roundOver() bool {
	var able []*Player
	for _, p := range h.Players {
		if p.canAct() {
			able = append(able, p)
		}
	}
	switch len(able) {
	case 0:
		return true
	case 1:
		// Alone against all-in players: only act if still behind their bets.
		p := able[0]
		for _, q := range h.Players {
			if q != p && q.Street > p.Street {
				return false
			}
		}
		return true
	}
	for _, p := range able {
		if h.needsToAct(p) {
			return false
		}
	}
	return true
}

func (h *Hand) nextToAct(from int) int {
	for i := h.next(from); ; i = h.next(i) {
		if h.needsToAct(h.Players[i]) {
			return i
		}
	}
}

// endStreet closes a betting round and deals the next street, running the
// board out when no more betting is possible.
func (h *Hand) endStreet() error {
	h.returnUncalled()
	for _, p := range h.Players {
		p.Street, p.acted, p.faced = 0, false, 0
	}
	h.CurrentBet = 0
	h.MinRaise = h.Config.BigBlind

	if h.Street == River {
		return h.finish()
	}
	h.Street++
	for b := range h.Boards {
		c := h.deal(h.Street.boardCards())
		h.Boards[b] = append(h.Boards[b], c...)
		h.emit(Event{Kind: EvBoard, Player: -1, Board: b, Cards: c})
	}

	able := 0
	for _, p := range h.Players {
		if p.canAct() {
			able++
		}
	}
	if able < 2 {
		return h.endStreet()
	}
	h.ToAct = h.nextToAct(h.Dealer)
	return nil
}

// returnUncalled gives back the part of the top bet nobody matched.
func (h *Hand) returnUncalled() {
	top, second := -1, int64(0)
	for i, p := range h.Players {
		if top < 0 || p.Street > h.Players[top].Street {
			if top >= 0 {
				second = max(second, h.Players[top].Street)
			}
			top = i
		} else {
			second = max(second, p.Street)
		}
	}
	p := h.Players[top]
	if excess := p.Street - second; excess > 0 {
		p.Street -= excess
		p.Total -= excess
		p.Stack += excess * p.Mult
		p.AllIn = p.Stack < p.Mult
		h.emit(Event{Kind: EvRefund, Player: top, Units: excess, Chips: excess * p.Mult})
	}
}

// finish ends the hand: the last player standing takes everything, otherwise
// each pot goes to its best eligible hand(s).
func (h *Hand) finish() error {
	h.returnUncalled()
	h.ToAct = -1
	h.Street = Showdown
	h.Pots = buildPots(h.Players)

	showdown := h.liveCount() > 1
	if showdown {
		h.ranks = make([][]eval.Hand, len(h.Players))
		for i, p := range h.Players {
			if p.Folded {
				continue
			}
			r, err := h.Variant.Rank(p, h.Boards)
			if err != nil {
				return fmt.Errorf("engine: ranking player %d: %w", i, err)
			}
			h.ranks[i] = r
			h.emit(Event{Kind: EvShow, Player: i, Cards: p.Hole, Desc: describe(r)})
		}
	}

	for pi, pot := range h.Pots {
		winners := pot.Eligible
		if showdown {
			winners = h.potWinners(pot.Eligible)
		}
		for w, chips := range splitPot(pot.Chips, winners, h.Dealer, len(h.Players)) {
			h.Players[w].Stack += chips
			h.Players[w].Won += chips
			h.emit(Event{Kind: EvWin, Player: w, Pot: pi, Chips: chips, Desc: h.handDesc(w)})
		}
	}
	for _, p := range h.Players {
		p.Street = 0
		p.AllIn = false
	}
	h.Done = true
	return nil
}

// potWinners picks the winners among eligible players. With one board, the
// best hand wins. With several boards, each board's best hand(s) score a
// point (ties each score a full point) and the most points win
// (docs/rules.md §4.4).
func (h *Hand) potWinners(eligible []int) []int {
	points := make(map[int]int, len(eligible))
	for b := range h.Boards {
		var best eval.HandRank
		for _, i := range eligible {
			best = max(best, h.ranks[i][b].Rank)
		}
		for _, i := range eligible {
			if h.ranks[i][b].Rank == best {
				points[i]++
			}
		}
	}
	top := 0
	for _, pts := range points {
		top = max(top, pts)
	}
	var winners []int
	for _, i := range eligible {
		if points[i] == top {
			winners = append(winners, i)
		}
	}
	return winners
}

func (h *Hand) handDesc(i int) string {
	if h.ranks == nil || h.ranks[i] == nil {
		return ""
	}
	return describe(h.ranks[i])
}

func describe(r []eval.Hand) string {
	s := ""
	for b, hand := range r {
		if b > 0 {
			s += " | "
		}
		s += hand.Rank.String()
	}
	return s
}

// Log returns the record needed to replay this hand.
func (h *Hand) Log() Log {
	l := h.log
	l.Actions = slices.Clone(l.Actions)
	return l
}
