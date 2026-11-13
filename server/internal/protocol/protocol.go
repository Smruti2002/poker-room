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
	MsgTakeLoan       = "takeLoan"       // below 500 chips, between hands
	MsgSitOut         = "sitOut"
	MsgSitIn          = "sitIn"
	MsgEndGame        = "endGame"    // host only
	MsgChat           = "chat"       // Text: a chat message to the table
	MsgReact          = "react"      // Emoji: a quick reaction shown over your seat
	MsgHistory        = "history"    // ask for the recent hand history (sent only to you)
	MsgNextHand       = "nextHand"   // any player, once a hand is over
	MsgSkip           = "skip"       // host only: Target gets the default move, or a broke player sits out
	MsgClaimHost      = "claimHost"  // become host while the host is offline
	MsgRematch        = "rematch"    // host only, after the game: a new room with the same players
	MsgVoteKick       = "voteKick"   // Target, Yes: start or answer a vote to remove a player
	MsgCancelKick     = "cancelKick" // Target: whoever started the vote withdraws it
	MsgAddBot         = "addBot"     // host only: seat a computer player
	MsgRemoveBot      = "removeBot"  // host only: Target is a bot to remove
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
	Text     string    `json:"text,omitempty"`
	Emoji    string    `json:"emoji,omitempty"`
	Target   string    `json:"target,omitempty"` // player ID, for skip and kick votes
	Yes      bool      `json:"yes,omitempty"`    // a kick vote's answer
}

// Action is a betting action. To is the street total for bet/raise, in units.
type Action struct {
	Type string `json:"type"` // fold, check, call, bet, raise, show
	To   int64  `json:"to,omitempty"`
	// Cards are the hole cards to reveal, for show (Show 2 Play 2).
	Cards []string `json:"cards,omitempty"`
	// Count is how many hole cards to take, for choose (Dacoit: 4 or 6).
	Count int `json:"count,omitempty"`
}

// Server message types.
const (
	MsgWelcome      = "welcome" // Token and PlayerID for this connection
	MsgState        = "state"   // full room snapshot for this viewer
	MsgError        = "error"   // a request failed; Error says why
	MsgHistoryReply = "history" // History: recent hands, as this viewer may see them
	MsgRemoved      = "removed" // the other players voted this player out of the room
)

// ServerMsg is sent to the browser.
type ServerMsg struct {
	Type     string        `json:"type"`
	Token    string        `json:"token,omitempty"`
	PlayerID string        `json:"playerId,omitempty"`
	State    *State        `json:"state,omitempty"`
	History  []HistoryHand `json:"history,omitempty"`
	Error    string        `json:"error,omitempty"`
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
	Variants       []string `json:"variants"`
	MaxPlayers     int      `json:"maxPlayers"`
	// Hints shows each player the name of their current best hand.
	Hints bool `json:"hints"`
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
	Variants    []string      `json:"variants"`  // every variant the server knows
	Chat        []ChatLine    `json:"chat"`      // the last messages, oldest first
	Reactions   []Reaction    `json:"reactions"` // reactions from the last few seconds
	Now         int64         `json:"now"`       // server time, unix ms
	// NextRoom is set once the host has started a new game with the same
	// players; the client moves there.
	NextRoom *NextRoom `json:"nextRoom,omitempty"`
	// ClosesAt is when the room will be closed for good (unix ms), however
	// busy it is; 0 if never.
	ClosesAt int64 `json:"closesAt,omitempty"`
	// KickVotes are the votes in progress, oldest first.
	KickVotes []KickVote `json:"kickVotes"`
	// WaitingOnLoan are broke players, online and not sitting out, that
	// Next hand waits for: they take a loan or sit out, or the host deals
	// without them.
	WaitingOnLoan []string `json:"waitingOnLoan,omitempty"`
	// Stats are the whole game's numbers, sent once it's over.
	Stats *GameStats `json:"stats,omitempty"`
}

// GameStats sum up a finished game, from what everyone saw at the table.
type GameStats struct {
	Hands int `json:"hands"`
	// Variants are hands played per variant, most played first.
	Variants   []VariantCount `json:"variants"`
	BiggestPot *PotStat       `json:"biggestPot,omitempty"`
	// Players are everyone dealt at least one hand, in seat order.
	Players []PlayerStats `json:"players"`
}

// VariantCount is how many hands of a variant were played.
type VariantCount struct {
	Variant string `json:"variant"`
	Hands   int    `json:"hands"`
}

// PotStat is the hand that paid out the most chips.
type PotStat struct {
	Chips   int64    `json:"chips"`
	Hand    int      `json:"hand"`
	Variant string   `json:"variant"`
	Winners []string `json:"winners"`
}

// PlayerStats are one player's numbers for the game.
type PlayerStats struct {
	ID           string `json:"id"`
	Hands        int    `json:"hands"`        // dealt in
	Won          int    `json:"won"`          // hands where they won chips
	Folded       int    `json:"folded"`       // hands they folded
	Showdowns    int    `json:"showdowns"`    // hands they were still in at a showdown
	ShowdownsWon int    `json:"showdownsWon"` // ...and won chips
	BiggestWin   int64  `json:"biggestWin"`   // most chips won in one hand
	// Net is chips won or lost per variant, by name.
	Net []VariantNet `json:"net"`
}

// VariantNet is a player's chips won or lost playing one variant.
type VariantNet struct {
	Variant string `json:"variant"`
	Chips   int64  `json:"chips"`
}

