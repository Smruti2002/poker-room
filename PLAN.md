# Poker Room — Build & Deploy Plan

A browser-based, multiplayer, **play-money** poker room in the style of richup.io: no sign-up, create a room, share a link, friends join, host configures and starts. Each hand the **dealer picks the variant**, including four house variants (Dacoit, Double Texas, Triple Texas, Show 2 Play 2).

---

## 1. Product scope

### 1.1 Core flow
1. Visitor enters a nickname → **Create room** → gets a share link (`/r/AB12CD`).
2. Friends open the link, enter nickname, take a seat (or watch as spectators).
3. Host edits **room settings** in the lobby, then hits **Start**.
4. Each hand, the dealer button moves one seat clockwise. The **dealer picks the variant** (30 s timer; on timeout the previous variant is reused).
5. Hands run until the host ends the game; a final **leaderboard** shows each player's profit (stack − starting stack − debt).

### 1.2 Host settings (lobby, locked once the game starts)

| Setting | Default | Notes |
|---|---|---|
| Starting stack | 2000 | |
| Loan amount | 2000 | Chips given when a player is broke |
| Loan penalty | 1000 | Extra debt added per penalized loan |
| Penalized loans | 3 | Loans 1..N carry the penalty; loans after N carry none |
| Small / big blind | 10 / 20 | Optional: blinds increase every X hands |
| Betting structure | No-limit | Pot-limit option for Omaha-style variants |
| Turn timer | 30 s | Auto check, or fold on timeout |
| Allowed variants | all 6 | Host can turn variants off |
| Max players | 8 | Each variant also has its own card limit (see §2.7) |
| Show-down muck allowed | yes | |

### 1.3 Loan rules
- A player whose stack hits **0 after a hand** is offered a loan (they can also choose to sit out).
- Taking loan *k*: `stack += loanAmount`; `debt += loanAmount + (k ≤ penalizedLoans ? loanPenalty : 0)`.
- **Profit = stack − starting stack − debt** is what the leaderboard shows and ranks on. 0 means the player broke even.
- Loans are only taken **between hands**, never during a hand.
- With defaults: loans 1–3 each add 3000 to debt, loan 4 onward adds 2000.

---

## 2. Variant rules (to freeze before coding)

All variants use one 52-card deck, blinds, and the usual betting rounds: pre-flop → flop (3) → turn (1) → river (1), unless stated otherwise.

### 2.1 Texas Hold'em
2 hole cards. Best 5 of 7, using any mix of hole and board cards.

### 2.2 Omaha
4 hole cards. Must use **exactly 2 hole + 3 board**.

### 2.3 Dacoit (house)
- **Choice phase, before dealing:** starting left of the dealer, each non-dealer player picks **4 or 6 cards** in turn. Everyone sees earlier choices. The **dealer always gets 4**.
- **Betting multiplier:** a 6-card player pays **2×** to match a 4-card player. The engine tracks bets in *units*: a 4-card player pays 1 chip per unit, a 6-card player pays 2 chips per unit. If a 4-card player bets 100, a 6-card player calls with 200. If a 6-card player raises by 200 chips, that is 100 units, so a 4-card player calls 100 more.
- Blinds follow the same multiplier.
- All-ins and side pots are computed in units. A 6-card player with 500 chips is all-in for 250 units.
- Showdown: **exactly 2 hole + 3 board**, same as Omaha.

### 2.4 Double Texas (house)
- 4 hole cards each. **Two separate 5-card boards**, dealt from the same deck and revealed street by street together.
- **One shared pot, one betting line** for both boards.
- **Allocation phase after the river:** every player still in the hand splits their 4 cards into 2+2, one pair per board. There is a timer; on timeout the server picks the strongest split.
- Each board is judged like Texas Hold'em: best 5 of that pair + that board.
- **Scoring:** each board awards 1 point to its winner. If players tie on a board, **each of them gets a full point** (the point is not divided). The player with the most points wins the whole pot. Players tied on points split it equally.
  - A wins board 1 and B wins board 2 → 1–1 → A and B split 50/50.
  - A and B tie on board 1, A wins board 2 → A 2, B 1 → A takes the whole pot.

