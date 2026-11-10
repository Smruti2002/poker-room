# Poker Room

Play-money multiplayer poker with friends. Create a room, share the link, and take turns as dealer picking the variant: Texas Hold'em, Omaha, Dacoit, Double Texas, Triple Texas, Show 2 Play 2, or BB.

- **Rules:** [docs/rules.md](docs/rules.md)
- **Roadmap & milestones:** [PLAN.md](PLAN.md)
- **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)

## Layout

```
server/   Go backend (HTTP + WebSocket, game engine)
web/      React + TypeScript + Vite frontend
docs/     Rules and design notes
```

## Prerequisites

- Go 1.27+
- Node 26 (see `web/.nvmrc`)
- Optional: `golangci-lint` (`brew install golangci-lint`)

## Development

```bash
make install
```

Run the server and the web app in two terminals:

```bash
make dev-server
```

```bash
make dev-web
```

Open http://localhost:5173. Vite proxies `/api`, `/ws` and `/healthz` to the Go server on `:8080`.

To play on your LAN, build once and serve everything from the Go server, then share `http://<your-ip>:8080`:

```bash
make run
```

## Checks

```bash
make ci
```

This runs the linters, all tests (Go with `-race`) and a production build. `make help` lists the other targets.

End-to-end tests drive real browsers against the built server (first run: `cd web && npx playwright install chromium`):

```bash
make e2e
```

### Engine stress test

```bash
make sim
```

Bots play 100,000 hands with invariants checked after every action. If one breaks, the hand log is written to `server/sim-failure.json`; replay it with:

```bash
cd server && go run ./cmd/replay sim-failure.json
```

To check a running server end to end (health, page, a room with two players, game start):

```bash
cd server && go run ./cmd/smoke -url http://localhost:8080
```

## Deploying (Fly.io)

The app runs as one small machine that stops when nobody is connected and starts again on the next visit. Rooms are saved to a volume, so they survive both. Config is in [fly.toml](fly.toml).

First time (install `flyctl` with `brew install flyctl`):

```bash
fly auth login
```

```bash
fly apps create poker-room-smruti
```

```bash
fly volumes create rooms --region sin --size 1 --app poker-room-smruti
```

```bash
fly deploy --ha=false
```

If the app name is taken, choose another and change it in `fly.toml` and `.github/workflows/deploy.yml`.

After that, deploys are manual: merging to `main` never deploys on its own, since each deploy restarts the server and briefly disconnects anyone playing.

**Releasing (the usual way).** Publish a GitHub release with a version tag. That exact commit is deployed, the site reports the tag as its version (`/healthz`), and the release notes list the PRs merged since the last one:

```bash
gh release create v0.2.0 --target main --generate-notes
```

Use the next version number each time: bump the last number for fixes (`v0.2.1`) and the middle one for new features (`v0.3.0`). To roll back, open **Actions → Deploy**, find the run for the release you want back, and choose **Re-run all jobs**.

**Deploying the latest `main` without a release:** open **Actions → Deploy → Run workflow**, or:

```bash
gh workflow run deploy.yml
```

Either way it refuses to deploy a commit CI hasn't passed on, and smoke-tests the live site afterwards. The repository's **Deployments → production** shows what's live and links to the site.

The Fly token is a secret of the `production` environment, which only `main` and `v*` tags may use; only admins can create `v*` tags. Create it once with:

```bash
fly tokens create deploy --app poker-room-smruti | gh secret set FLY_API_TOKEN --env production
```

To stop everything between game nights, and start it again:

```bash
fly scale count 0 --app poker-room-smruti
```

```bash
fly scale count 1 --app poker-room-smruti
```

## Hand histories

With `HISTORY_DIR` set (it is on Fly), every finished hand is archived. When a game ends, any of its players can tap **Download hand history** on the final standings to get every hand as JSON: each player's cards, every action and bet, the boards, and who won what.

The archives outlive their rooms (they're kept 30 days), so a game can be exported later from the server:

```bash
fly ssh sftp get /data/history/ROOMCODE.jsonl ROOMCODE.jsonl --app poker-room-smruti
```

```bash
cd server && go run ./cmd/history ../ROOMCODE.jsonl > ../ROOMCODE.json
```

## Configuration (server)

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `STATIC_DIR` | *(empty)* | Serve the built web app from this directory |
| `LOG_LEVEL` | `info` | Set to `debug` to log every request |
| `SNAPSHOT_PATH` | *(empty)* | File to save rooms to. Rooms are loaded at start, saved every 30s and on shutdown, so a restart or redeploy keeps games going. Empty disables saving. |
| `ROOM_IDLE` | `12h` | Close rooms nobody has been connected to for this long. |
| `ROOM_MAX_AGE` | `12h` | Close every room this long after it was created, even with people in it (hosts forget to end games). Players see the closing time during the last hour. |
| `HISTORY_DIR` | *(empty)* | Folder to archive every finished hand in, one file per room. Players can then download a whole game from the final standings, and archives outlive their rooms. Empty disables it. |
| `HISTORY_KEEP` | `720h` | Delete hand archives this long after their last hand (30 days). |
| `ROOMS_PER_MINUTE` | `10` | New rooms allowed per IP address per minute. |
| `TRUST_PROXY` | *(empty)* | Set to `1` behind Fly.io so the client IP comes from `Fly-Client-IP`. |
| `ALLOWED_ORIGINS` | *(empty)* | Extra browser origins allowed to open WebSockets, comma-separated host patterns (e.g. `localhost:5173`). Same-origin is always allowed. |
