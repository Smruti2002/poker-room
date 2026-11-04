# Poker Room

Play-money multiplayer poker with friends. Create a room, share the link, and take turns as dealer picking the variant: Texas Hold'em, Omaha, Dacoit, Double Texas, Triple Texas, or Show 2 Play 2.

- **Rules:** [docs/rules.md](docs/rules.md)
- **Roadmap & milestones:** [PLAN.md](PLAN.md)

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

## Configuration (server)

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `STATIC_DIR` | *(empty)* | Serve the built web app from this directory |
| `LOG_LEVEL` | `info` | Set to `debug` to log every request |
| `SNAPSHOT_PATH` | *(empty)* | File to save rooms to. Rooms are loaded at start, saved every 30s and on shutdown, so a restart or redeploy keeps games going. Empty disables saving. |
| `ROOM_IDLE` | `12h` | Close rooms nobody has been connected to for this long. |
| `ROOMS_PER_MINUTE` | `10` | New rooms allowed per IP address per minute. |
| `TRUST_PROXY` | *(empty)* | Set to `1` behind Fly.io so the client IP comes from `Fly-Client-IP`. |
| `ALLOWED_ORIGINS` | *(empty)* | Extra browser origins allowed to open WebSockets, comma-separated host patterns (e.g. `localhost:5173`). Same-origin is always allowed. |