### 2.5 Triple Texas (house)
Same as Double Texas, with **6 hole cards** and **3 boards**. Allocation is 2+2+2, with the same scoring rules (most points wins, normally 2 of 3). Example: A and B tie on board 1, C wins board 2, D wins board 3 → everyone has 1 point → all four split the whole pot equally. **Only offered when 6 or fewer players are in the hand** (single deck).

### 2.6 Show 2 Play 2 (house)
- 4 hole cards each.
- **Show phase, before pre-flop betting:** in seat order, each player picks 2 of their cards and turns them face up for everyone.
- Showdown: **exactly 3 board + 1 shown card + 1 hidden card**.

### 2.7 Card-count limits (deck is 52)

| Variant | Cards used | Max players |
|---|---|---|
| Hold'em | 2n + 5 | 23 (cap at room max) |
| Omaha / S2P2 | 4n + 5 | 11 |
| Dacoit | 4·(n4) + 6·(n6) + 5 | Choice phase blocks "6" once the deck would run out |
| Double Texas | 4n + 10 | 10 |
| Triple Texas | 6n + 15 | **6** (hard rule) |

Variants that don't fit the current number of seated players are greyed out in the dealer's picker.

### 2.8 Rule decisions (confirmed)
1. **Dacoit:** players choose 4 or 6 **before** seeing any cards. Blinds **double** for 6-card players.
2. **Double/Triple Texas:** each board awards 1 point. A tie on a board gives **each tied player a full point**. Most points wins the whole pot, and players tied on points split it equally (§2.4).
3. **Triple Texas:** single deck only, **max 6 players**. Two decks may come later.
4. **Loans:** no repayment. Final score = profit = stack − starting stack − debt. Repayment may come later (§7).
5. **Show 2 Play 2:** cards are revealed **in seat order**, starting left of the dealer.

---

## 3. Architecture

```
 Browser (React + TS)                    Go server (single binary)
 ┌──────────────────┐   HTTPS (REST)    ┌──────────────────────────────────┐
 │ Lobby / Table UI │ ───────────────▶  │ HTTP API: create/join room       │
 │ Zustand store    │                   │ Static files (embed.FS)          │
 │                  │ ◀─── WebSocket ─▶ │ WS gateway ─▶ Room actor (1 per  │
 └──────────────────┘   JSON messages   │   room, goroutine + channel)     │
                                        │      └─▶ Game engine (pure, no IO)│
                                        │ Snapshot store (Redis, optional) │
                                        └──────────────────────────────────┘
```

### Key decisions
- **The server is authoritative.** The client only sends *intents* (`bet 200`, `allocate [...]`). The server validates every action and sends each player a **filtered view**: you never receive another player's hidden cards.
- **One actor per room.** A single goroutine owns all of a room's state and reads commands from a channel. Room state needs no mutexes, and races are ruled out by design.
- **Pure engine.** `engine` is plain Go with no network code or clock: `state' , events = Apply(state, action)`. The RNG is injected and seedable. This is what makes the engine easy to test and replay.
- **Identity:** no accounts. On join, the server issues a random `playerToken`, stored in `localStorage`, which allows reconnecting to the same seat.
- **Shuffling:** `crypto/rand` in production, a seeded `math/rand/v2` in tests.
- **Storage:** in memory first. Milestone 8 adds Redis snapshots so rooms survive a redeploy.

### Tech stack
- **Backend:** Go 1.23+, `net/http` + `github.com/coder/websocket`, `log/slog`, `testify`.
- **Frontend:** React 18 + TypeScript + Vite, Zustand, Tailwind, Framer Motion for card animations.
- **Shared protocol:** Go structs → JSON Schema/TS types generated with `tygo`, so client and server can't drift.
- **Deploy:** Docker multi-stage build → **Fly.io** (good WebSocket support, cheap, single region to start). GitHub Actions for CI/CD.

### Repo layout
```
server/
  cmd/server/             # HTTP + WebSocket server
  cmd/sim/                # bot stress test (make sim)
  cmd/replay/             # replay a hand log JSON
  internal/cards/        # Card, Deck, shuffle
  internal/eval/         # 5-card evaluator + "choose k from hand, m from board" search
  internal/engine/       # Hand state machine, betting, pots (unit-based), showdown, and every variant
  internal/room/         # Room actor, settings, seats, loans, dealer rotation
  internal/ws/           # WebSocket gateway, session tokens, rate limit
  internal/protocol/     # Message types (source of truth for TS generation)
  internal/sim/          # Bots + simulation harness for tests
web/
  src/{pages,components,store,protocol}
e2e/                     # Playwright tests
deploy/                  # Dockerfile, fly.toml
```

