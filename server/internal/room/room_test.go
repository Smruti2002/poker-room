package room

import (
	"encoding/json"
	"errors"
	mrand "math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"pokerroom/internal/cards"
	"pokerroom/internal/engine"
	"pokerroom/internal/protocol"
)

type harness struct {
	t  *testing.T
	r  *Room
	ps []*Player
}

func newHarness(t *testing.T, names ...string) *harness {
	t.Helper()
	return newHarnessOpts(t, Options{}, names...)
}

func newHarnessOpts(t *testing.T, opts Options, names ...string) *harness {
	t.Helper()
	h := &harness{t: t}
	seed := uint64(0)
	// Hold'em only unless a test says otherwise, so hands deal without the
	// dealer having to pick a variant.
	if opts.Variants == nil {
		opts.Variants = map[string]engine.Variant{"holdem": engine.Holdem}
	}
	opts.Now = func() time.Time { return time.UnixMilli(1_000_000) }
	opts.Rand = mrand.New(mrand.NewPCG(1, 2))
	opts.NewSeed = func() cards.Seed { seed++; return cards.SeedFromUint64(seed) }
	h.r = New("ABCDEF", opts)
	for _, n := range names {
		p, err := h.r.Join(n)
		if err != nil {
			t.Fatal(err)
		}
		h.ps = append(h.ps, p)
	}
	return h
}

func (h *harness) do(p *Player, m protocol.ClientMsg) error {
	return h.r.Handle(p.ID, m)
}

func (h *harness) must(p *Player, m protocol.ClientMsg) {
	h.t.Helper()
	if err := h.do(p, m); err != nil {
		h.t.Fatalf("%s %s: %v", p.Name, m.Type, err)
	}
}

// advance does what the table now waits for a person to do: tap "Next
// hand" after a result, or the host skipping whoever the game is waiting on
// (the dealer choosing, the player to act, or everyone still splitting).
func (h *harness) advance() {
	h.t.Helper()
	if err := h.tryAdvance(); err != nil {
		h.t.Fatalf("advance in stage %q: %v", h.r.stage, err)
	}
}

func (h *harness) tryAdvance() error {
	host := h.r.hostID
	skip := func(id string) error { return h.r.Handle(host, protocol.ClientMsg{Type: protocol.MsgSkip, Target: id}) }
	switch h.r.stage {
	case protocol.StageResult:
		return h.r.Handle(host, protocol.ClientMsg{Type: protocol.MsgNextHand})
	case protocol.StageChoosing:
		return skip(h.r.dealerID)
	case protocol.StageHand:
		if h.r.hand.Allocating || h.r.hand.Drawing {
			for _, i := range append(h.r.hand.Pending(), h.r.hand.DrawPending()...) {
				if err := skip(h.r.handIDs[i]); err != nil {
					return err
				}
			}
			return nil
		}
		return skip(h.r.handIDs[h.r.hand.ToAct])
	}
	return errors.New("nothing to advance")
}

func (h *harness) start() {
	h.t.Helper()
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgStart})
}

// toAct is the player whose turn it is.
func (h *harness) toAct() *Player {
	h.t.Helper()
	if h.r.stage != protocol.StageHand || h.r.hand.Done {
		h.t.Fatalf("no hand in progress (stage %q)", h.r.stage)
	}
	return h.r.player(h.r.handIDs[h.r.hand.ToAct])
}

// passive checks or calls until the hand ends; in multi-board hands
// everyone takes the default split.
func (h *harness) playPassively() {
	h.t.Helper()
	for h.r.stage == protocol.StageHand && !h.r.hand.Done {
		if h.r.hand.Drawing {
			for _, i := range h.r.hand.DrawPending() {
				h.must(h.r.player(h.r.handIDs[i]), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "draw"}})
			}
			continue
		}
		if h.r.hand.Allocating {
			for _, i := range h.r.hand.Pending() {
				split := cardStrings(h.r.hand.DefaultSplit(i).Cards)
				h.must(h.r.player(h.r.handIDs[i]), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "allocate", Cards: split}})
			}
			continue
		}
		p := h.toAct()
		l := h.r.hand.Legal(h.r.hand.ToAct)
		a := &protocol.Action{Type: "call"}
		if l.Check {
			a.Type = "check"
		}
		h.must(p, protocol.ClientMsg{Type: protocol.MsgAction, Action: a})
	}
}

func (h *harness) totalStacks() int64 {
	var sum int64
	for _, p := range h.r.players {
		sum += p.Stack
	}
	return sum
}

