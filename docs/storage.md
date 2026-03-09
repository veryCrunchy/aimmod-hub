# AimMod Hub Storage Design

## Goal

Store a very large amount of user practice data cheaply without giving up the features that matter:
- profiles
- scenario history
- public sharing
- replay review
- coaching
- aim fingerprint improvements
- aggregate learning from community data

The key rule is:

**keep summaries and derived features forever, keep raw high-volume data selectively, and move large blobs out of Postgres**

## Storage layers

### 1. Relational core: Postgres

Use Postgres for:
- users
- linked accounts
- upload batches
- runs
- per-run summaries
- derived features
- scenario metadata
- public sharing metadata
- aggregate tables

Postgres should be the source of truth for product behavior.

It should **not** be the long-term home for:
- raw replay frames
- large raw telemetry blobs
- frame-by-frame video assets

### 2. Blob storage: object storage

Use S3/R2-compatible object storage for:
- replay video blobs
- frame bundles
- raw telemetry archives
- screenshots
- exported share media

Blob storage should be addressed by immutable object keys and referenced from Postgres.

### 3. Derived analytics layer

Derived analytics should live in compact relational tables:
- per-run feature vectors
- per-scenario aggregates
- per-user rolling aggregates
- coaching-ready segment summaries

These are what most product surfaces should read.

## Data classes

### A. Permanent low-volume data

Keep forever:
- user identity and settings
- scenario run summaries
- compact per-second timelines
- derived feature sets
- public profile and sharing metadata
- scenario aggregate snapshots

This gives us nearly all useful product functionality at low storage cost.

### B. Retained medium-volume data

Keep temporarily, then downsample:
- raw shot telemetry
- detailed segment telemetry
- dense mouse paths
- detailed replay timing streams

Suggested retention:
- 30 to 90 days for private runs
- forever for shared, starred, top-performance, or explicitly retained runs

### C. Large blob data

Keep in blob storage only:
- replay video assets
- frame bundles
- media exports

Suggested retention:
- short retention for private runs
- long retention only for shared or important runs

## Principle: features over raw samples

Most value does not require storing every sample forever.

Instead of keeping all raw path points forever, keep:
- overshoot count
- overshoot severity
- correction ratio
- path efficiency
- jitter
- speed percentiles
- stop precision
- target-switch hesitation
- click timing variance
- target lead/lag features
- tracking stability features

These can support:
- aim fingerprint
- coaching
- practice profile
- scenario comparison
- aggregate learning

without needing permanent raw sample storage.

## Replay model

Replays should use a tiered representation.

### Full fidelity replay

Used for:
- recent runs
- shared runs
- starred runs
- top runs

Contains:
- dense mouse path
- shot events
- per-second timeline
- video or frame bundle

### Compact replay

Used for old private runs.

Contains:
- per-second timeline
- context windows
- saved moments
- derived replay annotations
- optional sparse mouse keyframes

This preserves most coaching value while dropping heavy storage.

## Recommended canonical run model

Each uploaded run should fan out into these pieces:

### 1. `scenario_runs`

One row per run.

Fields:
- `run_id`
- `user_id`
- `session_id`
- `scenario_id`
- `played_at`
- `duration_ms`
- `score`
- `accuracy`
- `kps`
- `spm`
- `scenario_type`
- `scenario_subtype`
- `shared_visibility`
- `retention_tier`

### 2. `run_summaries`

Compact stable summary values.

Fields:
- score floor / median / peak markers
- best TTK
- avg TTK
- damage efficiency
- shot totals
- hit totals
- pause count
- replay availability flags

### 3. `run_timeline_seconds`

One row per second of run.

This is cheap and should usually be kept forever.

Fields:
- `run_id`
- `t_sec`
- `score`
- `accuracy`
- `damage_eff`
- `spm`
- `shots`
- `hits`
- `kills`
- `paused`

### 4. `run_feature_sets`

Dense derived coaching and aim profile features.

Fields:
- smoothness metrics
- jitter metrics
- overshoot metrics
- correction metrics
- click timing metrics
- target transition metrics
- tracking lag/lead metrics
- consistency metrics

