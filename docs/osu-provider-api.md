# osu! provider API

AimMod Hub exposes one normalized, versioned Connect service for osu! beatmap discovery. Desktop and web clients call Hub; they do not open provider websites or receive provider credentials.

The contract is defined in `proto/aimmod/osu/v1/osu.proto`. Its RPC procedures are:

- `/aimmod.osu.v1.OsuService/GetProviderStatus`
- `/aimmod.osu.v1.OsuService/SearchBeatmapItems`
- `/aimmod.osu.v1.OsuService/GetBeatmapItem`
- `/aimmod.osu.v1.OsuService/GetDownloadHandoff`
- `/aimmod.osu.v1.OsuService/GetOfficialUserProfile`
- `/aimmod.osu.v1.OsuService/GetSkinProviderStatus`
- `/aimmod.osu.v1.OsuService/SearchSkins`
- `/aimmod.osu.v1.OsuService/GetSkin`
- `/aimmod.osu.v1.OsuService/GetSkinDownloadHandoff`

Search pagination is provider-specific. Clients must return each `ProviderCursor` unchanged on the next request. A provider failure is reported in `ProviderStatus`; successful results from another requested provider are still returned.

## Official osu! provider

The official adapter uses OAuth 2.0 client credentials with the `public` scope. It calls the documented [beatmapset search](https://osu.ppy.sh/docs/index.html#get-beatmapsets-search) and [beatmapset detail](https://osu.ppy.sh/docs/index.html#get-a-beatmapset) endpoints.

Configure these server-only values:

```dotenv
AIMMOD_OSU_CLIENT_ID=
AIMMOD_OSU_CLIENT_SECRET=
```

Do not expose the client secret to a desktop or browser bundle. AimMod Hub does not proxy the official API beatmapset download endpoint because that endpoint requires the reserved `lazer` OAuth scope. Instead, download handoff returns `osu://dl/{beatmapset_id}`. The desktop passes that URI to osu!lazer, which presents its own confirmation and performs the download.

Supported official search inputs include text, ruleset, status, sort, and numeric ranges for stars, BPM, length, approach rate, circle size, and overall difficulty. The normalized response includes beatmapset metadata, difficulties, aggregate ranges, provider capability state, and the lazer handoff.

`GetOfficialUserProfile` calls the documented `GET /api/v2/users/{user}/{mode}` endpoint. Callers must state whether `identifier` is an osu! user ID or username. Hub returns only the current official response, including official avatar and cover URLs, country, team, and mode statistics. Missing OAuth credentials or an upstream failure returns an error. Hub does not synthesize profile data or placeholder assets.

## osu!Collector provider

The osu!Collector adapter uses the site's current public, read-only API:

- `GET https://osucollector.com/api/collections/search`
- `GET https://osucollector.com/api/collections/{id}`
- `GET https://osucollector.com/api/collections/{id}/beatmapsv2`
- `GET https://osucollector.com/api/collections/recent` for availability checks

These endpoints are operated by osu!Collector but are not formally documented. Provider status therefore reports `contract_is_documented=false`. The adapter is deliberately read-only and isolated so contract changes do not affect the official provider.

Collection search supports text and cursor pagination. Ruleset, star, and BPM ranges are applied locally to the aggregate collection metadata. When a requested filter cannot be represented safely by osu!Collector's collection response, that provider's results are omitted and the limitation is returned in `ProviderStatus.message`.

No verified bulk collection download contract is available. A collection detail contains individual beatmapset IDs; selecting one produces the same lazer `osu://dl/{beatmapset_id}` handoff.

## Resource bounds

Hub applies an independent outbound request limiter to each provider and a shared bounded in-memory response cache. Only successful responses up to 8 MiB are cached. Defaults are:

```dotenv
AIMMOD_OSU_CACHE_TTL=5m
AIMMOD_OSU_CACHE_MAX_ENTRIES=256
AIMMOD_OSU_PROVIDER_RPS=4
AIMMOD_OSU_REQUEST_TIMEOUT=10s
```

Invalid tuning values fall back to these defaults. Cache entries are copied on read and write, expired lazily, and the oldest entry is evicted at capacity.

## Skin providers

Skin RPCs use their own `SkinProvider` enum. Search pagination remains provider-specific. A skin provider failure appears in `SkinProviderStatus` while another requested provider may still return results.

### osuskins.net

The osuskins.net adapter reads the site's public server-rendered catalog. It uses the site's own query parameters for keyword, mode, sort, direction, and page. Creator and player names resolve through the site's public `/load-more-filters` directory endpoint before Hub submits the catalog filter.

Detail parsing uses the page's Article JSON-LD and screenshot markup. Returned thumbnails and screenshots are the real `cdn.osuskins.net` asset URLs. Hub does not generate substitute images.

Catalog cards abbreviate view and download counts, and detail pages round file sizes to decimal megabytes. Hub sets `counts_are_approximate` or `file_size_is_approximate` when it converts those display values. `sensitive` remains absent when the provider supplies no trustworthy sensitivity marker.

The download form posts to `/skin/{id}/download`, then the site's client requires Cloudflare Turnstile verification through `/verify-turnstile`. Hub never submits or bypasses that challenge. `GetSkinDownloadHandoff` therefore returns `available=false`, an empty URI, and `requires_interactive_verification=true` for this provider.

The site does not publish a documented API. Provider status reports `contract_is_documented=false`, and the short cache and request limiter also apply to its HTML and directory responses.

### skins.osuck.net

Direct server requests to skins.osuck.net currently receive a Cloudflare 403 response. Hub reports the provider unavailable and returns no search, detail, screenshot, or download data. It does not replay browser cookies, solve challenges, or construct download URLs from unverified client code. This provider can become active after its owner exposes a server-readable contract or a complete live request and file response can be verified without bypassing Cloudflare.
