# AimMod Hub

AimMod Hub is the separate website and API for the AimMod ecosystem.

This repo is intentionally independent from the desktop app repo so:
- website deploys do not trigger Tauri or updater workflows
- app releases do not drag the website along with them
- accounts, uploads, public profiles, and community analytics can evolve on their own cadence

## Stack

- API: Go + Connect RPC + protobuf
- Frontend: Vite + React
- Contracts: protobuf + `buf`

## Repo layout

- `api/` Go API server
- `proto/` protobuf contracts
- `gen/` generated Go code
- `web/` Vite frontend
- `docs/` architecture notes

## Why this split

The desktop app repo should stay focused on:
- Tauri desktop app
- UE4SS runtime and mod bridge
- Windows release packaging
- updater metadata

AimMod Hub should own:
- accounts
- public profiles
- uploaded run history
- scenario and replay sharing
- community-wide analytics
- APIs that help AimMod learn from aggregate data

## Current scaffold

This scaffold includes:
- a Go API server in `api/cmd/aimmod-hub`
- a Connect RPC service definition in `proto/aimmod/hub/v1/hub.proto`
- Vite frontend wired to Connect RPC
- Discord-backed website auth
- upload token creation for the desktop app
- versioned ingest endpoint shape for authenticated app uploads

## Development

### Generate protobuf code

```bash
pnpm proto:generate
```

### Run the API

```bash
pnpm dev:db:up
go run ./api/cmd/aimmod-hub
```

### Run the frontend

```bash
pnpm install
pnpm dev:web
```

### Run everything

```bash
pnpm install
pnpm dev
```

This starts:
- local Postgres via Docker Compose
- protobuf codegen
- the Go API
- the Vite frontend

## Railpack deployment (API)

This repo is a mixed Go + Node monorepo, so auto-detection can choose the wrong runtime.

- `railpack.json` forces Go provider detection
- runtime start command is `./out` (Railpack Go default output binary)
- set `RAILPACK_GO_WORKSPACE_MODULE=api/cmd/aimmod-hub` in your deploy environment so Railpack builds the nested API entrypoint

At runtime, the API now prefers `AIMMOD_HUB_ADDR`, then falls back to `PORT`, then `:8080`.

## Environment

See [`.env.example`](./.env.example).

Frontend uses:
- `VITE_API_BASE_URL`

API uses:
- `DATABASE_URL`
- `AIMMOD_HUB_ADDR`
- `AIMMOD_HUB_VERSION`
- `AIMMOD_HUB_WEB_ORIGIN`
- `DISCORD_CLIENT_ID`
- `DISCORD_CLIENT_SECRET`
- `DISCORD_REDIRECT_URI`
- `SESSION_COOKIE_SECURE`

## Auth flow

- users sign in on the website with Discord
- the website creates a session cookie for account pages and token management
- users create a personal upload token on `/account`
- the desktop app stores that upload token and uses it for authenticated run ingest

This keeps browser OAuth on the website and avoids forcing a full Discord auth flow inside the desktop app.

## Principles

- the desktop app must remain useful without the website
- uploads should be opt-in
- public sharing should be explicit
- raw ingest and derived/public analytics should be separated
