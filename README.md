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
- `cmd/` Go entrypoints (`aimmod-hub`)
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
- a Go API server in `cmd/aimmod-hub` (with shared runtime under `api/`)
- a Connect RPC service definition in `proto/aimmod/hub/v1/hub.proto`
- Vite frontend wired to Connect RPC
- Discord-backed website auth
- desktop device linking for the AimMod app
- admin-only dashboard support
- versioned ingest endpoint shape for authenticated app uploads

## Development

### Generate protobuf code

```bash
pnpm proto:generate
```

### Run the API

```bash
pnpm dev:db:up
go run ./cmd/aimmod-hub
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
- the Go API
- the Vite frontend

## Deployment

The root `Dockerfile` builds both the frontend and the API into a single image. The Go server serves the SPA with per-route server-side meta tag injection (`og:*`, `twitter:*`, `<title>`) so social media link previews and crawlers see real content without a separate renderer.

```
[browser] → Go API → static asset or meta-injected index.html
           ↕
        Postgres
```

Build and run:

```bash
docker build -t aimmod-hub .
docker run -p 8080:8080 \
  -e DATABASE_URL=postgres://... \
  -e DISCORD_CLIENT_ID=... \
  -e DISCORD_CLIENT_SECRET=... \
  -e DISCORD_REDIRECT_URI=https://aimmod.app/auth/discord/callback \
  -e AIMMOD_HUB_WEB_ORIGIN=https://aimmod.app \
  -e SESSION_COOKIE_SECURE=true \
  aimmod-hub
```

The `AIMMOD_HUB_API_BASE_URL` env var controls which API URL the frontend uses at runtime (written into `runtime-config.js` on container start). When the frontend and API are on the same origin this can be left unset — it defaults to `https://api.aimmod.app` which you can override:

```bash
-e AIMMOD_HUB_API_BASE_URL=https://aimmod.app
```

### Split deployment (API + Nginx, no SSR meta)

The legacy `web/Dockerfile` builds a standalone Nginx image serving the SPA. Page titles and OG tags update client-side after the SPA boots — fine for Google, not for social previews.

```bash
docker build -f web/Dockerfile -t aimmod-hub-web .
docker run -p 8080:8080 \
  -e AIMMOD_HUB_API_BASE_URL=https://api.aimmod.app \
  aimmod-hub-web
```

The API runs as a separate container pointed at the same database.

## Railpack deployment

`railpack.json` at the repo root configures the full single-server build on Railway or any Railpack-compatible platform.

Railpack auto-detects Go as the primary runtime and builds `cmd/aimmod-hub`. The config adds two extra steps:

- **install-web** — runs `pnpm install` for the frontend
- **build-web** — runs `pnpm build:web` and includes `web/dist` in the deploy output

`AIMMOD_HUB_STATIC_DIR=/app/web/dist` is set as a deploy variable so the Go server serves the built frontend with SSR meta injection automatically.

Set `AIMMOD_HUB_API_BASE_URL` to your public URL in your Railway service variables so the frontend runtime config points at the right API endpoint. All other required env vars (`DATABASE_URL`, `DISCORD_*`, etc.) are set via the platform's environment configuration.

At runtime the server prefers `AIMMOD_HUB_ADDR`, then `PORT`, then `:8080`.

## Environment

See [`.env.example`](./.env.example).

Frontend uses:
- `VITE_API_BASE_URL`
- `AIMMOD_HUB_API_BASE_URL` for Docker/runtime injection

Notes:
- `VITE_*` values are compiled into the Vite bundle at build time.
- For runtime-only env injection, set `window.__AIMMOD_HUB__.apiBaseUrl` via `web/public/runtime-config.js` (served as `/runtime-config.js`).
- The Docker web image writes `/runtime-config.js` on container startup from `AIMMOD_HUB_API_BASE_URL`, falling back to `VITE_API_BASE_URL`, then `https://api.aimmod.app`.
- If neither is provided, the frontend defaults to `https://api.aimmod.app`.

API uses:
- `DATABASE_URL`
- `AIMMOD_HUB_ADDR`
- `AIMMOD_HUB_VERSION`
- `AIMMOD_HUB_WEB_ORIGIN` — allowed CORS origin for the web frontend
- `AIMMOD_HUB_STATIC_DIR` — (optional) path to built `web/dist`; enables Mode B single-server deployment with SSR meta injection
- `DISCORD_CLIENT_ID`
- `DISCORD_CLIENT_SECRET`
- `DISCORD_REDIRECT_URI`
- `AIMMOD_HUB_ADMIN_DISCORD_USER_ID`
- `SESSION_COOKIE_SECURE`

## Auth flow

- users sign in on the website with Discord
- the website creates a session cookie for account pages, device linking, and admin access
- the desktop app opens a device-link flow against the hub
- once approved in the browser, the desktop app receives its upload token and syncs runs automatically

This keeps browser OAuth on the website and avoids forcing a full Discord auth flow inside the desktop app.

## Principles

- the desktop app must remain useful without the website
- uploads should be opt-in
- public sharing should be explicit
- raw ingest and derived/public analytics should be separated
