# Poker Room: notes for Claude

Play-money multiplayer poker for friends: a host creates a room and shares a link; each hand's dealer picks the variant (Texas Hold'em, Omaha, Dacoit, Double Texas, Triple Texas, Show 2 Play 2, BB). The full rules, with worked examples, are in [docs/rules.md](docs/rules.md); that file is the source of truth for game behaviour.

Go backend (`server/`) and React + TypeScript frontend (`web/`), deployed as one small Fly.io machine. Setup and commands are in [README.md](README.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## Ground rules (check every change against these)

- **Phone first.** Most players are on phones. UI must work at 375px wide with no sideways scrolling, and tap targets must be comfortable (about 44px).
- **No gameplay timers.** No turn clocks, countdowns or auto-actions on a timer, by design. When the table waits on someone, the host can Skip them; any player taps Next hand. Short cosmetic fades (reactions, the raise banner) are fine, and so is housekeeping (idle rooms, the 12-hour room limit), but nothing in play may happen because time passed.
- **Hidden cards stay hidden.** Each player gets their own view (`room.View(viewer)`, `NarrateFor(viewer)`). Never put another player's hole cards, a Double/Triple Texas split before the showdown, or another player's session token in shared state or logs.
- **Chips are conserved.** Chips only enter through starting stacks and loans. Players' profits sum to minus the loan penalties paid. The engine checks invariants (`engine.CheckInvariants`); the fuzz and property tests rely on them.
- **Hands replay exactly.** A hand is its seed plus its actions (`engine.Log`). Restarts rebuild rooms by replaying logs, so engine changes must keep replay deterministic (no map iteration order, no wall-clock reads inside the engine).
- **The rules doc changes with the rules.** A change to how the game plays updates docs/rules.md in the same PR.

## Map

**server/internal**
- `cards`: cards, decks, seeds (ChaCha8).
- `eval`: hand evaluation; `BestOf` covers every variant's "use exactly N hole cards" rule.
- `engine`: one hand as a pure state machine. `hand.go` (betting, phases: Dacoit picking, S2P2 showing, multi-board allocating), `draw.go` (BB's card exchanges), `pots.go` (side pots in units, a per-player multiplier for Dacoit's 6-card players), `variant.go`, `events.go` (events and narration), `invariants.go`. No I/O, no goroutines.
- `room`: a room around the engine: lobby, settings, seats, dealer rotation, loans, leaderboard, host handoff, kick votes (`kick.go`), rematch (`rematch.go`), bet callouts (`callout.go`), per-viewer views (`view.go`), snapshots (`snapshot.go`).
  - `actor.go`: each room runs on one goroutine (an inbox of closures, no locks). `Manager` creates, finds, sweeps, saves and loads rooms.
- `history`: the hand archive: every finished hand of a room appended to `<HISTORY_DIR>/<ROOM>.jsonl`, and `Replay` to turn it into readable hands. Downloads (`GET /api/rooms/{code}/history`) are only for the room's players once the game has ended, since they show everyone's cards.
- `protocol`: the JSON messages. **These Go types are the source of truth**; `web/src/protocol/types.ts` is generated from them with `make protocol`, and CI fails if it's stale.
- `ws`: REST (`/api/rooms`) and the WebSocket (`/ws/{code}`), rate limits, pings.
- `httpapi`: `/healthz`, static files, cache headers.
- `sim`: random bots that play huge numbers of hands checking invariants.
- `bot`: the computer players hosts can add. `Decide` gets only a `View` of what that seat may see (never other players' cards). `room/bots.go` seats them and has them act whenever the game waits on one.

**server/cmd**
- `server`: the server binary.
- `smoke`: the end-to-end check run after every deploy.
- `sim`, `replay`: stress-test and debugging tools.
- `history`: exports a room's archive file as readable JSON, even after the room has closed.

**web/src**
- `useRoom.ts`: the WebSocket: reconnects, resumes by token, follows a rematch to the new room.
- `pages/RoomPage.tsx`: the room screen and its banners.
- `components/Table.tsx`: the table, seats and the dock (your cards and actions).
- Pickers: `ActionBar` (bet chips), `ShowPicker`, `ChoosePicker`, `SplitPicker`, `HandCards` (rearranging your cards).
- Pure logic outside components: `cardOrder.ts`, `callouts.ts`, `roundSteps.ts`.

## Tests

- **Go:** table-driven scenarios in `engine/scenarios_test.go` and `variants_test.go`; property tests with `rapid`; `room` tests use the `harness` in `room_test.go`; `FuzzRoom` throws random messages at a room. Run with `make test-server` (race detector on).
- **Web:** Vitest + Testing Library. Shared fixtures are in `web/src/test/fixtures.ts`. Put a new feature's tests in their own `*.test.tsx` file, so parallel PRs don't conflict.
- **End to end:** Playwright in `web/e2e`. Tests tagged `@phone` also run on a 375px phone profile. Put new specs in their own file, again to avoid conflicts.
- **Full set:** `make ci` (lint, all tests, build) and `make e2e`. CI runs Server, Web, Docker image and End-to-end, and all four must pass.

## Reviewing a pull request

Check, in this order:

1. **Correctness against docs/rules.md.** Dacoit's 2× multiplier applies to blinds, bets and pot-limit maths. Ties split, and odd chips go left of the dealer. Multi-board scoring is one point per board, and ties give a full point each.
2. **Hidden information.** Does any new field in `protocol.State` or `PlayerView` leak cards, splits or tokens to the wrong viewer?
3. **Ground rules above:** no timers, chips conserved, deterministic replay, phone layout.
4. **Protocol changes:** `make protocol` was run and `types.ts` is committed; new fields are optional where older clients or snapshots may lack them.
5. **Snapshots:** new room state that must survive a restart is saved and restored in `snapshot.go`, with a round-trip test. Adding an optional field is fine. A change that old snapshots can't load needs `snapshotVersion` bumped.
6. **Tests** cover the behaviour (Go for rules and room logic, Vitest for UI, Playwright for flows), including the edge cases: all-ins, side pots, folds, Dacoit 6-card players, players offline, kicked players.
7. **Style:** matches the surrounding code; gofmt, golangci-lint and oxlint are clean.

## Workflow

- Changes reach `main` only through PRs: all four CI checks must pass, and the owner (code owner) must approve. PRs are squash-merged.
- Merging doesn't deploy. A GitHub release (`v*` tag, admins only) deploys that version; **Actions → Deploy → Run workflow** deploys the latest `main`.
