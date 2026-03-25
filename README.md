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
- updateable coaching knowledge packs that local AimMod features can query
- the stable `/llm/manifest.json` entrypoint for local-coach runtime and model downloads

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

### Build the coaching pack

```bash
pnpm build:coachpack
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

### Serving the local coach manifest

The recommended production setup is:
- AimMod Hub serves `https://aimmod.app/llm/manifest.json`
- the manifest points at upstream-hosted assets on official sources like GitHub Releases or Hugging Face
- the heavy runtime/model files do not need to live on your server

Example:

```bash
docker run -p 8080:8080 \
  -e DATABASE_URL=postgres://... \
  -e AIMMOD_HUB_WEB_ORIGIN=https://aimmod.app \
  -e AIMMOD_HUB_LLM_MANIFEST_VERSION=2026-03-25.v1 \
  -e AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_URL=https://github.com/ggml-org/llama.cpp/releases/download/b5694/llama-b5694-bin-win-cpu-x64.zip \
  -e AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_SHA256=<sha256> \
  -e AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_ARCHIVE_TYPE=zip \
  -e AIMMOD_HUB_LLM_MODEL_URL=https://huggingface.co/ggml-org/gemma-3-1b-it-GGUF/resolve/main/gemma-3-1b-it-Q4_K_M.gguf \
  -e AIMMOD_HUB_LLM_MODEL_SHA256=<sha256> \
  -e AIMMOD_HUB_LLM_MODEL_FILENAME=aimmod-coach.gguf \
  aimmod-hub
```

With that configured, the desktop app downloads:
- `https://aimmod.app/llm/manifest.json` from AimMod Hub
- runtime ZIPs from the upstream `llama.cpp` release URLs in the manifest
- the model from the upstream Hugging Face URL in the manifest

If you ever do want Hub to serve local files directly, `AIMMOD_HUB_LLM_DIR` still mounts a directory under `/llm/` before the SPA fallback.

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
- `AIMMOD_HUB_API_BASE_URL` — public base URL used in runtime config and replay media URLs
- `AIMMOD_HUB_STATIC_DIR` — (optional) path to built `web/dist`; enables Mode B single-server deployment with SSR meta injection
- `AIMMOD_HUB_LLM_DIR` — (optional) path mounted at `/llm/` for directly served local-coach assets
- `AIMMOD_HUB_LLM_MANIFEST_VERSION` — enables generated `/llm/manifest.json` output
- `AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_URL`
- `AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_SHA256`
- `AIMMOD_HUB_LLM_RUNTIME_WINDOWS_X64_ARCHIVE_TYPE`
- `AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_URL`
- `AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_SHA256`
- `AIMMOD_HUB_LLM_RUNTIME_WINDOWS_ARM64_ARCHIVE_TYPE`
- `AIMMOD_HUB_LLM_MODEL_URL`
- `AIMMOD_HUB_LLM_MODEL_SHA256`
- `AIMMOD_HUB_LLM_MODEL_FILENAME`
- `DISCORD_CLIENT_ID`
- `DISCORD_CLIENT_SECRET`
- `DISCORD_REDIRECT_URI`
- `AIMMOD_HUB_ADMIN_DISCORD_USER_ID`
- `SESSION_COOKIE_SECURE`
- `AIMMOD_HUB_MEDIA_BACKEND` — `local` or `s3`
- `AIMMOD_HUB_MEDIA_DIR` — media storage root for the local backend
- `AIMMOD_HUB_S3_BUCKET`
- `AIMMOD_HUB_S3_REGION`
- `AIMMOD_HUB_S3_ENDPOINT`
- `AIMMOD_HUB_S3_ACCESS_KEY_ID`
- `AIMMOD_HUB_S3_SECRET_ACCESS_KEY`
- `AIMMOD_HUB_S3_FORCE_PATH_STYLE`

## Replay media storage

Replay video uploads stay out of Postgres. The hub stores only replay metadata in the database and writes the actual MP4 to a media backend:

- `local` for development and simple single-host installs
- `s3` for S3-compatible object storage like Cloudflare R2, MinIO, Backblaze B2 S3, or AWS S3

Example S3-compatible configuration:

```bash
AIMMOD_HUB_MEDIA_BACKEND=s3
AIMMOD_HUB_S3_BUCKET=aimmod-replays
AIMMOD_HUB_S3_REGION=auto
AIMMOD_HUB_S3_ENDPOINT=https://<your-s3-endpoint>
AIMMOD_HUB_S3_ACCESS_KEY_ID=...
AIMMOD_HUB_S3_SECRET_ACCESS_KEY=...
AIMMOD_HUB_S3_FORCE_PATH_STYLE=true
```

AimMod still uploads normal summary/timeline/feature data separately. Replay video uploads are opt-in from the desktop app and can stay limited to favorites, PBs, or higher-quality future tiers.

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

## Coaching knowledge API

AimMod Hub now also exposes a lightweight JSON coaching layer intended for local tools and future local-LLM orchestration:

- `GET /api/coaching/manifest`
- `POST /api/coaching/query`

The intent is:

- keep session analytics and flaw detection local in the desktop app
- let the hub serve updateable structured coaching knowledge
- let a local model use that knowledge without forcing desktop app updates for every coaching-data tweak

Normalized coaching authoring lives under `api/internal/coaching/content/`, with more detail in [docs/coaching-knowledge.md](./docs/coaching-knowledge.md).