### 5. `run_context_windows`

Compressed coaching moments instead of raw samples.

Fields:
- `run_id`
- `start_ms`
- `end_ms`
- `window_type`
- `label`
- `feature_summary`
- `coaching_tags`

This is the right long-term substrate for coaching UX.

### 6. `run_blob_refs`

References to optional heavy assets.

Fields:
- `run_id`
- `blob_type`
- `storage_key`
- `encoding`
- `size_bytes`
- `expires_at`

## Object storage formats

### Mouse path

Do not store as JSON arrays long-term.

Preferred:
- protobuf binary
- delta encoded x/y/time
- zstd compressed

Alternative:
- msgpack + zstd

### Shot telemetry

Preferred:
- protobuf binary with repeated structured records
- zstd compressed

### Replay frames / video

Do not store frame base64 in SQL.

Preferred:
- mp4/webm clip if we want simple playback
- or chunked frame bundle in object storage if exact frame stepping matters

For the hub, clips are often better than raw frames.

Desktop can remain richer than web if needed.

## Retention tiers

Every run should be assigned a retention tier at ingest or shortly after.

### Tier 0: summary only

Keep:
- run row
- summary
- per-second timeline
- feature set
- context windows

Delete:
- raw telemetry blob
- dense path blob
- replay video blob

Use for:
- ordinary old private runs

### Tier 1: compact replay

Keep:
- everything in tier 0
- sparse mouse path blob
- shot telemetry blob

Delete:
- full video after retention window

Use for:
- normal recent runs

### Tier 2: full replay

Keep:
- everything in tier 1
- full mouse path
- full shot telemetry
- video/blob asset

Use for:
- shared runs
- starred runs
- PB runs
- top percentile runs
- manually pinned runs

## Upload strategy

The desktop app should not upload everything blindly.

### Upload always

- run summary
- derived feature set
- per-second timeline
- compact run metadata

### Upload conditionally

- full shot telemetry
- dense mouse path
- video / frame blob

Conditions:
- user enabled sync
- user enabled replay upload
- run is shared/starred/PB
- retention policy allows it

This makes storage predictable.

## Compression strategy

### Relational tables

Keep rows narrow.

Prefer:
- numeric columns
- enum-like small strings or ids
- foreign keys to dictionaries

Avoid:
- large JSON blobs in core tables

### Telemetry blobs

Prefer:
- protobuf binary
- delta encoding
- zstd compression

This is much smaller than JSON and much cheaper to move around.

## Deduplication strategy

Deduplicate anything static or repeated:
- scenario definitions
- map metadata
- bot profile labels
- target metadata dictionaries
- replay thumbnails if identical generation pipeline output is reused

Avoid embedding repeated descriptive strings on every row when an ID will do.

## Aggregation strategy

Do not recompute large aggregates from raw runs on every page load.

Instead maintain:
- `user_scenario_rollups`
- `scenario_global_rollups`
- `user_practice_rollups`
- `coaching_model_inputs`

Update them asynchronously after ingest.

## Privacy and cost controls

### Private by default

All uploaded runs should default to private.

### Replay asset opt-in

Video and dense replay assets should require explicit opt-in.

### Auto-pruning

Old private full-fidelity assets should be pruned automatically.

### Hard byte budgets

Per user, define quotas for:
- retained replay media
- retained dense telemetry

Then degrade old runs automatically to lower tiers.

## Recommended first implementation

### Keep forever

- run summary
- feature set
- per-second timeline
- context windows

### Keep for 30 days

- full shot telemetry
- dense mouse path
- replay video

### Keep forever only when explicitly important

- shared runs
- starred runs
- PB runs
- top percentile runs

This is the best starting point because it preserves the features users care about while keeping long-term storage under control.

## What the app should eventually upload

### Required upload payload

- run identity
- scenario identity
- summary metrics
- per-second timeline
- derived feature vector
- compact context windows

### Optional upload payload

- full shot telemetry blob
- dense mouse path blob
- replay video/blob reference

That split gives the hub enough to be useful even when users do not upload expensive replay assets.