func TestJoin(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	if h.r.hostID != h.ps[0].ID {
		t.Fatal("first player should be host")
	}
	if h.ps[0].Stack != 2000 || h.ps[0].Token == "" || h.ps[0].Token == h.ps[1].Token {
		t.Fatalf("bad player setup: %+v %+v", h.ps[0], h.ps[1])
	}
	if h.r.PlayerByToken(h.ps[1].Token) != h.ps[1] || h.r.PlayerByToken("nope") != nil || h.r.PlayerByToken("") != nil {
		t.Fatal("PlayerByToken lookup broken")
	}

	for _, bad := range []string{"", "   ", strings.Repeat("x", 21), "bad\nname", "ann", " BOB "} {
		if _, err := h.r.Join(bad); err == nil {
			t.Errorf("Join(%q) should fail", bad)
		}
	}
	if _, err := h.r.Join(" Cy "); err != nil {
		t.Fatal(err)
	}
	if h.r.players[2].Name != "Cy" {
		t.Fatalf("name not trimmed: %q", h.r.players[2].Name)
	}

	h.r.settings.MaxPlayers = 3
	if _, err := h.r.Join("Dee"); !errors.Is(err, ErrRoomFull) {
		t.Fatalf("err = %v, want ErrRoomFull", err)
	}
}

func TestSettings(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	s := DefaultSettings([]string{"holdem"})
	s.StartingStack = 5000
	s.SmallBlind, s.BigBlind = 25, 50

	if err := h.do(h.ps[1], protocol.ClientMsg{Type: protocol.MsgUpdateSettings, Settings: &s}); !errors.Is(err, ErrNotHost) {
		t.Fatalf("non-host update: %v", err)
	}
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgUpdateSettings}); err == nil {
		t.Fatal("missing settings should fail")
	}
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgUpdateSettings, Settings: &s})
	if h.ps[1].Stack != 5000 || h.r.settings.BigBlind != 50 {
		t.Fatal("settings not applied")
	}

	invalid := map[string]func(*protocol.Settings){
		"stack too small":    func(s *protocol.Settings) { s.StartingStack = 50 },
		"negative penalty":   func(s *protocol.Settings) { s.LoanPenalty = -1 },
		"big below small":    func(s *protocol.Settings) { s.BigBlind = 5 },
		"no variants":        func(s *protocol.Settings) { s.Variants = nil },
		"unknown variant":    func(s *protocol.Settings) { s.Variants = []string{"go-fish"} },
		"duplicate variant":  func(s *protocol.Settings) { s.Variants = []string{"holdem", "holdem"} },
		"max below joined":   func(s *protocol.Settings) { s.MaxPlayers = 1 },
		"max over 10":        func(s *protocol.Settings) { s.MaxPlayers = 11 },
		"loan amount absurd": func(s *protocol.Settings) { s.LoanAmount = 10_000_000 },
	}
	for name, mutate := range invalid {
		bad := DefaultSettings([]string{"holdem"})
		mutate(&bad)
		if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgUpdateSettings, Settings: &bad}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	h.start()
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgUpdateSettings, Settings: &s}); err == nil {
		t.Fatal("settings should be locked after start")
	}
}

func TestStart(t *testing.T) {
	h := newHarness(t, "Ann")
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgStart}); err == nil {
		t.Fatal("start with one player should fail")
	}
	bob, _ := h.r.Join("Bob")
	if err := h.r.Handle(bob.ID, protocol.ClientMsg{Type: protocol.MsgStart}); !errors.Is(err, ErrNotHost) {
		t.Fatalf("non-host start: %v", err)
	}
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgStart})
	if h.r.phase != protocol.PhasePlaying || h.r.stage != protocol.StageHand {
		t.Fatalf("phase %q stage %q", h.r.phase, h.r.stage)
	}
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgStart}); err == nil {
		t.Fatal("second start should fail")
	}
	if err := h.r.Handle("p99", protocol.ClientMsg{Type: protocol.MsgStart}); err == nil {
		t.Fatal("unknown player should fail")
	}
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: "dance"}); !errors.Is(err, ErrUnknownMsg) {
		t.Fatalf("unknown message: %v", err)
	}
}

func TestHandsPlayAndDealerRotates(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	var dealers []string
	for range 6 {
		dealers = append(dealers, h.r.dealerID)
		h.playPassively()
		if h.r.stage != protocol.StageResult {
			t.Fatalf("stage %q after hand", h.r.stage)
		}
		if got := h.totalStacks(); got != 6000 {
			t.Fatalf("chips not conserved: %d", got)
		}
		h.advance() // result pause -> next hand
	}
	// The button moves one seat clockwise every hand.
	order := []string{"p1", "p2", "p3"}
	start := slices.Index(order, dealers[0])
	for i, d := range dealers {
		if want := order[(start+i)%3]; d != want {
			t.Fatalf("dealers = %v", dealers)
		}
	}
	if h.r.handNo != 7 {
		t.Fatalf("handNo = %d", h.r.handNo)
	}
}

