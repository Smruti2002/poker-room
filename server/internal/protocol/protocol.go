// Package protocol defines the JSON messages exchanged over the WebSocket.
// These types are the source of truth: web/src/protocol/types.ts is
// generated from them with tygo (make protocol).
package protocol

// Client message types.
const (
	MsgJoin           = "join"           // Name: join as a new player
	MsgResume         = "resume"         // Token: reconnect as an existing player
	MsgUpdateSettings = "updateSettings" // host only, lobby only
	MsgStart          = "start"          // host only
	MsgChooseVariant  = "chooseVariant"  // dealer only
	MsgAction         = "action"         // player to act
	MsgTakeLoan       = "takeLoan"       // broke player, between hands
	MsgSitOut         = "sitOut"
	MsgSitIn          = "sitIn"
	MsgEndGame        = "endGame" // host only
)

// ClientMsg is sent by the browser. The first message on a connection must
// be join or resume.
type ClientMsg struct {
	Type     string    `json:"type"`
	Name     string    `json:"name,omitempty"`
	Token    string    `json:"token,omitempty"`
	Settings *Settings `json:"settings,omitempty"`
	Variant  string    `json:"variant,omitempty"`
	Action   *Action   `json:"action,omitempty"`
}

// Action is a betting action. To is the street total for bet/raise, in units.
type Action struct {
	Type string `json:"type"` // fold, check, call, bet, raise, show
	To   int64  `json:"to,omitempty"`
	// Cards are the hole cards to reveal, for show (Show 2 Play 2).
	Cards []string `json:"cards,omitempty"`
}

// Server message types.
const (
	MsgWelcome = "welcome" // Token and PlayerID for this connection
	MsgState   = "state"   // full room snapshot for this viewer
	MsgError   = "error"   // a request failed; Error says why
)

// ServerMsg is sent to the browser.
type ServerMsg struct {
	Type     string `json:"type"`
	Token    string `json:"token,omitempty"`
	PlayerID string `json:"playerId,omitempty"`
	State    *State `json:"state,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Settings are the host's room options (docs/rules.md §2).
type Settings struct {
	StartingStack  int64    `json:"startingStack"`
	LoanAmount     int64    `json:"loanAmount"`
	LoanPenalty    int64    `json:"loanPenalty"`
	PenalizedLoans int      `json:"penalizedLoans"`
	SmallBlind     int64    `json:"smallBlind"`
	BigBlind       int64    `json:"bigBlind"`
	PotLimit       bool     `json:"potLimit"` // pot-limit for Omaha, Dacoit and S2P2
	TurnSeconds    int      `json:"turnSeconds"`
	Variants       []string `json:"variants"`
	MaxPlayers     int      `json:"maxPlayers"`
}

// Room phases.
const (
	PhaseLobby   = "lobby"
	PhasePlaying = "playing"
	PhaseEnded   = "ended"
)

// Stages within the playing phase.
const (
	StageWaiting  = "waiting"  // fewer than 2 players can play
	StageChoosing = "choosing" // dealer picks the variant
	StageHand     = "hand"     // a hand is being played
	StageResult   = "result"   // showing the finished hand before the next
)

// State is a snapshot of the room as one viewer may see it. Other players'
// hole cards are only included once shown at showdown.
type State struct {
	Code        string        `json:"code"`
	Phase       string        `json:"phase"`
	Stage       string        `json:"stage,omitempty"`
	You         string        `json:"you"`
	HostID      string        `json:"hostId"`
	Settings    Settings      `json:"settings"`
	Players     []PlayerView  `json:"players"`
	Hand        *HandView     `json:"hand,omitempty"`
	Choosing    *ChoosingView `json:"choosing,omitempty"`
	Log         []string      `json:"log"`
	Leaderboard []Standing    `json:"leaderboard"`
	Variants    []string      `json:"variants"` // every variant the server knows
	Now         int64         `json:"now"`      // server time, unix ms
}

// PlayerView is one seat.
type PlayerView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Stack      int64  `json:"stack"` // chips behind, live during a hand
	Debt       int64  `json:"debt"`
	Loans      int    `json:"loans"`
	Profit     int64  `json:"profit"` // stack − starting stack − debt
	Connected  bool   `json:"connected"`
	SittingOut bool   `json:"sittingOut"`
	Broke      bool   `json:"broke"` // no chips; may take a loan
	// The fields below describe the current or last hand.
	InHand bool  `json:"inHand"`
	Dealer bool  `json:"dealer"`
	Folded bool  `json:"folded"`
	AllIn  bool  `json:"allIn"`
	Bet    int64 `json:"bet"`  // units on the current street
	Mult   int64 `json:"mult"` // chips per unit
	// HoleCount is how many hole cards the player holds, so others can draw
	// the right number of face-down cards.
	HoleCount int      `json:"holeCount,omitempty"`
	Hole      []string `json:"hole,omitempty"`
	// Shown are hole cards revealed to everyone before betting (Show 2 Play 2).
	Shown    []string `json:"shown,omitempty"`
	HandDesc string   `json:"handDesc,omitempty"`
	Won      int64    `json:"won,omitempty"`
}

// HandView is the current (or just finished) hand.
type HandView struct {
	Number  int    `json:"number"`
	Variant string `json:"variant"`
	Street  string `json:"street"`
	// Showing is true while players reveal cards before betting (Show 2 Play 2).
	Showing bool `json:"showing,omitempty"`
	// Reveal is how many cards each player shows in this variant.
	Reveal     int        `json:"reveal,omitempty"`
	Boards     [][]string `json:"boards"`
	Pot        int64      `json:"pot"` // chips, including current bets
	Pots       []PotView  `json:"pots,omitempty"`
	ToAct      string     `json:"toAct,omitempty"`
	CurrentBet int64      `json:"currentBet"`
	// Deadline is when the player to act times out or, once the hand is
	// done, when the next hand starts (unix ms).
	Deadline int64  `json:"deadline,omitempty"`
	Legal    *Legal `json:"legal,omitempty"` // only for the player to act
	Done     bool   `json:"done"`
	// Winners is filled in when the hand is done.
	Winners []WinnerView `json:"winners,omitempty"`
}

// WinnerView is a player who won chips, and how.
type WinnerView struct {
	PlayerID string `json:"playerId"`
	Chips    int64  `json:"chips"`
	// Desc is the winning hand, empty when everyone else folded.
	Desc string `json:"desc,omitempty"`
	// Cards are the five cards that make the hand, per board.
	Cards [][]string `json:"cards,omitempty"`
}

// PotView is a main or side pot after the hand ends.
type PotView struct {
	Chips    int64    `json:"chips"`
	Eligible []string `json:"eligible"`
}

// Legal lists what the player to act may do; amounts are in units.
type Legal struct {
	// Show: reveal cards (Show 2 Play 2). It is then the only legal action.
	Show      bool  `json:"show,omitempty"`
	Fold      bool  `json:"fold"`
	Check     bool  `json:"check"`
	Call      bool  `json:"call"`
	CallUnits int64 `json:"callUnits"`
	Bet       bool  `json:"bet"`
	Raise     bool  `json:"raise"`
	MinTo     int64 `json:"minTo"`
	MaxTo     int64 `json:"maxTo"`
}

// ChoosingView is shown while the dealer picks the variant.
type ChoosingView struct {
	DealerID string   `json:"dealerId"`
	Options  []string `json:"options"`
	Deadline int64    `json:"deadline"`
}

// Standing is one leaderboard row.
type Standing struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Profit int64  `json:"profit"`
}
