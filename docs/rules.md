# Poker Room — Rules

This is the source of truth for game rules. Every worked example here is also a test fixture. If code and this doc disagree, fix one of them in the same change.

All chips are virtual. Chip amounts are always whole numbers.

---

## 1. Room & game flow

1. A host creates a room and shares its link. Players join with a nickname.
2. The host sets the [room settings](#2-room-settings) in the lobby and presses **Start**. Settings are locked from then on.
3. Each hand:
   1. The dealer button moves one seat clockwise to the next seated player who is not sitting out. On the first hand, the button goes to a random seat.
   2. The **dealer picks the variant** from the host's allowed list (30 s timer; on timeout the previous variant is reused, or Texas Hold'em on the first hand). Variants the current table can't support (§4.7) can't be picked.
   3. The hand runs according to the chosen variant.
   4. When the hand ends, the result (who won, how much, and with which five cards) stays on screen for about 9 seconds after a showdown, or 4 seconds when everyone else folded, before the next hand.
4. Between hands, broke players may take a [loan](#3-loans).
5. The host ends the game whenever they want. The leaderboard ranks players by **profit = stack − starting stack − debt**. 0 means the player broke even, a positive number means they are up, and a negative one means they are down.

## 2. Room settings

| Setting | Default | Allowed range |
|---|---|---|
| Starting stack | 2000 | 100 – 1,000,000 |
| Loan amount | 2000 | 100 – 1,000,000 |
| Loan penalty | 1000 | 0 – 1,000,000 |
| Penalized loans | 3 | 0 – 100 |
| Small / big blind | 10 / 20 | SB ≥ 1, BB ≥ SB |
| Betting structure | No-limit | No-limit, or pot-limit (Omaha, Dacoit, S2P2) |
| Turn timer | 30 s | 10 – 120 s |
| Allowed variants | all 6 | at least 1 |
| Max players | 8 | 2 – 10 |

## 3. Loans

- A loan is offered only **between hands**, and only to a player whose stack is **0**. They can accept, or decline and sit out.
- When a player takes loan number *k* (counting from 1):
  - `stack += loanAmount`
  - `debt += loanAmount + (k ≤ penalizedLoans ? loanPenalty : 0)`
- Loans are never repaid. (Repayment may be added later.)
- Chips only enter the game through starting stacks and loan principal, so the players' profits always sum to **minus the total penalties paid**. The engine checks this at the end of every game.

**Example (defaults):** a player takes 5 loans. Their debt grows 3000 → 6000 → 9000 → 11000 → 13000. If they finish with 4500 chips, their profit is 4500 − 2000 − 13000 = **−10500**.

---

## 4. Variants

### 4.0 Common rules

**Hand ranking** (high to low): straight flush, four of a kind, full house, flush, straight, three of a kind, two pair, one pair, high card.
- An ace can be high, or low in the wheel straight A-2-3-4-5.
- Kickers break ties. Suits never break ties.
- Every final hand has exactly 5 cards.

**Blinds & action order**
- Small blind is the first player left of the dealer, big blind the next.
- **Heads-up:** the dealer posts the small blind and acts first pre-flop and last on later streets.
- Pre-flop action starts with the player left of the big blind. On later streets it starts with the first active player left of the dealer.
- A player who can't cover a blind posts what they have and is all-in.

**Betting**
- *No-limit:* a bet or raise can be anything up to the player's whole stack.
- *Pot-limit:* the largest raise is the pot size after the player calls.
- *Minimum bet* is the big blind. *Minimum raise* is the size of the last full bet or raise on this street.
- An all-in for less than a full raise does **not** reopen betting for players who have already acted on this street.
- If a bet is not fully called, the uncalled part goes back to the bettor.
- **Turn timeout:** check if possible, otherwise fold.

**Showdown & pots**
- If everyone else folds, the last player wins every pot they're part of and doesn't have to show.
- At showdown, the last aggressor on the final street shows first. If there was no bet on the final street, the first active player left of the dealer shows first. Players who can't win a pot may muck.
- **Side pots** are made in increasing order of each all-in player's total contribution. A player can only win the pots they paid into.
- **Odd chips:** when a pot doesn't split evenly, the leftover chips are handed out one at a time to winners in seat order, starting left of the dealer.

**Phase list** (each variant below lists its own):
`Choice` → `Blinds` → `Deal` → `Show` → `Bet(pre-flop)` → `Flop(3)` → `Bet` → `Turn(1)` → `Bet` → `River(1)` → `Bet` → `Allocate` → `Showdown`

### 4.1 Texas Hold'em

- **Phases:** Blinds, Deal 2, Bet, Flop, Bet, Turn, Bet, River, Bet, Showdown.
- **Final hand:** the best 5 of the 7 cards, using 0, 1 or 2 hole cards.

**Example 4.1-a:** hole K♠ Q♠, board A♠ J♠ T♠ 2♦ 3♣ → royal flush (A♠ K♠ Q♠ J♠ T♠).

**Example 4.1-b (playing the board):** hole 2♣ 3♦, board A♠ K♠ Q♠ J♠ T♠ → royal flush using 0 hole cards. Every player still in the hand ties and splits.

### 4.2 Omaha

- **Phases:** Blinds, Deal 4, Bet, Flop, Bet, Turn, Bet, River, Bet, Showdown.
- **Final hand:** **exactly 2** hole cards + **exactly 3** board cards.

**Example 4.2-a (flush trap):** hole A♥ K♥ Q♥ J♥, board 9♥ 4♣ 7♦ 2♠ K♣.
There's only one heart on the board, so **no flush**. Best hand: pair of kings, K♥ K♣ A♥ 9♥ 7♦.

**Example 4.2-b (board-pair trap):** hole A♠ 2♣ 3♦ 4♥, board K♣ K♦ K♥ Q♠ Q♦.
A full house (K K K Q Q) would need 0 hole cards, so it doesn't count. Best: K♣ K♦ K♥ + A♠ + 4♥ → three kings, A 4 kickers.

### 4.3 Dacoit (house variant)

- **Phases:** **Choice**, Blinds, Deal 4/6, Bet, Flop, Bet, Turn, Bet, River, Bet, Showdown.
- **Choice:** before any cards are dealt, each player except the dealer picks **4 or 6 cards**, in seat order starting left of the dealer. Everyone sees earlier picks. The **dealer always gets 4**. Timeout picks 4.
- **Deck limit:** "6" can't be picked if `cards already chosen + 6 + 4 × (players still to choose, including the dealer) + 5 > 52`.
- **Multiplier:** each player has a multiplier *m*: 1 for 4 cards, 2 for 6 cards. All betting is in **units**, and a player pays `m` chips per unit. That covers blinds, bets, calls, raises and pot-limit maximums.
  - A 6-card player's largest possible bet is `floor(stack / 2)` units. One leftover odd chip just stays in their stack.
  - Side pots are built from each player's total **units**. A pot's chip value is the sum over its contributors of `units in that pot × m`.
  - Pots are won and split in **chips**.
  - *Pot-limit Dacoit:* the largest raise, in units, equals the pot in chips after the player calls.
- **Final hand:** **exactly 2** hole cards + **exactly 3** board cards (same as Omaha).

**Example 4.3-a (pre-flop):** blinds 10/20. Seats in order: D (dealer), A, B, C.
- Picks: A **6**, B **4**, C **6**, D 4 (fixed). Multipliers: A=2, B=1, C=2, D=1.
- Blinds: A posts SB 10 units = **20 chips**. B posts BB 20 units = **20 chips**.
- C calls 20 units = **40 chips**. D raises to 60 units = **60 chips**.
- A calls 60 units: 50 more units = **100 chips** (120 in total).
- B calls: 40 more units = **40 chips** (60 in total).
- C calls: 40 more units = **80 chips** (120 in total).
- Pot = 60 + 120 + 60 + 120 = **360 chips**. Everyone has put in 60 units.

**Example 4.3-b (side pots, continuing 4.3-a):** on the flop A has 200 chips left (= 100 units).
- B bets 300 units (300 chips). A goes all-in for 100 units (200 chips). C calls 300 units (600 chips). D folds.
- Total units over the whole hand: D 60 (folded), A 160 (all-in), B 360, C 360.
- **Main pot** (up to 160 units each), contested by A, B, C:
  D 60×1 + A 160×2 + B 160×1 + C 160×2 = 60 + 320 + 160 + 320 = **860 chips**.
- **Side pot** (units 160 → 360), contested by B and C:
  B 200×1 + C 200×2 = **600 chips**.
- Check: the chips put in are 360 (pre-flop) + 200 + 300 + 600 = 1460 = 860 + 600 ✓.

### 4.4 Double Texas (house variant)

- **Phases:** Blinds, Deal 4, Bet, Flop×2, Bet, Turn×2, Bet, River×2, Bet, **Allocate**, Showdown.
- There are **two boards** of 5 cards, dealt from one deck and revealed together street by street.
- There is **one pot and one betting line** for both boards.
- **Allocate:** after the river betting, every player still in the hand splits their 4 cards into two pairs, one pair per board. Picks are hidden until everyone has locked in. On timeout, the server picks the split whose **weakest** board hand is strongest. Ties go to the stronger second-weakest hand, then to the lowest board number.
- Each board is judged on its own as Texas Hold'em: best 5 of that pair + that board's 5 cards.
- **Scoring:**
  - Each board gives **1 point** to its winner.
  - If players tie on a board, **each tied player gets a full point**.
  - The player with the most points wins the **whole pot**. Players tied on points split it equally (odd chips per §4.0).
  - Each side pot is scored the same way, counting only the players who paid into it.

**Example 4.4-a:** A wins board 1, B wins board 2 → A 1, B 1 → **A and B split**.

**Example 4.4-b:** A and B tie on board 1, A wins board 2 → A 2, B 1 → **A wins the pot**.

**Example 4.4-c (choosing a split):** hole A♠ A♥ 7♣ 8♣. Board 1: 9♣ T♣ 2♦ 3♥ K♠. Board 2: A♦ 4♠ 4♥ J♣ Q♦.
- Split (A♠A♥ | 7♣8♣): board 1 = pair of aces (A A K T 9). Board 2 = pair of fours (4 4 Q J 8).
- Split (7♣8♣ | A♠A♥): board 1 = king high (K T 9 8 7). Board 2 = **aces full of fours** (A A A 4 4).
- Which split is better depends on what the opponents hold, and that is the point of the game.

### 4.5 Triple Texas (house variant)

- Same as Double Texas, with **6 hole cards** and **three boards**. Cards are allocated 2 + 2 + 2.
- **Can only be picked with 6 or fewer players in the hand** (6 × 6 + 15 = 51 ≤ 52).
- Same scoring: most points wins, and players tied on a board each get a full point.

**Example 4.5-a:** A and B tie on board 1, C wins board 2, D wins board 3 → A 1, B 1, C 1, D 1 → **all four split the pot**.

**Example 4.5-b:** A wins boards 1 and 2, B wins board 3 → **A wins the pot**.

### 4.6 Show 2 Play 2 (house variant)

- **Phases:** Blinds, Deal 4, **Show**, Bet, Flop, Bet, Turn, Bet, River, Bet, Showdown.
- **Show:** in seat order, starting left of the dealer, each player picks 2 of their 4 cards and shows them to everyone. Each reveal is visible before the next player picks. On timeout, the server reveals the player's 2 lowest-ranked cards.
- **Final hand:** **exactly 3** board cards + **exactly 1** shown card + **exactly 1** hidden card.

**Example 4.6-a:** shown A♠ A♦, hidden K♣ 5♥, board A♣ K♦ K♥ 9♠ 2♣.
- Aces full (A A A K K) would need both shown aces, so it's not allowed.
- Best: A♠ (shown) + K♣ (hidden) + K♦ K♥ A♣ → **kings full of aces** (K K K A A).

### 4.7 Table limits (single 52-card deck)

| Variant | Cards needed | Max players in hand |
|---|---|---|
| Texas Hold'em | 2n + 5 | room max |
| Omaha | 4n + 5 | 10 (room max) |
| Show 2 Play 2 | 4n + 5 | 10 (room max) |
| Dacoit | Σ picks + 5 | 10, with "6" restricted by the deck limit |
| Double Texas | 4n + 10 | 10 |
| Triple Texas | 6n + 15 | **6** |
