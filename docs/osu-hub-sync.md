# AimMod osu! Hub sync v1

AimMod's native osu! client reuses the Hub device-link flow used by the KovaaK's client. The desktop app starts a link at `POST /auth/device/start`, opens the returned verification URL, polls `POST /auth/device/poll`, and stores the resulting revocable upload token. All write endpoints require that token as `Authorization: Bearer <token>`.

## Privacy model

- `private` is the default when visibility is omitted. Metadata, replay bytes, and analysis are only retained for the authenticated account and are not returned by public endpoints.
- `unlisted` records can be opened with their opaque share ID but are excluded from community and profile listings.
- `public` records are included in community and public profile listings.
- Replay-file upload is independent of visibility. `uploadFile` must be explicitly enabled by the client; score metadata and analysis can be shared without uploading the `.osr` file.
- Exact judgement analysis is a separate native-client opt-in and is omitted by default, even for a public score.
- Upload tokens are revocable from the existing Hub account page. Tokens are never accepted in query strings.
- A Hub account is authenticated, but the v1 osu! profile attached by the native client is client-reported. It must not be presented as an osu! OAuth verification badge.

## Score sync

`POST /api/osu/v1/sync`

Required headers:

```text
Authorization: Bearer <upload token>
Idempotency-Key: <clientUploadId>
Content-Type: application/json
```

The request is capped at 4 MiB and rejects unknown JSON fields. `schemaVersion` is currently `1`. The payload contains explicit objects for:

- `profile`: osu! user ID, username, country, avatar and current aggregate profile statistics.
- `beatmapSet`: stable local/online set key and display metadata.
- `difficulty`: stable per-difficulty key, checksum/online ID, ruleset and difficulty attributes.
- `score`: score identity, timestamp, PP, accuracy, combo, hit counts and mods.
- `replay`: SHA-256, original file name and the independent file-upload opt-in.
- `analysis`: analysis schema/engine and exact judgement payload including miss classifications.

The client computes `contentHash` from immutable score fields. Hub enforces unique `(user, clientScoreId)` and `(user, contentHash)` keys. Repeating the same request returns the existing `shareId`; reusing a client score ID with a different content hash returns `409 Conflict`.

Example response:

```json
{
  "shareId": "osu_0123456789abcdef0123456789abcdef",
  "visibility": "unlisted",
  "created": true,
  "replayUploadRequired": true
}
```

## Replay bytes

`POST /api/osu/v1/replays/{shareId}/file`

Required headers:

```text
Authorization: Bearer <upload token>
X-Content-SHA256: <lowercase SHA-256 from replay metadata>
Content-Type: application/x-osu-replay
```

The server verifies account ownership, declared metadata hash, actual byte hash, content type, and the 64 MiB size cap before recording the object. Storage keys are content-addressed. Re-uploading an already completed identical file is deduplicated.

## Public reads

- `GET /api/osu/v1/community?limit=36` returns only `public` scores.
- `GET /api/osu/v1/profiles/{hubHandle}?limit=36` returns the client-reported osu! profile and only public replay history.
- `GET /api/osu/v1/replays/{shareId}` returns public or unlisted metadata/analysis.
- `GET /media/osu-replays/{shareId}.osr` downloads replay bytes only for public or unlisted shares with an uploaded file.

The public response joins the Hub identity, osu! profile, beatmap set, exact difficulty, score, optional replay file state, and optional analysis. Storage keys and owner IDs are never exposed.

## Storage and deployment

The existing startup schema bootstrap creates these PostgreSQL tables and indexes:

- `osu_profiles`
- `osu_beatmap_sets`
- `osu_beatmap_difficulties`
- `osu_scores`
- `osu_replay_files`
- `osu_replay_analyses`

No new environment variables are required. Replay bytes use the existing `AIMMOD_HUB_MEDIA_BACKEND` local/S3 configuration. Operators should snapshot the database before first deployment because schema creation currently runs at process startup rather than through a separate migration runner.