func TestActionErrors(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	act := func(p *Player, a *protocol.Action) error {
		return h.do(p, protocol.ClientMsg{Type: protocol.MsgAction, Action: a})
	}
	if err := act(h.ps[0], &protocol.Action{Type: "call"}); err == nil {
		t.Fatal("action in lobby should fail")
	}
	h.start()
	p := h.toAct()
	var other *Player
	for _, q := range h.ps {
		if q != p {
			other = q
			break
		}
	}
	if err := act(other, &protocol.Action{Type: "call"}); !errors.Is(err, ErrNotYourTurn) {
		t.Fatalf("out of turn: %v", err)
	}
	if err := act(p, nil); err == nil {
		t.Fatal("missing action should fail")
	}
	if err := act(p, &protocol.Action{Type: "shove"}); err == nil {
		t.Fatal("unknown action type should fail")
	}
	if err := act(p, &protocol.Action{Type: "check"}); !errors.Is(err, engine.ErrIllegalAction) {
		t.Fatalf("check facing big blind: %v", err)
	}
}

func TestTurnTimeoutChecksOrFolds(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	first := h.r.hand.ToAct
	h.advance() // facing the big blind: fold
	if !h.r.hand.Players[first].Folded {
		t.Fatal("timed-out player facing a bet should fold")
	}
	// Small blind calls; big blind times out with the option: check.
	h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "call"}})
	bb := h.r.hand.ToAct
	h.advance()
	if h.r.hand.Players[bb].Folded || h.r.hand.Street != engine.Flop {
		t.Fatalf("big blind should have checked to the flop; street %v", h.r.hand.Street)
	}
}

func TestDealerChoosesVariant(t *testing.T) {
	opts := Options{Variants: map[string]engine.Variant{"holdem": engine.Holdem, "alt": engine.Holdem}}
	h := newHarnessOpts(t, opts, "Ann", "Bob", "Cy")
	h.start()
	if h.r.stage != protocol.StageChoosing {
		t.Fatalf("stage = %q", h.r.stage)
	}
	v := h.r.View(h.ps[0].ID)
	if v.Choosing == nil || v.Choosing.DealerID != h.r.dealerID || len(v.Choosing.Options) != 2 {
		t.Fatalf("choosing view = %+v", v.Choosing)
	}

	dealer := h.r.player(h.r.dealerID)
	var other *Player
	for _, p := range h.ps {
		if p != dealer {
			other = p
		}
	}
	choose := func(p *Player, name string) error {
		return h.do(p, protocol.ClientMsg{Type: protocol.MsgChooseVariant, Variant: name})
	}
	if err := choose(other, "alt"); err == nil {
		t.Fatal("non-dealer choice should fail")
	}
	if err := choose(dealer, "omaha"); err == nil {
		t.Fatal("unavailable variant should fail")
	}
	if err := choose(dealer, "alt"); err != nil {
		t.Fatal(err)
	}
	if h.r.stage != protocol.StageHand || h.r.variant != "alt" {
		t.Fatalf("stage %q variant %q", h.r.stage, h.r.variant)
	}
	if err := choose(dealer, "alt"); err == nil {
		t.Fatal("choosing outside the choosing stage should fail")
	}

	// Next dealer times out: the previous variant is kept.
	h.playPassively()
	h.advance()
	if h.r.stage != protocol.StageChoosing {
		t.Fatalf("stage = %q", h.r.stage)
	}
	h.advance()
	if h.r.stage != protocol.StageHand || h.r.variant != "alt" {
		t.Fatalf("timeout should reuse alt; got %q", h.r.variant)
	}
}

