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

## Checks

```bash
make ci
```

This runs the linters, all tests (Go with `-race`) and a production build. `make help` lists the other targets.

## Configuration (server)

| Env var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `STATIC_DIR` | *(empty)* | Serve the built web app from this directory |
| `LOG_LEVEL` | `info` | Set to `debug` to log every request |