### Variant abstraction
Each variant is a set of **phases** plus a **showdown rule**. The engine drives the phases.

```go
type Variant interface {
    ID() VariantID
    CanStart(numPlayers int) error
    Phases() []Phase          // e.g. Choice, Deal, Show, Bet, Board(3), Bet, Board(1), Bet, Board(1), Bet, Allocate, Showdown
    HoleCards(p *Player) int  // 2, 4, or 4/6 (Dacoit)
    Boards() int              // 1, 2, or 3
    Multiplier(p *Player) int // 1, or 2 for Dacoit 6-card players
    BestHand(p *Player, boards [][]Card) []HandRank // one per board
    Score(results) []Payout   // single board or points-based
}

// Phase kinds: ChoicePhase, DealPhase, ShowPhase, BettingRound, RevealBoard, AllocatePhase, Showdown
```

The hand evaluator exposes one general search used by every variant:

```go
// Best 5-card hand using between minHole..maxHole hole cards and the rest from the board,
// with optional forced-card constraints (S2P2: 1 from shown, 1 from hidden).
func Best(hole, board []Card, rule HandRule) HandRank
```
- Hold'em: `{minHole:0, maxHole:2}`
- Omaha/Dacoit: `{exactHole:2}`
- S2P2: `{groups: [{shown,1},{hidden,1}], board:3}`
- Double/Triple: Hold'em rule per board using the allocated pair.

### WebSocket protocol (sketch)
Client → server: `join`, `sit`, `updateSettings` (host), `start` (host), `chooseVariant` (dealer), `chooseCardCount` (Dacoit), `showCards` (S2P2), `action {fold|check|call|bet|raise, amount}`, `allocate {boards:[[c,c],[c,c]]}`, `takeLoan`, `sitOut`, `chat`.

Server → client: `roomState` (full filtered snapshot on join/reconnect), `event` (incremental: dealt, acted, boardRevealed, potsUpdated, showdown, loanTaken…), `yourTurn {legalActions, minRaise, maxBet, deadline}`, `error`.

Every message carries `seq`. A client that misses one asks for a fresh snapshot.

---

## 4. Testing strategy (runs across every milestone)

| Layer | What | Tools |
|---|---|---|
| Evaluator unit | All 9 hand categories, kickers, wheel straight (A-2-3-4-5), ties | table-driven tests |
| Evaluator correctness | Compare against a slow brute-force reference on 1M random hands; check category frequencies over all C(52,5) hands | Go test, `-short` skips the long run |
| Variant rules | Omaha "exactly 2" traps (e.g. 4 hearts in hand + 1 on board is **not** a flush). S2P2 forced 1+1. Allocation scoring | table-driven tests |
| Betting/pots | Min-raise, short all-in not reopening action, side pots, Dacoit unit math, odd-chip split | table-driven + golden-file scenarios |
| Invariants (property) | **Total chips are conserved**, no negative stacks, every pot is paid out, no card dealt twice, a folded player never wins | `pgregory.net/rapid` |
| Simulation | Random/heuristic bots play 100k hands across all variants with random settings; invariants checked after every action | `internal/sim`, nightly CI job |
| Fuzz | Random and garbage client messages never panic the room actor or leak hidden cards | `go test -fuzz` |
| Concurrency | Every test runs with `-race`. Room actor stress test: 50 clients sending at once | `go test -race` |
| WS integration | `httptest.Server` + N real WebSocket clients play a scripted hand end to end. Check that hidden cards never appear in other players' payloads | Go test |
| Frontend unit | Store reducers, card-selection components (allocation, show-2, Dacoit choice) | Vitest + Testing Library |
| E2E | 3–4 browser contexts: create room → join via link → configure → play one hand of each variant → loan flow → reconnect | Playwright |
| Load | 200 rooms × 6 bots over WebSockets; target p95 action latency < 100 ms | k6 |
| Replay | Every hand logs `(seed, actions)`; a CLI replays it so bugs can be reproduced exactly | `cmd/replay` |