func TestLoans(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	ann := h.ps[0]
	if err := h.do(ann, protocol.ClientMsg{Type: protocol.MsgTakeLoan}); err == nil {
		t.Fatal("loan with chips should fail")
	}
	// docs/rules.md §3: debt grows 3000, 6000, 9000, 11000, 13000.
	for _, want := range []int64{3000, 6000, 9000, 11000, 13000} {
		ann.Stack = 0
		h.must(ann, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
		if ann.Debt != want || ann.Stack != 2000 {
			t.Fatalf("debt %d stack %d, want debt %d", ann.Debt, ann.Stack, want)
		}
	}
	ann.Stack = 4500
	if got := h.r.Profit(ann); got != -10500 {
		t.Fatalf("profit = %d, want -10500", got)
	}
}

// Busted players are skipped; with fewer than two players the room waits
// until someone takes a loan.
func TestBrokePlayersAndWaiting(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	h.playPassively()
	loser, winner := h.ps[0], h.ps[1]
	if loser.Stack > winner.Stack {
		loser, winner = winner, loser
	}
	// Simulate a bust between hands.
	winner.Stack += loser.Stack
	loser.Stack = 0
	h.advance()
	if h.r.stage != protocol.StageWaiting {
		t.Fatalf("stage = %q, want waiting", h.r.stage)
	}
	v := h.r.View(loser.ID)
	if !v.Players[slices.IndexFunc(v.Players, func(p protocol.PlayerView) bool { return p.ID == loser.ID })].Broke {
		t.Fatal("busted player should be marked broke")
	}
	h.must(loser, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
	if h.r.stage != protocol.StageHand {
		t.Fatalf("loan should restart play; stage %q", h.r.stage)
	}
}

func TestSitOutAndIn(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	cy := h.ps[2]
	h.must(cy, protocol.ClientMsg{Type: protocol.MsgSitOut})
	h.playPassively()
	h.advance()
	if slices.Contains(h.r.handIDs, cy.ID) {
		t.Fatal("sitting-out player was dealt in")
	}
	h.must(h.ps[1], protocol.ClientMsg{Type: protocol.MsgSitOut})
	h.playPassively()
	h.advance()
	if h.r.stage != protocol.StageWaiting {
		t.Fatalf("stage %q, want waiting with one active player", h.r.stage)
	}
	h.must(cy, protocol.ClientMsg{Type: protocol.MsgSitIn})
	if h.r.stage != protocol.StageHand || !slices.Contains(h.r.handIDs, cy.ID) {
		t.Fatal("sitting back in should deal a hand")
	}
}

func TestLateJoinerPlaysNextHand(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	dee, err := h.r.Join("Dee")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(h.r.handIDs, dee.ID) {
		t.Fatal("late joiner dealt into a hand in progress")
	}
	h.playPassively()
	h.advance()
	if !slices.Contains(h.r.handIDs, dee.ID) {
		t.Fatal("late joiner not dealt the next hand")
	}
}

func TestEndGame(t *testing.T) {
	h := newHarness(t, "Ann", "Bob")
	h.start()
	if err := h.do(h.ps[1], protocol.ClientMsg{Type: protocol.MsgEndGame}); !errors.Is(err, ErrNotHost) {
		t.Fatalf("non-host end: %v", err)
	}
	h.must(h.ps[0], protocol.ClientMsg{Type: protocol.MsgEndGame})
	if h.r.phase != protocol.PhasePlaying {
		t.Fatal("game must not end mid-hand")
	}
	h.playPassively() // the game ends as soon as that hand is over
	if h.r.phase != protocol.PhaseEnded {
		t.Fatalf("phase = %q after the hand", h.r.phase)
	}
	if err := h.do(h.ps[0], protocol.ClientMsg{Type: protocol.MsgEndGame}); !errors.Is(err, ErrGameOver) {
		t.Fatalf("second end: %v", err)
	}
	if _, err := h.r.Join("Late"); !errors.Is(err, ErrGameOver) {
		t.Fatalf("join after end: %v", err)
	}
	h.ps[1].Stack = 0
	if err := h.do(h.ps[1], protocol.ClientMsg{Type: protocol.MsgTakeLoan}); !errors.Is(err, ErrGameOver) {
		t.Fatalf("loan after end: %v", err)
	}

	// Ending between hands is immediate.
	h2 := newHarness(t, "Ann", "Bob")
	h2.must(h2.ps[0], protocol.ClientMsg{Type: protocol.MsgEndGame})
	if h2.r.phase != protocol.PhaseEnded {
		t.Fatal("ending in the lobby should be immediate")
	}
}

func TestViewHidesOtherPlayersCards(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()
	for _, viewer := range h.ps {
		v := h.r.View(viewer.ID)
		for _, pv := range v.Players {
			if pv.ID == viewer.ID && len(pv.Hole) != 2 {
				t.Fatalf("%s can't see own cards", viewer.Name)
			}
			if pv.ID != viewer.ID && pv.Hole != nil {
				t.Fatalf("%s can see %s's cards", viewer.Name, pv.Name)
			}
		}
		for _, line := range v.Log {
			if strings.Contains(line, "is dealt") && !strings.HasPrefix(line, viewer.Name+" ") {
				t.Fatalf("%s's log leaks: %q", viewer.Name, line)
			}
		}
		// Legal actions only for the player to act.
		if (v.Hand.Legal != nil) != (v.Hand.ToAct == viewer.ID) {
			t.Fatalf("legal shown to %s while %s acts", viewer.Name, v.Hand.ToAct)
		}
	}
	// A spectator-like viewer sees no hole cards at all.
	for _, pv := range h.r.View("nobody").Players {
		if pv.Hole != nil {
			t.Fatal("unknown viewer sees cards")
		}
	}

	// First player folds, the rest check down: shown hands become public,
	// the folded hand stays hidden.
	folder := h.toAct()
	h.must(folder, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
	h.playPassively()
	for _, viewer := range h.ps {
		for _, pv := range h.r.View(viewer.ID).Players {
			switch {
			case pv.ID == folder.ID && viewer.ID != folder.ID && pv.Hole != nil:
				t.Fatalf("%s sees the folded hand", viewer.Name)
			case pv.ID != folder.ID && (len(pv.Hole) != 2 || pv.HandDesc == ""):
				t.Fatalf("%s doesn't see %s's shown hand", viewer.Name, pv.Name)
			}
		}
	}
	v := h.r.View(h.ps[0].ID)
	if !v.Hand.Done || len(v.Hand.Pots) == 0 || v.Hand.Pot == 0 || v.Hand.Legal != nil {
		t.Fatalf("finished hand view = %+v", v.Hand)
	}
}

// Chips only enter through starting stacks and loans, so profits always
// sum to minus the penalties paid (docs/rules.md §3).
func TestProfitsSumToMinusPenalties(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy", "Dee")
	h.r.settings.SmallBlind, h.r.settings.BigBlind = 100, 200
	h.start()
	rng := mrand.New(mrand.NewPCG(3, 4))
	for range 300 {
		switch h.r.stage {
		case protocol.StageHand:
			p := h.toAct()
			l := h.r.hand.Legal(h.r.hand.ToAct)
			a := &protocol.Action{Type: "call"}
			switch {
			case l.Raise && rng.IntN(3) == 0:
				a = &protocol.Action{Type: "raise", To: l.MaxTo}
			case l.Check:
				a.Type = "check"
			}
			h.must(p, protocol.ClientMsg{Type: protocol.MsgAction, Action: a})
		default:
			for _, p := range h.ps {
				if p.Stack == 0 {
					h.must(p, protocol.ClientMsg{Type: protocol.MsgTakeLoan})
				}
			}
			if h.r.stage != protocol.StageHand {
				h.advance()
			}
		}
	}
	h.playPassively()

	var profits, penalties int64
	loans := 0
	for _, p := range h.ps {
		profits += h.r.Profit(p)
		penalties += int64(min(p.Loans, h.r.settings.PenalizedLoans)) * h.r.settings.LoanPenalty
		loans += p.Loans
	}
	if loans == 0 {
		t.Fatal("test should exercise loans")
	}
	if profits != -penalties {
		t.Fatalf("profits sum %d, want %d (loans %d)", profits, -penalties, loans)
	}
}

func FuzzRoom(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte("start the game and play some hands"))
	f.Fuzz(func(t *testing.T, script []byte) {
		h := newHarnessOpts(t, Options{Variants: engine.AllVariants()}, "Ann", "Bob", "Cy")
		h.r.Join("Dee")
		players := h.r.players
		variants := h.r.variantNames
		base := int64(len(players)) * h.r.settings.StartingStack
		for i := 0; i+1 < len(script); i += 2 {
			p := players[int(script[i])%len(players)]
			op := script[i+1]
			var m protocol.ClientMsg
			switch op % 10 {
			case 0:
				m.Type = protocol.MsgStart
			case 1:
				m = protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "call"}}
			case 2:
				m = protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "check"}}
				if h.r.hand != nil && h.r.hand.Allocating {
					if idx := slices.Index(h.r.handIDs, p.ID); idx >= 0 {
						m.Action = &protocol.Action{Type: "allocate", Cards: cardStrings(h.r.hand.Players[idx].Hole)}
					}
				}
				if h.r.hand != nil && h.r.hand.Drawing {
					if idx := slices.Index(h.r.handIDs, p.ID); idx >= 0 {
						hole := cardStrings(h.r.hand.Players[idx].Hole)
						m.Action = &protocol.Action{Type: "draw", Cards: hole[:int(op)%(len(hole)+1)]}
					}
				}
				if h.r.hand != nil && h.r.hand.Picking {
					m.Action = &protocol.Action{Type: "choose", Count: 4 + 2*int(op%2)}
				}
				if op%4 == 2 && h.r.hand != nil && h.r.hand.Showing {
					// Reveal the first two hole cards of the player to act.
					hole := cardStrings(h.r.hand.Players[h.r.hand.ToAct].Hole)
					m.Action = &protocol.Action{Type: "show", Cards: hole[:2]}
				}
			case 3:
				m = protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "raise", To: int64(op) * 7}}
			case 4:
				m = protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}}
			case 5:
				m.Type = protocol.MsgTakeLoan
			case 6:
				m.Type = protocol.MsgSitOut
			case 7:
				m.Type = protocol.MsgSitIn
			case 8:
				_ = h.tryAdvance()
				continue
			case 9:
				switch {
				case op > 250:
					m.Type = protocol.MsgEndGame
				case op > 200:
					// Come and go, and vote people out.
					p.Connections = int(op % 2)
					target := players[int(op/3)%len(players)]
					m = protocol.ClientMsg{Type: protocol.MsgVoteKick, Target: target.ID, Yes: op%4 < 2}
					h.r.resolveKicks()
				default:
					m = protocol.ClientMsg{Type: protocol.MsgChooseVariant, Variant: variants[int(op)%len(variants)]}
				}
			}
			_ = h.r.Handle(p.ID, m) // errors are fine; panics and broken invariants are not
			for _, pp := range players {
				_ = h.r.View(pp.ID)
			}
			if h.r.stage != protocol.StageHand {
				var loans int64
				for _, pp := range players {
					loans += int64(pp.Loans) * h.r.settings.LoanAmount
				}
				if got := h.totalStacks(); got != base+loans {
					t.Fatalf("chips %d, want %d", got, base+loans)
				}
			} else if err := h.r.hand.CheckInvariants(); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// Finished hands report who won and how, and stay up until a player taps
