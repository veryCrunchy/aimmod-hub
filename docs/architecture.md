# AimMod Hub Architecture

## Boundary

This repo owns:
- public web surfaces
- authentication and profiles
- upload APIs
- shareable run pages
- community analytics
- derived data used to improve AimMod

This repo does not own:
- Tauri desktop packaging
- UE4SS payload staging
- Windows updater releases
- app release workflows

## Stack choice

### API
- Go
- Connect RPC
- protobuf contracts managed by `buf`

This gives us:
- strong versioned contracts
- good ingest ergonomics for the desktop app
- generated clients for web and future services
- clean separation between transport and product logic

### Frontend
- Vite + React

This is a better fit here than a heavier full-stack frontend because:
- the web app should stay independently deployable
- we already have React experience and UI code patterns in AimMod
- the API is separate anyway, so a thin frontend is enough

## Data flow

1. AimMod desktop records local sessions.
2. User opts into sync.
3. AimMod uploads versioned protobuf payloads to AimMod Hub.
4. Hub stores raw ingest records.
5. Background jobs derive public and coaching-ready views.
6. Website reads derived/public views, not raw ingest directly.

## Suggested first database domains

- `users`
- `linked_accounts`
- `upload_batches`
- `scenario_runs`
- `scenario_catalog`
- `profiles_public`
- `shared_runs`
- `scenario_aggregates`
- `coaching_feature_sets`

## First RPC surface

- `HubService.GetHealth`
- `HubService.IngestSession`

Later:
- `ProfileService.GetProfile`
- `ScenarioService.GetScenario`
- `RunService.GetRun`

## Non-goals for v1

- browser replay parity with desktop
- public write APIs for third parties
- real-time multiplayer
- large media processing pipelines
