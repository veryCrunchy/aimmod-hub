# Research: how evxl presents KovaaK's benchmarks

Notes from studying evxl.app (October 2026) as a product reference for AimMod
Hub's KovaaK's pages. These are summaries in our own words; nothing here is
copied from the site, and none of its code, text or images is used.

## What evxl is

A benchmark tracker for KovaaK's (and Aimbeast). Any player can be looked up
by Steam id, Steam profile link or vanity name, with no account. All data
comes from KovaaK's public endpoints: benchmark progress, scenario
leaderboards and player search.

## Search

- One search box in the header and on the home page, opened with Ctrl+K.
- Results appear while typing, but only **players**: name plus Steam id and an
  initial as avatar. Scenarios and benchmarks have separate, page-local
  filters (benchmark list filter, scenario browser search).
- Inputs accepted: Steam64 id, vanity name, full profile URL; a help popover
  explains each.

Take-away: instant results and the keyboard shortcut are expected. We can do
better with one ranked palette across players (AimMod and KovaaK's),
scenarios and benchmarks, grouped by kind, with keyboard navigation.

## Benchmark pages (per player)

- A sheet per benchmark difficulty, chosen with tabs (Novice, Intermediate,
  Advanced, ...). URL: player id, benchmark name and difficulty, plus `?tab=`.
- Rows grouped by category and subcategory with vertical labels; the author's
  order is kept.
- Each scenario row: name, a small history icon, a play link, score, percent
  of the top threshold, and one cell per rank threshold, filled in the rank's
  colour once reached.
- Per-subcategory energy figures and a total; the overall rank shown as a
  large badge ("X Complete") with a rank-calculation selector.
- Actions: launch the benchmark playlist, refresh, share code, screenshot
  the sheet, copy.
- Charts tab: radar per category or subcategory with "percent above rank",
  scenarios per rank, rank by scenario, global rank distribution per
  benchmark (counts, cumulative or percentile; merge or hide complete and
  unranked), a scenario by rank heatmap or ridgeline, and sensitivity
  (cm/360) distribution.
- "Compare with Steam ID" next to the sheet.

Take-away: the threshold grid with rank colours is the core and players know
it. Missing: an explicit "what do I need next" (points to next rank) and an
order that tells you what to practise. History of ranks over time requires a
manual upload of the KovaaK's stats folder.

## Leaderboards

- Index of all benchmarks grouped (aim groups, notable), filter box,
  favourites, difficulty chips per benchmark, counts of groups and
  difficulties, last update time.
- Benchmark leaderboard: tabs Global, Category, Country, Friends; "My
  position"; toggle to exclude model overrides; country filter; username
  filter; total entries; "updated N days ago, next in N hours"; first,
  previous, next, last; page size; page number input.
- Columns: position with movement arrow, flag, player, energy, rank badge with
  progress percent, average position, average cm/360, date, links to the
  KovaaK's and Steam profiles.
- Category and subcategory leaderboards rank players by average rank per
  subcategory, drawn as coloured bars.
- Every filter is in the query string (`tab`, `lb`, `lbFilter`, `lbGroupBy`,
  `lbGroup`, `lbScenario`), so views can be shared.

Take-away: paging controls, jump to my position, update freshness and URL
state are all worth matching.

## Player profile

- A grid of every benchmark with the player's rank per benchmark, tag
  filters (Tracking, Clicking, ...), views by benchmark, category or scenario,
  a game switch, sorting, favourites, and a random pick.
- No general score history, trends or consistency: the profile is a
  benchmark index.

Take-away: AimMod has uploaded runs, so we can add what evxl cannot: per
scenario bests, trends, steadiness, AimMod standing, activity and head to
head comparisons.

## Scenario pages

- A scenario browser (name, type, author, paged) beside the selected
  scenario's leaderboard: score, rank badge against the benchmark that uses
  the scenario ("+N%" past the top rank, or percent towards the next),
  accuracy, cm/360 and date, plus "Percentile: top X%" for the viewed player.
- A cm/360 box plot of the top players.

Take-away: show where a score lands (percentile), the global board, and the
benchmark thresholds for that scenario.

## Rank history

- A separate tool that reads the local KovaaK's stats folder in the browser,
  with filters for sensitivity, dates and playtime, then plots rank over time
  for chosen benchmarks.

Take-away: AimMod can do this automatically from uploads.

## What we adopted

- Search palette across players, KovaaK's players, scenarios and benchmarks,
  ranked (exact, prefix, word prefix, substring, then popularity), with
  Ctrl+K and `/`, grouped sections, keyboard navigation and recent picks.
- One benchmark sheet for AimMod and KovaaK's players in the author's order
  with threshold cells in rank colours, plus a "next rank" column with points
  needed and progress, sorts for closest to next rank and weakest first, a
  phone layout with cards and a segmented ladder, and an overall badge with
  scenarios per rank.
- Rank over time from the player's own AimMod uploads, without any upload
  step.
- Scenario leaderboards: AimMod (period, sort, linked accounts only, name
  filter, paging, jump to me) and KovaaK's global (paging, country, AimMod
  marker, jump to me), percentiles with the viewer's position, and the
  benchmarks that use the scenario with their thresholds.
- KovaaK's-only player pages: anyone can be looked up, clearly marked as not
  on AimMod, with a "This is me" path to linking.
- Benchmark catalog hides empty, test-like and barely played benchmarks
  using KovaaK's own signals, with "Show all".
- Profile scenario table with trend, steadiness, AimMod rank and sparkline,
  an activity strip, and player comparisons.
- Filters in the URL everywhere.

## Not adopted yet (candidates)

- Global rank distribution per benchmark and the scenario by rank heatmap:
  needs a KovaaK's-wide sample; expensive to gather politely.
- Country and friends tabs on benchmark leaderboards; sensitivity
  distribution; playlist and share-code export; sheet screenshots.
- Energy and alternative rank calculations: KovaaK's progress is shown, but
  community energy formulas differ per benchmark and are not public data.