// "Next hand": nothing moves on by itself.
func TestWinnersAndNextHand(t *testing.T) {
	h := newHarness(t, "Ann", "Bob", "Cy")
	h.start()

	// Hand 1: everyone folds to the big blind, whose unmatched 10 comes back.
	for h.r.stage == protocol.StageHand && !h.r.hand.Done {
		h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "fold"}})
	}
	hv := h.r.View(h.ps[0].ID).Hand
	if len(hv.Winners) != 1 || hv.Winners[0].Desc != "" || hv.Winners[0].Cards != nil || hv.Winners[0].Chips != 20 {
		t.Fatalf("fold win = %+v", hv.Winners)
	}
	if h.r.stage != protocol.StageResult {
		t.Fatalf("stage %q", h.r.stage)
	}

	// Any player, not just the host, starts the next hand; only once.
	bob := h.ps[1]
	h.must(bob, protocol.ClientMsg{Type: protocol.MsgNextHand})
	if h.r.stage != protocol.StageHand || h.r.handNo != 2 {
		t.Fatalf("stage %q hand %d", h.r.stage, h.r.handNo)
	}
	if err := h.do(bob, protocol.ClientMsg{Type: protocol.MsgNextHand}); err == nil {
		t.Fatal("next hand mid-hand should fail")
	}

	// Hand 2: checked down to a showdown.
	h.playPassively()
	hv = h.r.View(h.ps[0].ID).Hand
	if len(hv.Winners) == 0 {
		t.Fatal("no winners")
	}
	var paid int64
	for _, w := range hv.Winners {
		paid += w.Chips
		if w.Desc == "" || len(w.Cards) != 1 || len(w.Cards[0]) != 5 {
			t.Fatalf("showdown winner = %+v", w)
		}
	}
	if paid != hv.Pot {
		t.Fatalf("winners got %d of a %d pot", paid, hv.Pot)
	}
}

