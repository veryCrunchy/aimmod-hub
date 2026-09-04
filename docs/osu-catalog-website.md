# Website osu! Catalogs

The website calls the existing `aimmod.osu.v1.OsuService` Connect API for catalog searches and details. Shared community plays are a separate dataset and are not a substitute for the beatmap catalog.

## Server Setup

Set `AIMMOD_OSU_CLIENT_ID` and `AIMMOD_OSU_CLIENT_SECRET` in the API hosting environment, then restart or redeploy the service. The server exchanges these credentials for a public-scope OAuth token. Never use `VITE_` variables for credentials.

Verify `SearchBeatmapItems` returns an available official provider and catalog items. Missing credentials produce an unavailable provider response, not an empty successful search. Skin search uses the public catalog and does not need osu! OAuth credentials.

Provider responses use the existing bounded server cache and per-provider request limits. Search cursors are opaque: clients must return them unchanged with the same filters.

## App Links

The native osu! companion has a dedicated URI scheme, separate from the KovaaK's client:

| Action | URI |
| --- | --- |
| Review a beatmapset in AimMod | `aimmod-osu://beatmapsets/123` |
| Review an osuskins.net skin in AimMod | `aimmod-osu://skins/osuskins/3sXe0RR` |
| Review an osuck skin in AimMod | `aimmod-osu://skins/osuck/123` |
| Download a beatmapset in osu! | `osu://dl/123` |

AimMod links select an item for review; they do not authorize downloads or installation. The operating system chooses the registered application. The native app must include the URI handler; earlier builds cannot open these links. The website must keep its app download page accessible alongside the handoff.

Use validated positive decimal beatmapset IDs and provider-specific skin IDs. Do not insert untrusted provider URLs into an app link or derive executable commands from URL contents.

## Skin Downloads

Catalog screenshots can be previewed on the website. osuskins.net currently requires interactive verification for downloads; direct import must not be advertised when the provider supplies no usable download handoff. Open the source page for that step. Likewise, show a source-site action when osuck blocks catalog access.

## Verification

Run `pnpm typecheck:web`, `pnpm test:web`, `pnpm build:web`, and `go test ./api/internal/osu/...`. Verify populated, empty, unavailable, and detail states at desktop and mobile sizes. Test successive searches and pagination to ensure older responses cannot overwrite the current filters. Audio previews should stop when the selected item or page changes.
