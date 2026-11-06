# Account verification and refresh

Uploads require an upload token obtained through the existing device linking flow. Steam and KovaaK's identifiers observed in uploads are retained as unverified observations. They cannot merge profiles or reserve ownership. Only proven links are used to attribute external benchmark ranks to a Hub profile.

Set `STEAM_OPENID_REALM` to the public HTTPS API origin, without a trailing slash, to enable Steam verification. The callback is `/auth/steam/callback` on that origin. No Steam API key is required. Signed-in users start verification from Account and return there after server-side verification. Discord and Steam verification cookies require HTTPS, including local testing through an HTTPS endpoint.

Schema upgrades run through the existing startup migration mechanism. The ownership upgrade preserves observed identities, and adds an ownership proof field. Links that existed before the upgrade keep their verified state; only new links made from uploads start unverified until the player verifies with Steam. Discord links and run history remain. Existing merged accounts cannot be reconstructed automatically.

Benchmark sync keeps `definition_synced_at` as last success and adds `last_attempt`, `next_retry`, and `failure_count`. Failed definitions retry after 60 seconds with exponential backoff capped at six hours. Provider backoff stops the batch.

Batch uploads accept at most 100 sessions and 16 MiB. Each session accepts at most 7,200 timeline points, 2,000 context windows, 256 summary/features per map, and 64 tags per window. Videos accept at most 256 MiB; each user has a 2 GiB quota. Uploads spool to temporary disk before streaming to the configured storage backend. The temporary directory must have sufficient space.

Benchmark leaderboard snapshots are held in memory, with at most 64 benchmarks per process and one background refresh queue. Refreshes use two concurrent player requests through the existing shared provider rate budget. Snapshots refresh on demand after 15 minutes; restarting a process clears them. Pagination and filtering run on the server. Response headers expose snapshot time, partial status, total count and current page. Failed refreshes retain the preceding snapshot.

Run `go test ./...`, `pnpm --dir web test`, and `pnpm --dir web build`. Set `AIMMOD_TEST_DATABASE_URL` to a disposable PostgreSQL database to run the isolated-schema ownership and retry regression test.