func TestShowTwoInARoom(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"show2": engine.ShowTwo}}, "Ann", "Bob", "Cy")
	h.start()
	if !h.r.hand.Showing {
		t.Fatal("hand should start with the reveal phase")
	}
	first := h.toAct()
	v := h.r.View(first.ID)
	if v.Hand.Legal == nil || !v.Hand.Legal.Show || !v.Hand.Showing || v.Hand.Reveal != 2 {
		t.Fatalf("hand view = %+v legal %+v", v.Hand, v.Hand.Legal)
	}
	show := func(p *Player, cs ...string) error {
		return h.do(p, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "show", Cards: cs}})
	}
	if err := show(first, "Zz", "As"); err == nil {
		t.Fatal("a malformed card should be rejected")
	}
	hole := cardStrings(h.r.hand.Players[slices.Index(h.r.handIDs, first.ID)].Hole)
	if err := show(first, hole[0], hole[1]); err != nil {
		t.Fatal(err)
	}

	// Everyone sees the two shown cards, but not the other two.
	for _, viewer := range h.ps {
		for _, pv := range h.r.View(viewer.ID).Players {
			if pv.HoleCount != 4 {
				t.Fatalf("hole count %d", pv.HoleCount)
			}
			if pv.ID == first.ID {
				if !slices.Equal(pv.Shown, hole[:2]) {
					t.Fatalf("%s sees shown %v", viewer.Name, pv.Shown)
				}
				if viewer.ID != first.ID && pv.Hole != nil {
					t.Fatalf("%s sees %s's hidden cards", viewer.Name, pv.Name)
				}
			}
		}
	}

	// The next player times out: their lowest two cards are shown.
	second := h.toAct()
	idx := slices.Index(h.r.handIDs, second.ID)
	want := h.r.hand.DefaultShow(idx).Cards
	h.advance()
	if !slices.Equal(h.r.hand.Players[idx].Shown, want) {
		t.Fatalf("timed-out reveal = %v, want %v", h.r.hand.Players[idx].Shown, want)
	}

	// Last reveal, then betting starts and the hand plays out.
	third := h.toAct()
	h.advance()
	if h.r.hand.Showing {
		t.Fatalf("still showing after %s", third.Name)
	}
	h.playPassively()
	if got := h.totalStacks(); got != 6000 {
		t.Fatalf("chips = %d", got)
	}
}

func TestOmahaInARoom(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"omaha": engine.Omaha}}, "Ann", "Bob")
	h.r.settings.PotLimit = true
	h.start()
	if h.r.hand.Config.Structure != engine.PotLimit {
		t.Fatal("pot-limit setting should apply to Omaha")
	}
	v := h.r.View(h.ps[0].ID)
	for _, pv := range v.Players {
		if pv.HoleCount != 4 || (pv.ID == h.ps[0].ID && len(pv.Hole) != 4) {
			t.Fatalf("player view %+v", pv)
		}
	}
	h.playPassively()
	if h.r.stage != protocol.StageResult {
		t.Fatalf("stage %q", h.r.stage)
	}
}