**CI (GitHub Actions) on every PR:** `go vet`, `staticcheck`, `go test -race ./...`, `golangci-lint`, `tsc --noEmit`, `vitest`, Playwright (headless), Docker build. **Nightly:** long simulation + fuzz job.

---

## 5. Milestones

Each milestone ends with a **demo** and a **green test gate**. Don't start the next one until the gate passes.

### M0 — Spec freeze & skeleton (≈2 days) ✅
- Write the final rules (§2, decisions in §2.8) into `docs/rules.md`, with one worked example per variant.
- Set up the monorepo, Go module, Vite app, Makefile, linters, and the GitHub Actions CI skeleton.
- **Gate:** CI is green on an empty project. Rules doc approved.

### M1 — Cards & hand evaluator (≈3 days) ✅
- `cards`: Card type, Deck, seeded shuffle.
- `eval`: fast 5-card ranker plus the `Best(hole, board, rule)` search covering all rule types.
- **Tests:** evaluator unit tests, brute-force comparison, category frequency check (e.g. 4 royal flushes, 40 straight flushes out of 2,598,960 hands), Omaha and S2P2 rule traps. A benchmark showing Best() under ~5 µs for Dacoit (150 combos).
- **Gate:** 100% of evaluator tests pass. Coverage ≥ 95% on `eval`.

### M2 — Headless engine + Texas Hold'em (≈1 week) ✅
- Hand state machine: blinds, action order (heads-up special case), legal actions, min-raise, all-in, side pots, showdown, odd chips, everyone-folds early win.
- Pots are computed in **units with per-player multipliers** from day one, so Dacoit needs no rework later.
- `sim` package with random bots, plus `cmd/replay`.
- **Tests:** golden scenarios (≥ 30 tricky betting situations), rapid property tests for chip conservation, 100k-hand simulation.
- **Gate:** a CLI plays a full Hold'em game between bots with zero invariant violations.

