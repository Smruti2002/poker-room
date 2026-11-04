# Contributing

Thanks for helping. This is a play-money poker game for friends. Most people play it on their phones, so every change has to work at a 375px-wide screen.

## Before you start

- For anything bigger than a small fix, open an issue first (or comment on an existing one) so we can agree on the approach.
- The rules of every variant are in [docs/rules.md](docs/rules.md). If your change affects how the game plays, update that file in the same PR.
- **No gameplay timers.** There are no turn clocks or countdowns, by design. If the table is waiting on someone, the host can skip them. Please don't add timers back.

## Set up

You need Go 1.27+ and Node 26 (see `web/.nvmrc`). Fork the repo, clone your fork, then:

```bash
make install
```

Run the server and the web app in two terminals, then open http://localhost:5173:

```bash
make dev-server
```

```bash
make dev-web
```

[README.md](README.md) has more: playing on your LAN, the engine stress test, and the server's settings.

## Making a change

1. Create a branch in your fork.
2. Add tests for what you change:
   - Game rules: engine tests in `server/internal/engine`.
   - Rooms, players, votes: room tests in `server/internal/room`.
   - UI: component tests in `web/src` (Vitest). For whole flows, add a Playwright test in `web/e2e`; tag it `@phone` so it also runs at phone size.
3. If you change the messages between browser and server (`server/internal/protocol`), regenerate the TypeScript types and commit them:

   ```bash
   make protocol
   ```

4. Check UI changes at 375px wide. Nothing may scroll sideways.
5. Run what CI runs: lint, every test (Go with the race detector) and a production build, then the end-to-end tests. The first time, install the browser for the end-to-end tests with `cd web && npx playwright install chromium`.

   ```bash
   make ci
   ```

   ```bash
   make e2e
   ```

## Opening the pull request

- Open it against `main`, and say which issue it closes (`Closes #12`).
- Describe what changed for players and how you tested it. A screenshot at phone width helps for UI changes.
- CI runs on every PR: Server (Go), Web (React), Docker image and End-to-end. All four must pass. For a first contribution, a maintainer has to approve the CI run before it starts.
- Every change needs the maintainer's approval before it's merged, and PRs are squash-merged. Pushing new commits after an approval cancels it, so the PR needs a fresh review.
- Deploys are separate and manual: merging doesn't change the live site.

## Code style

- Go: `gofmt` and `golangci-lint` (run by `make lint`).
- Web: TypeScript and `oxlint` (also run by `make lint`).
- Match the code around you: its naming, comments and structure.
