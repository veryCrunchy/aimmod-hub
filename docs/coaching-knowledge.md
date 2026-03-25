# Coaching Knowledge Model

AimMod Hub now uses a normalized coaching authoring model under `api/internal/coaching/content/`.

## Why

We want the coaching layer to scale beyond hand-written cards:

- per-flaw scenario recommendations
- scenario explanations and cautions
- mechanic explanations and cues
- transcript-derived knowledge with provenance
- local LLM tool use over stable published packs

## Source of truth

The source of truth is the normalized content tree:

- `content/config.json`
- `content/sources/*.json`
- `content/flaws/*.json`
- `content/mechanics/*.json`
- `content/scenarios/*.json`
- `content/recommendations/*.json`

These files are intended to be easy for humans and transcript-processing pipelines to write.

## Entity model

### `source`

Origin metadata for claims.

Suggested use:
- YouTube video
- coaching transcript
- internal seed content
- reviewed article or note

### `flaw`

The player problem or bottleneck.

Carries:
- signal keys
- context tags
- telltales
- contraindications
- linked mechanic ids

### `mechanic`

A mechanical concept that can appear across multiple flaws.

Carries:
- explanation
- cues
- failure modes
- related signal keys

### `scenario`

A drill family or named scenario.

Carries:
- aliases
- scenario families
- what it trains
- which flaws it helps
- cautions

### `recommendation`

The main runtime knowledge object.

Carries:
- linked flaw id
- linked mechanic ids
- linked scenario ids
- preference targeting
- actions
- why
- avoid
- evidence

## Provenance

Every reusable coaching claim should eventually carry evidence with:

- `sourceId`
- `claim`
- `excerpt`
- `startSec`
- `endSec`
- `confidence`
- `reviewStatus`

This lets us ingest transcript-derived candidates without blindly promoting them into runtime guidance.

## Publish step

Build the published runtime pack with:

```bash
pnpm build:coachpack
```

That writes:

- `api/internal/coaching/published/knowledge.v1.json`
- `api/internal/coaching/published/manifest.json`

The current API compiles from normalized content directly, but the published pack is the stable artifact local tooling and future review workflows can inspect.

## Intended transcript pipeline

The next stage should look like:

1. store transcript chunks under a separate ingestion workspace
2. use a larger LLM locally to extract structured candidates
3. write candidate `source`, `flaw`, `mechanic`, `scenario`, and `recommendation` JSON
4. attach provenance and review status
5. review and merge into `content/`
6. rebuild the published pack

The important rule is:

- transcripts generate candidates
- `content/` stores reviewed structured knowledge
- the app and local LLM read the published/runtime pack, not raw transcripts