// Regression (found by FuzzRoom): players sitting out while the dealer is
// choosing must not crash the deal.
func TestSitOutWhileDealerChooses(t *testing.T) {
	opts := Options{Variants: map[string]engine.Variant{"holdem": engine.Holdem, "omaha": engine.Omaha}}
	h := newHarnessOpts(t, opts, "Ann", "Bob", "Cy")
	h.start()
	if h.r.stage != protocol.StageChoosing {
		t.Fatalf("stage %q", h.r.stage)
	}
	for _, p := range h.ps {
		if p.ID != h.r.dealerID {
			h.must(p, protocol.ClientMsg{Type: protocol.MsgSitOut})
		}
	}
	h.advance() // dealer's time runs out with nobody left to play
	if h.r.stage != protocol.StageWaiting {
		t.Fatalf("stage %q, want waiting", h.r.stage)
	}

	// The dealer sitting out mid-choice passes the button on.
	for _, p := range h.ps {
		h.must(p, protocol.ClientMsg{Type: protocol.MsgSitIn})
	}
	dealer := h.r.player(h.r.dealerID)
	h.must(dealer, protocol.ClientMsg{Type: protocol.MsgSitOut})
	h.advance()
	if h.r.dealerID == dealer.ID || h.r.stage != protocol.StageChoosing {
		t.Fatalf("dealer %s stage %q", h.r.dealerID, h.r.stage)
	}
}

func TestDacoitInARoom(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"dacoit": engine.Dacoit}}, "Ann", "Bob", "Cy")
	h.r.settings.PotLimit = true
	h.start()
	if !h.r.hand.Picking || h.r.hand.Config.Structure != engine.PotLimit {
		t.Fatal("dacoit should start with the choice, pot-limit when enabled")
	}
	first := h.toAct()
	v := h.r.View(first.ID)
	if l := v.Hand.Legal; l == nil || !l.Choose || l.ChooseMax != 6 || !v.Hand.Picking {
		t.Fatalf("hand %+v legal %+v", v.Hand, v.Hand.Legal)
	}
	h.must(first, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "choose", Count: 6}})

	// Everyone sees the choice; nobody has cards yet.
	for _, viewer := range h.ps {
		for _, pv := range h.r.View(viewer.ID).Players {
			if pv.ID == first.ID && (pv.Picked != 6 || pv.Mult != 2) {
				t.Fatalf("%s sees %s picked %d mult %d", viewer.Name, pv.Name, pv.Picked, pv.Mult)
			}
			if pv.Hole != nil || pv.HoleCount != 0 {
				t.Fatalf("cards before the deal: %+v", pv)
			}
		}
	}

	// The next player times out and takes 4; the dealer is fixed at 4, so
	// the hand is dealt.
	h.advance()
	if h.r.hand.Picking {
		t.Fatal("still picking after the last choice")
	}
	counts := map[string]int{}
	for _, pv := range h.r.View(first.ID).Players {
		counts[pv.ID] = pv.HoleCount
	}
	if counts[first.ID] != 6 {
		t.Fatalf("hole counts %v", counts)
	}
	for id, n := range counts {
		if id != first.ID && n != 4 {
			t.Fatalf("hole counts %v", counts)
		}
	}
	h.playPassively()
	if got := h.totalStacks(); got != 6000 {
		t.Fatalf("chips = %d", got)
	}
}

// Regression: before any event (Dacoit's choice phase) the log must still be
// an empty JSON array, not null; null crashed the web client.
// BB has no board at all: boards must still be [].
func TestViewListsAreNeverNull(t *testing.T) {
	for name, v := range map[string]engine.Variant{"dacoit": engine.Dacoit, "bb": engine.BB} {
		h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{name: v}}, "Ann", "Bob")
		h.start()
		for _, viewer := range h.ps {
			b, err := json.Marshal(h.r.View(viewer.ID))
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{`"log":null`, `"leaderboard":null`, `"players":null`, `"variants":null`, `"boards":null`, `"chat":null`, `"reactions":null`} {
				if strings.Contains(string(b), field) {
					t.Fatalf("%s: %s in %s", name, field, b)
				}
			}
		}
	}
}

