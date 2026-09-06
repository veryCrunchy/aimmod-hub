# AimMod PP worker

This headless service follows AimMod native's `OfficialPpWhatIfCalculator`: decode the exact beatmap, use the official difficulty calculator, construct a score, then run the official performance calculator. It pins `ppy.osu.Game` and all four `ppy.osu.Game.Rulesets.*` packages to **2026.730.0**, difficulty version **20260706** (July 2026 rebalance).

Lazer is the default for hypothetical full combos. Stable adds Classic and sets `IsLegacyScore`; recorded scores keep their own mode and full mod settings. FC estimates generate the nearest attainable judgement distribution, with perfect slider ends and no tick misses. Actual plays retain supplied statistics, combo and accuracy. Standard failed plays use timed difficulty for the judged prefix while displaying full-map stars. Taiko, catch and mania use their official mode-specific score calculators and full judgement distributions. Stable mania holds count once; catch validates combo objects separately from legacy tiny-droplet rounding. Hypothetical FC targets remain osu!standard. Official PP supplied by osu! remains authoritative.

The Go API forwards POST `/api/osu/v1/pp/calculate` to the loopback worker on port 5192. Requests contain beatmap bytes and calculation inputs, never account identity. Files are checksum-checked, staged under an OS-generated temporary name, and deleted after calculation. Requests and results are not logged. The worker bounds request size, concurrent calculations and calculation time.

Run locally with `dotnet run --project pp/AimMod.Pp.csproj`. Run calculator regression checks with `dotnet run --project tests/pp/AimMod.Pp.Tests.csproj -c Release`; run web checks with `pnpm --dir web test`. The container builds and runs both services.

For a ruleset update, review the official osu! release and NuGet package, update both pinned packages, the worker engine identifier, and both web cache namespaces. Re-run the calculator tests (including stable/lazer, sliders, configured mods and partial plays), review golden value changes, and verify the browser. Never retain old calculated PP caches across formula updates or silently fall back to the previous engine.

Sources: [official release](https://github.com/ppy/osu/releases/tag/2026.730.0), [Q2 ruleset change](https://github.com/ppy/osu/pull/37850).