// NextRoom is where the players went after a game ended. Token is this
// viewer's own session in the new room, so it only appears in their view.
type NextRoom struct {
	Code  string `json:"code"`
	Token string `json:"token,omitempty"`
}

// ChatLine is one chat message.
type ChatLine struct {
	ID       int    `json:"id"`
	PlayerID string `json:"playerId"`
	Name     string `json:"name"`
	Text     string `json:"text"`
	At       int64  `json:"at"` // unix ms
}

// Reaction is a quick emoji shown over a player's seat for a moment.
type Reaction struct {
	ID       int    `json:"id"`
	PlayerID string `json:"playerId"`
	Emoji    string `json:"emoji"`
	At       int64  `json:"at"` // unix ms
}

// HistoryHand is a finished hand as one viewer may see it.
type HistoryHand struct {
	Number  int          `json:"number"`
	Variant string       `json:"variant"`
	Boards  [][]string   `json:"boards"`
	Winners []WinnerView `json:"winners"`
	Log     []string     `json:"log"`
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
	// CanBorrow: the player may take a loan now: they're below the early-loan
	// line (500 chips) and not in a hand being played.
	CanBorrow bool `json:"canBorrow,omitempty"`
	// Kicked players were voted out; they stay listed for the leaderboard
	// and until a hand they were in has finished.
	Kicked bool `json:"kicked,omitempty"`
	// Bot is a computer player the host added; it plays by itself.
	Bot bool `json:"bot,omitempty"`
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
	Shown []string `json:"shown,omitempty"`
	// Allocated is true once the player has locked in their split (Double/
	// Triple Texas); the split itself is hidden until the showdown.
	Allocated bool `json:"allocated,omitempty"`
	// Split is the pair the player played on each board, after the showdown.
	Split [][]string `json:"split,omitempty"`
	// Picked is how many hole cards the player chose (Dacoit), public.
	Picked   int    `json:"picked,omitempty"`
	HandDesc string `json:"handDesc,omitempty"`
	// Draws is how many cards the player exchanged in each BB exchange so
	// far; Exchanging is true while they're still choosing this exchange.
	Draws      []int `json:"draws,omitempty"`
	Exchanging bool  `json:"exchanging,omitempty"`
	// LastBet is the player's bet or raise on the current street, until
	// they act again or the street ends.
	LastBet *BetCallout `json:"lastBet,omitempty"`
	Won     int64       `json:"won,omitempty"`
}

// HandView is the current (or just finished) hand.
type HandView struct {
	Number  int    `json:"number"`
	Variant string `json:"variant"`
	Street  string `json:"street"`
	// Showing is true while players reveal cards before betting (Show 2 Play 2).
	Showing bool `json:"showing,omitempty"`
	// Allocating is true while players split their cards across the boards
	// (Double/Triple Texas). There is no single player to act.
	Allocating bool `json:"allocating,omitempty"`
	// BoardWinners lists, per board, who had the best hand there (after the
	// showdown).
	BoardWinners [][]string `json:"boardWinners,omitempty"`
	// Picking is true while players choose 4 or 6 cards, before the deal (Dacoit).
	Picking bool `json:"picking,omitempty"`
	// Drawing is true during a BB exchange: everyone still in chooses which
	// cards to exchange, all at once. DrawRound counts the exchanges done.
	Drawing   bool `json:"drawing,omitempty"`
	DrawRound int  `json:"drawRound,omitempty"`
	// Reveal is how many cards each player shows in this variant.
	Reveal     int        `json:"reveal,omitempty"`
	Boards     [][]string `json:"boards"`
	Pot        int64      `json:"pot"` // chips, including current bets
	Pots       []PotView  `json:"pots,omitempty"`
	ToAct      string     `json:"toAct,omitempty"`
	CurrentBet int64      `json:"currentBet"`
	Legal      *Legal     `json:"legal,omitempty"` // only for the player to act
	Done       bool       `json:"done"`
	// Hint names the viewer's current best hand, when the host allows hints
	// and there are enough cards to make one.
	Hint string `json:"hint,omitempty"`
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
	Show bool `json:"show,omitempty"`
	// Choose: take 4 cards, or up to ChooseMax (Dacoit). It is then the only
	// legal action.
	Choose    bool `json:"choose,omitempty"`
	ChooseMax int  `json:"chooseMax,omitempty"`
	// Allocate: split your cards, two per board, in board order.
	Allocate bool `json:"allocate,omitempty"`
	// Draw: choose which of your cards (0-5) to exchange (BB).
	Draw      bool  `json:"draw,omitempty"`
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
}

// Standing is one leaderboard row.
type Standing struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Profit int64  `json:"profit"`
	Kicked bool   `json:"kicked,omitempty"`
}

// BetCallout describes a bet or raise for the table to see, in the chips
// the player actually put in.
type BetCallout struct {
	Type  string `json:"type"`  // bet or raise
	By    int64  `json:"by"`    // how much more than the bet they faced (a bet: its size)
	To    int64  `json:"to"`    // their total on this street
	AllIn bool   `json:"allIn"` // the bet or raise put them all-in
}

// KickVote is an open vote to remove Target from the room. It passes once
// Needed of the connected players other than Target have voted yes.
type KickVote struct {
	Target    string   `json:"target"`
	StartedBy string   `json:"startedBy"`
	Yes       []string `json:"yes"`
	No        []string `json:"no"`
	Needed    int      `json:"needed"`
}