func TestDoubleTexasInARoom(t *testing.T) {
	h := newHarnessOpts(t, Options{Variants: map[string]engine.Variant{"double": engine.DoubleTexas}}, "Ann", "Bob", "Cy")
	h.start()
	// Check it down to the river.
	for !h.r.hand.Allocating {
		l := h.r.hand.Legal(h.r.hand.ToAct)
		a := &protocol.Action{Type: "call"}
		if l.Check {
			a.Type = "check"
		}
		h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: a})
	}

	// Everyone may split now, in any order.
	for _, p := range h.ps {
		v := h.r.View(p.ID)
		if !v.Hand.Allocating || v.Hand.ToAct != "" || v.Hand.Legal == nil || !v.Hand.Legal.Allocate {
			t.Fatalf("%s view: %+v legal %+v", p.Name, v.Hand, v.Hand.Legal)
		}
	}
	last := h.ps[2]
	idx := slices.Index(h.r.handIDs, last.ID)
	hole := cardStrings(h.r.hand.Players[idx].Hole)
	h.must(last, protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "allocate", Cards: hole}})
	for _, viewer := range h.ps {
		for _, pv := range h.r.View(viewer.ID).Players {
			if pv.ID != last.ID {
				continue
			}
			if !pv.Allocated {
				t.Fatalf("%s doesn't see %s locked in", viewer.Name, pv.Name)
			}
			if (viewer.ID == last.ID) != (pv.Split != nil) {
				t.Fatalf("%s sees split %v", viewer.Name, pv.Split)
			}
		}
	}

	// The host skips the other two: they get the default split, and the
	// showdown shows every split and who won each board.
	h.advance()
	if h.r.stage != protocol.StageResult {
		t.Fatalf("stage %q", h.r.stage)
	}
	v := h.r.View(h.ps[0].ID)
	if len(v.Hand.BoardWinners) != 2 || len(v.Hand.Winners) == 0 {
		t.Fatalf("board winners %v winners %v", v.Hand.BoardWinners, v.Hand.Winners)
	}
	for _, pv := range v.Players {
		if len(pv.Split) != 2 || !strings.Contains(pv.HandDesc, " | ") {
			t.Fatalf("%s split %v desc %q", pv.Name, pv.Split, pv.HandDesc)
		}
	}
	if got := h.totalStacks(); got != 6000 {
		t.Fatalf("chips = %d", got)
	}
}

// The host can move the game on when it's waiting for someone; nobody else
// can, and only for the player the game is actually waiting on.
func TestHostSkip(t *testing.T) {
	opts := Options{Variants: map[string]engine.Variant{"holdem": engine.Holdem, "dacoit": engine.Dacoit}}
	h := newHarnessOpts(t, opts, "Ann", "Bob", "Cy")
	h.r.settings.Variants = []string{"holdem", "dacoit"} // a skipped dealer gets the first
	ann := h.ps[0]
	skip := func(by *Player, target string) error {
		return h.do(by, protocol.ClientMsg{Type: protocol.MsgSkip, Target: target})
	}
	if err := skip(ann, "p2"); err == nil {
		t.Fatal("nothing to skip in the lobby")
	}
	h.start()

	// Dealer is choosing: only the dealer can be skipped, only by the host.
	var other *Player
	for _, p := range h.ps[1:] {
		if p.ID != h.r.dealerID {
			other = p
		}
	}
	if err := skip(other, h.r.dealerID); !errors.Is(err, ErrNotHost) {
		t.Fatalf("non-host skip: %v", err)
	}
	if err := skip(ann, other.ID); err == nil {
		t.Fatal("skipping someone the game isn't waiting on should fail")
	}
	h.must(ann, protocol.ClientMsg{Type: protocol.MsgSkip, Target: h.r.dealerID})
	if h.r.stage != protocol.StageHand || h.r.variant != "holdem" {
		t.Fatalf("skipped choice: stage %q variant %q", h.r.stage, h.r.variant)
	}

	// Facing the big blind, a skipped player folds; later, checks.
	first := h.r.hand.ToAct
	h.must(ann, protocol.ClientMsg{Type: protocol.MsgSkip, Target: h.r.handIDs[first]})
	if !h.r.hand.Players[first].Folded {
		t.Fatal("skipped player facing a bet should fold")
	}
	h.must(h.toAct(), protocol.ClientMsg{Type: protocol.MsgAction, Action: &protocol.Action{Type: "call"}})
	bb := h.r.hand.ToAct
	h.must(ann, protocol.ClientMsg{Type: protocol.MsgSkip, Target: h.r.handIDs[bb]})
	if h.r.hand.Players[bb].Folded || h.r.hand.Street != engine.Flop {
		t.Fatal("skipped big blind with the option should check")
	}

	// Nobody moves on by itself: the table waits indefinitely.
	before := h.r.hand.ToAct
	h.r.View(ann.ID)
	if h.r.hand.ToAct != before || h.r.stage != protocol.StageHand {
		t.Fatal("the hand moved without anyone acting")
	}

	// After the hand there's nobody to skip; Next hand is for everyone.
	h.playPassively()
	if err := skip(ann, h.ps[1].ID); err == nil {
		t.Fatal("skip during the result should fail")
	}
	h.must(h.ps[2], protocol.ClientMsg{Type: protocol.MsgNextHand})

	// Dacoit: a skipped chooser takes 4 cards.
	if h.r.stage == protocol.StageChoosing {
		h.must(h.r.player(h.r.dealerID), protocol.ClientMsg{Type: protocol.MsgChooseVariant, Variant: "dacoit"})
	}
	if h.r.variant == "dacoit" {
		chooser := h.r.hand.ToAct
		h.must(ann, protocol.ClientMsg{Type: protocol.MsgSkip, Target: h.r.handIDs[chooser]})
		if h.r.hand.Players[chooser].Picked != 4 {
			t.Fatalf("skipped chooser picked %d", h.r.hand.Players[chooser].Picked)
		}
	}
}
