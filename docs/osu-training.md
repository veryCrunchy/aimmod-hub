# osu! training profiles

Training sessions use the linked Hub account as their owner. They do not establish ownership of an official osu! account. The generic Hub profile includes training, and an osu! profile includes it when it resolves to a Hub handle. A dedicated view is available at `/osu/profiles/{hubHandle}/training`; `/osu/training` shows the signed-in account's sessions and sharing controls.

## Desktop sync

In **Settings → Account & sharing → Training**, enable **Sync new completed training sessions**. Sync and public sharing both default off. New sessions are private unless **Show new training sessions on my public profile** is enabled. Existing history is not backfilled. Stopped sessions are not uploaded.

The persistent queue captures the Hub link, osu! account and consent generation when the session starts. It uses bearer authentication, keeps results after failed requests, and retries with exponential backoff up to one hour. Disabling sync invalidates unsent sessions; re-enabling does not publish a previous consent generation. Relinking or changing accounts cannot send another account's pending sessions. Local history remains independent of upload success.

Input keys, audio offset, source filenames, replay data, tablet settings and device paths are not included in the upload DTO. A local digest distinguishes timing-affecting configurations without transmitting those settings.

## API

| Endpoint | Access | Behaviour |
| --- | --- | --- |
| `POST /api/osu/v1/training/sessions` | Upload bearer token | `{ schemaVersion: 1, sessions: [...] }`; 1–20 completed sessions, maximum 128 KiB. Returns `{ accepted: count }`. |
| `GET /api/osu/v1/training/profiles/{hubHandle}` | Public | Public sessions only, irrespective of viewer credentials. |
| `GET /api/osu/v1/training/me` | Session cookie or bearer token | The authenticated owner's public and private sessions. |
| `POST /api/osu/v1/training/visibility` | Owner cookie or bearer token | `{ id, visibility: "private" \| "public" }`. Cookie requests require the trusted web origin. |

Read queries accept `days` (1–365, default 30) and `mode` (`steady`, `alternating`, `bursts`, `rhythm`, `aim`, `reading`, `reaction`). Results contain totals, daily UTC activity, skill summaries and the latest 50 sessions. Aggregation is bounded to the latest 5,000 sessions in the chosen period; `truncated` explicitly describes partial coverage. Profile responses are not cached, so visibility changes take effect on the next read.

The payload contract is defined by `TrainingSession` / `TrainingSetup` in the store package and `HubTrainingSession` / `HubTrainingSetup` in the desktop Hub code. Accuracy uses percentages from 0 to 100. Timing metrics use milliseconds; practice duration uses seconds. Nullable metrics remain unknown, never zero-filled.

Sessions are keyed by owner and UUID. Retries do not duplicate sessions. A changed result with an existing UUID returns 409, and the entire batch rolls back. Retrying an old upload never overrides a later visibility change. The authenticated token supplies the owner; request bodies cannot assign a user ID.

## Progress calculations

- Practice totals include completed sessions in the selected window. They do not count background app time.
- Clean native sessions need at least 95% accuracy, 95% targets hit, no more than 5% extra taps and at most 25 ms timing spread.
- Clean peak pace requires at least three qualifying runs and uses a conservative lower-quartile observation. It is not a sustained stream-speed rating or an account rank.
- Improvement compares the first three and latest three sessions in the most recently practised matching fixed configuration, including engine and source digest. The groups never overlap. Randomized layouts contribute to practice volume and clean observations, not controlled before/after comparisons.
- Accuracy changes are percentage points. Negative timing-spread changes indicate tighter timing. Reaction sessions compare response time separately from osu! judgement accuracy.
- These are trainer observations; they do not establish transfer to an original ranked map or a causal training benefit.

## Validation

Run `go test ./...`, the web tests and the desktop Hub/trainer tests. Database integration is opt-in with `AIMMOD_TRAINING_TEST_DATABASE_URL`; the test creates an isolated schema and verifies persistence, duplicate retries, rollback, owner isolation and visibility. It must use a dedicated test database. UI fixtures and captures stay outside the repositories.

The table and indexes are created by the existing idempotent schema setup. Deploy the API before enabling the feature in released clients; clients retain pending sessions while an older server returns 404. These changes do not migrate or publish existing local practice history.