### M3 — Rooms, lobby, WebSocket, loans → *first playable* (≈1.5 weeks) ✅
- REST: `POST /api/rooms` and `GET /api/rooms/{code}`. WebSocket at `/ws/{code}`.
- Room actor: seats, host role (passed on if the host leaves), settings validation, start, dealer rotation, variant picker with timer, turn timers, sit-out, **loan flow + debt tracking**, leaderboard.
- Session tokens + reconnect with a full snapshot.
- Minimal React UI: home, lobby with settings form, plain table view (text cards are fine), action bar, leaderboard.
- **Tests:** WebSocket integration tests (scripted hands, hidden-card leak check, reconnect mid-hand), loan math tests (penalty applies to loans 1..N only; profits sum to minus total penalties), fuzz test on message decoding, first Playwright test (create → join → play one Hold'em hand).
- **Gate:** 3 people on different machines (via ngrok or a LAN IP) can play Hold'em with loans from start to finish.

### M4 — Omaha + Show 2 Play 2 (≈4 days) ✅
- Omaha: 4 cards, exact-2 rule, optional pot-limit betting.
- S2P2: show phase (pick 2 to reveal, in seat order, with timer and auto-pick on timeout), 3+1+1 showdown, and the shown cards included in every player's view.
- UI: 4-card hand, card picker for the show phase, shown cards displayed beside each opponent.
- **Tests:** variant table tests, pot-limit max-bet tests, simulation across both variants, Playwright for the S2P2 show flow.
- **Gate:** the dealer can switch between Hold'em, Omaha, and S2P2 hand by hand in a live room.

### M5 — Dacoit (≈4 days) ✅
- Choice phase in seat order (dealer fixed at 4) with the deck-capacity guard.
- 2× multiplier applied to blinds, calls, raises, all-ins, and side pots. The UI shows both the "units to call" and the "chips you pay".
- **Tests:** hand-calculated multiplier scenarios (6-card all-in vs 4-card big stack, mixed side pots), chip conservation under the multiplier, simulation.
- **Gate:** a mixed table of 4-card and 6-card players with all-ins settles correctly, and the result is checked by hand against the rules doc example.

### M6 — Double & Triple Texas (≈1 week) ✅
- Multi-board dealing and reveal, shared betting, allocation phase (drag cards onto each board; "auto-best split" is the timeout default), per-board judging, points-based pot split.
- UI: 2 or 3 boards stacked, an allocation tray, and a showdown that highlights who won which board.
- **Tests:** all splits enumerated (6 for Double, 15 for Triple), scoring tables (clean win, 1–1 split, tie on a board gives every tied player a full point, the 4-way A/B-C-D Triple example, a tie on one board + a win on another), timeout auto-allocation, Triple Texas refused with 7+ players.
- **Gate:** all 6 variants are playable in one room. A 100k-hand mixed-variant simulation is clean.

### M7 — UI/UX polish (≈1.5 weeks) ✅
- Oval table layout, chip stacks, dealer button, deal/flip/chip animations, sounds (toggleable), timer rings, hand-strength hint (optional setting), hand history panel, chat, emoji reactions.
- Mobile layout (portrait), dark mode.
- Accessibility: keyboard shortcuts for fold/check/raise, colour-blind 4-colour deck option.
- **Tests:** Vitest component tests, Playwright visual snapshots of key screens, manual cross-browser pass (Chrome, Safari iOS, Firefox).
- **Gate:** playtest session with 5+ friends. Fix everything ranked "confusing" or worse.
- **Done:** chat and emoji reactions, hand history (last 20 hands), timer rings, deal/bet/pot/reaction animations, sounds and vibration (toggleable, per device), four-colour deck, keyboard shortcuts, current-hand hints (host setting), phone layout with a bottom dock and a phone E2E project.
- **Deferred:** oval seat layout (the grid reads better on phones with up to 10 players) and screenshot-diff tests (baselines differ between macOS and CI Linux). The 5+ friends playtest is still to do.

### M8 — Hardening (≈1 week)
- Disconnect grace period (60 s, then auto-fold/sit-out), host transfer, room cleanup after inactivity, per-connection rate limits, message size limits, origin check.
- Optional Redis snapshots after each hand, so a deploy doesn't wipe active rooms.
- Structured logs, Prometheus metrics (rooms, connections, hands/min, action latency), `/healthz`.
- **Tests:** chaos tests (kill a connection mid-action, duplicate messages, out-of-order seq), k6 load test, restart-recovery test with Redis.
- **Gate:** load target met (§4). No panics in a 24 h soak run with bots.

### M9 — Deploy (≈3 days)
- Multi-stage Dockerfile: build the web app → embed it into the Go binary → `distroless` image (~20 MB).
- Fly.io: `fly.toml`, 1 shared-cpu machine to start, health checks, `fly secrets` for config, optional Upstash Redis. Custom domain + automatic TLS.
- GitHub Actions: on merge to `main` → test → build image → `flyctl deploy`. Use preview environments for PRs if wanted.
- Error tracking (Sentry for frontend + backend) and uptime monitoring.
- **Scaling note:** rooms live in memory, so stay on **one instance** at first. When that's outgrown, pin each room to a machine (room code → machine ID, routed with Fly's `fly-replay` header).
- **Tests:** a smoke Playwright test runs against production after each deploy.
- **Gate:** public URL live. Friends play a full session on production.

### Rough timeline
About **7–8 weeks** for one developer working part-time-to-full-time. M0–M3 (≈3 weeks) gives you a deployable Hold'em room early. Consider a quick **early deploy right after M3** to shake out real-network issues.

---

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Side-pot/multiplier bugs (Dacoit) | Unit-based pot model from M2, property tests, hand-checked golden scenarios |
| Leaking hidden cards | Per-player view projection in one place; an automated leak assertion in every integration test |
| House rules ambiguity | Decisions recorded in §2.8; worked examples in `docs/rules.md` used directly as test fixtures |
| Deck runs out (Triple Texas, Dacoit) | Capacity checks in `CanStart` and in the Dacoit choice phase |
| State lost on redeploy | Redis snapshots (M8), deploy during low traffic, graceful-shutdown drain |
| Slow/AFK players | Turn timers, auto-actions, host kick |

---

## 7. Nice-to-haves (after launch)
- Bots to fill empty seats, using the `sim` heuristics.
- Loan repayment during the game.
- Two-deck mode so Triple Texas works with more than 6 players.
- Room passwords / private rooms; public lobby list.
- Hand replays shareable by link.
- Optional accounts to keep lifetime stats.
- Tournament mode with rising blinds and elimination (loans disabled).
