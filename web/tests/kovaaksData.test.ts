import { test } from "node:test";
import assert from "node:assert/strict";
import { createElement } from "react";
import { renderToString } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { HelmetProvider } from "../src/lib/helmet";
import { AuthProvider } from "../src/lib/AuthContext";
import {
  groupCategories,
  rankCounts,
  rankProgress,
  sheetColumns,
  sortScenarios,
  splitCategory,
  compactScore,
  type SheetCategory,
} from "../src/lib/benchmarkSheet";
import { readUrlState, writeUrlState, intParam } from "../src/lib/urlState";
import { groupQuickResults, loadRecentSearches, quickResultHref, rememberSearch } from "../src/lib/quickSearch";
import { countryFlag } from "../src/lib/country";
import { BenchmarkSheet, BenchmarkSummary } from "../src/components/BenchmarkSheet";
import { KovaaksPlayerNotice } from "../src/components/KovaaksPlayerNotice";
import { Pager, pageCount } from "../src/components/ui/Pager";
import { RankBadge } from "../src/components/RankBadge";
import { sortStats } from "../src/components/PlayerScenarioStats";
import { compareDelta, sortCompared } from "../src/pages/ComparePage";
import { AimModPage, BETA_SETUP_URL } from "../src/pages/AimModPage";
import { buildItems } from "../src/components/HeaderSearch";
import { browseBenchmarkGroups } from "../src/lib/benchmarkGroups";
import { CompareScenario, PlayerScenarioStat, QuickSearchResult } from "../src/gen/aimmod/hub/v1/hub_pb";

const ladder = [
  { rankIndex: 1, rankName: "Iron", color: "#8a8f98", score: 100 },
  { rankIndex: 2, rankName: "Bronze", color: "#b07840", score: 200 },
  { rankIndex: 3, rankName: "Silver", color: "#b8c0c8", score: 300 },
];

const sheet: SheetCategory[] = [
  { categoryName: "Static Clicking", scenarios: [
    { scenarioName: "Synthetic A", categoryName: "Static Clicking", score: 250, thresholds: ladder },
    { scenarioName: "Synthetic B", categoryName: "Static Clicking", score: 50, thresholds: ladder },
  ] },
  { categoryName: "Smooth Tracking", scenarios: [
    { scenarioName: "Synthetic C", categoryName: "Smooth Tracking", score: 330, thresholds: ladder, scoreSource: "aimmod" },
    { scenarioName: "Synthetic D", categoryName: "Smooth Tracking", score: 290, thresholds: ladder },
  ] },
  { categoryName: "Dynamic Clicking", scenarios: [
    { scenarioName: "Synthetic E", categoryName: "Dynamic Clicking", score: 150, thresholds: ladder },
  ] },
];

function render(route: string, element: ReturnType<typeof createElement>) {
  return renderToString(createElement(HelmetProvider, { context: {} },
    createElement(AuthProvider, null, createElement(MemoryRouter, { initialEntries: [route] }, element))));
}

test("rank progress reports the current rank, the next one and what is left", () => {
  const mid = rankProgress(250, ladder);
  assert.equal(mid.current?.rankName, "Bronze");
  assert.equal(mid.next?.rankName, "Silver");
  assert.equal(mid.pointsToNext, 50);
  assert.equal(mid.pctToNext, 50);
  const below = rankProgress(50, ladder);
  assert.equal(below.current, null);
  assert.equal(below.pctToNext, 50);
  const top = rankProgress(330, ladder);
  assert.ok(top.maxed);
  assert.equal(top.pctOverTop, 10);
  assert.equal(rankProgress(10, []).next, null);
});

test("sheet helpers keep author order, group families and sort by need", () => {
  assert.deepEqual(splitCategory("Smoothness Tracking"), { parent: "Tracking", sub: "Smoothness" });
  assert.deepEqual(splitCategory("Speed"), { parent: "Speed", sub: null });
  const groups = groupCategories(sheet);
  assert.deepEqual(groups.map((g) => [g.parent, g.rows]), [["Clicking", 3], ["Tracking", 2]]);
  assert.deepEqual(sortScenarios(sheet, "sheet").map((s) => s.scenarioName), ["Synthetic A", "Synthetic B", "Synthetic E", "Synthetic C", "Synthetic D"]);
  // D is 90% of the way to Silver, maxed C goes last.
  assert.deepEqual(sortScenarios(sheet, "closest").map((s) => s.scenarioName), ["Synthetic D", "Synthetic A", "Synthetic B", "Synthetic E", "Synthetic C"]);
  assert.equal(sortScenarios(sheet, "weakest")[0].scenarioName, "Synthetic B");
  assert.equal(sheetColumns(sheet).length, 3);
  const counts = rankCounts(sheet);
  assert.equal(counts.total, 5);
  assert.equal(counts.unranked, 1);
  assert.deepEqual(counts.counts.map((c) => [c.rankName, c.count]), [["Silver", 1], ["Bronze", 2], ["Iron", 1]]);
  assert.equal(compactScore(12_345), "12.3k");
  assert.equal(compactScore(1250), "1,250");
  assert.equal(compactScore(812.5), "812.5");
});

test("benchmark sheet renders thresholds, next ranks and the AimMod score marker", () => {
  const html = render("/", createElement(BenchmarkSheet, { categories: sheet, sort: "sheet", onSortChange: () => {}, scenarioHref: (s) => `/scenarios/${s.scenarioName}` }));
  assert.match(html, /Next rank/);
  assert.match(html, /Silver/);
  assert.match(html, /\+(<!-- -->)?50/);
  assert.match(html, /Top rank/);
  assert.match(html, /AimMod/);
  assert.match(html, /href="\/scenarios\/Synthetic A"/);
  assert.match(html, /role="progressbar"/);
  const summary = render("/", createElement(BenchmarkSummary, { categories: sheet, overallRank: { rankIndex: 0, rankName: "No Rank" }, benchmarkProgress: 420 }));
  assert.match(summary, /Not ranked yet/);
  assert.match(summary, /4<\/strong>.*of (<!-- -->)?5(<!-- -->)? scenarios ranked/);
  assert.match(summary, /420/);
  assert.match(summary, /1(<!-- -->)? (<!-- -->)?score(<!-- -->)? from AimMod uploads/);
});

test("url state drops defaults and ignores unknown choices", () => {
  const defaults = { tab: "aimmod", page: "1", q: "" };
  const params = new URLSearchParams("tab=bogus&page=3&other=keep");
  assert.deepEqual(readUrlState(params, defaults, { tab: ["aimmod", "kovaaks"] }), { tab: "aimmod", page: "3", q: "" });
  const next = writeUrlState(params, defaults, { tab: "kovaaks", page: "1", q: "synthetic" });
  assert.equal(next.toString(), "tab=kovaaks&other=keep&q=synthetic");
  assert.equal(intParam("12", 1, 1, 10), 10);
  assert.equal(intParam("x", 4), 4);
});

test("quick search links, groups and remembers results", () => {
  assert.equal(quickResultHref({ kind: "player", title: "Demo", userHandle: "demo-a" }), "/profiles/demo-a");
  assert.equal(quickResultHref({ kind: "kovaaks_player", title: "x", steamId: "76561190000002001" }), "/u/76561190000002001");
  assert.equal(quickResultHref({ kind: "kovaaks_scenario", title: "Synthetic Grid", scenarioSlug: "synthetic-grid", scenarioName: "Synthetic Grid" }), "/scenarios/synthetic-grid?tab=kovaaks&name=Synthetic%20Grid");
  assert.equal(quickResultHref({ kind: "benchmark", title: "B", benchmarkId: 9 }), "/benchmarks/9");
  const groups = groupQuickResults([
    { kind: "scenario", title: "s", relevance: 50 },
    { kind: "player", title: "p", relevance: 90 },
    { kind: "scenario", title: "s2", relevance: 40 },
  ]);
  assert.deepEqual(groups.map((g) => [g.label, g.items.length]), [["Players", 1], ["Scenarios", 2]]);
  const store = new Map<string, string>();
  const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => { store.set(k, v); } };
  rememberSearch(storage, { title: "A", href: "/a", kind: "player" });
  rememberSearch(storage, { title: "B", href: "/b", kind: "player" });
  rememberSearch(storage, { title: "A", href: "/a", kind: "player" });
  assert.deepEqual(loadRecentSearches(storage).map((r) => r.href), ["/a", "/b"]);
  store.set("aimmod-recent-searches-v1", JSON.stringify([{ title: "x", href: "https://elsewhere.example", kind: "player" }]));
  assert.deepEqual(loadRecentSearches(storage), [], "only same-site paths are kept");
  assert.deepEqual(loadRecentSearches(undefined), []);
});

test("header search rows are grouped and capped per section", () => {
  const results = [
    ...Array.from({ length: 7 }, (_, i) => new QuickSearchResult({ kind: "scenario", title: `Synthetic ${i}`, scenarioSlug: `synthetic-${i}`, relevance: 80 - i, count: BigInt(10) })),
    new QuickSearchResult({ kind: "kovaaks_player", title: "synthetic-remote-01", steamId: "76561190000002001", relevance: 30, country: "nl" }),
  ];
  const items = buildItems(results);
  assert.equal(items.filter((i) => i.kind === "scenario").length, 5);
  assert.equal(items[0].group, "Scenarios");
  assert.equal(items[0].meta, "10 runs");
  assert.equal(items.at(-1)?.to, "/u/76561190000002001");
});

test("player stats and comparisons sort as labelled", () => {
  const stats = [
    new PlayerScenarioStat({ scenarioName: "Synthetic A", runCount: 3, bestScore: 10, percentile: 50, trendPct: 4, consistency: 70, lastPlayedAtIso: "2026-01-02T00:00:00Z", scenarioType: "Tracking" }),
    new PlayerScenarioStat({ scenarioName: "Synthetic B", runCount: 9, bestScore: 5, percentile: 90, trendPct: -2, consistency: 95, lastPlayedAtIso: "2026-01-05T00:00:00Z", scenarioType: "Clicking" }),
  ];
  assert.equal(sortStats(stats, "runs")[0].scenarioName, "Synthetic B");
  assert.equal(sortStats(stats, "best")[0].scenarioName, "Synthetic A");
  assert.equal(sortStats(stats, "trend")[0].scenarioName, "Synthetic A");
  assert.equal(sortStats(stats, "recent")[0].scenarioName, "Synthetic B");
  assert.deepEqual(sortStats(stats, "name", "click").map((s) => s.scenarioName), ["Synthetic B"]);
  const shared = [
    new CompareScenario({ scenarioName: "Synthetic A", score: 110, otherScore: 100, runCount: 1, otherRunCount: 1 }),
    new CompareScenario({ scenarioName: "Synthetic B", score: 80, otherScore: 100, runCount: 5, otherRunCount: 5 }),
  ];
  assert.equal(Math.round(compareDelta(110, 100)), 10);
  assert.equal(compareDelta(5, 0), 100);
  assert.equal(sortCompared(shared, "lead")[0].scenarioName, "Synthetic A");
  assert.equal(sortCompared(shared, "gap")[0].scenarioName, "Synthetic B");
  assert.equal(sortCompared(shared, "played")[0].scenarioName, "Synthetic B");
});

test("benchmark browse hides junk only when asked and sorts by KovaaK's players", () => {
  const items = [
    { benchmarkId: 1, benchmarkName: "Synthetic Popular", benchmarkIconUrl: "", benchmarkAuthor: "a", benchmarkType: "", playerCount: 0, kovaaksPlayers: BigInt(9000) },
    { benchmarkId: 2, benchmarkName: "Synthetic Niche", benchmarkIconUrl: "", benchmarkAuthor: "b", benchmarkType: "", playerCount: 2, kovaaksPlayers: BigInt(40) },
    { benchmarkId: 3, benchmarkName: "1212311231231", benchmarkIconUrl: "", benchmarkAuthor: "c", benchmarkType: "", playerCount: 0, kovaaksPlayers: BigInt(0), hidden: true },
  ];
  assert.deepEqual(browseBenchmarkGroups(items, "", "kovaaks", false, false).map((g) => g.base), ["Synthetic Popular", "Synthetic Niche"]);
  assert.equal(browseBenchmarkGroups(items, "", "kovaaks", false, true).length, 3);
  assert.equal(browseBenchmarkGroups(items, "", "players", false, false)[0].base, "Synthetic Niche");
});

test("KovaaK's player notice marks unlinked players and offers the link path", () => {
  const html = render("/", createElement(KovaaksPlayerNotice, { steamId: "76561190000002001" }));
  assert.match(html, /not on AimMod/);
  assert.match(html, /href="\/app\/kovaaks\?claim=76561190000002001#link-account"/);
  const linked = render("/", createElement(KovaaksPlayerNotice, { steamId: "76561190000001001", aimmodHandle: "demo-kestrel" }));
  assert.match(linked, /href="\/profiles\/demo-kestrel"/);
  assert.doesNotMatch(linked, /link my account/);
});

test("pager and rank badge render their states", () => {
  assert.equal(pageCount(0, 50), 1);
  assert.equal(pageCount(101, 50), 3);
  const pager = renderToString(createElement(Pager, { page: 0, pageSize: 50, total: 120, onPage: () => {} }));
  assert.match(pager, /1–50 of 120/);
  assert.match(pager, /of (<!-- -->)?3/);
  assert.match(pager, /disabled="" aria-label="Previous page"/);
  assert.match(renderToString(createElement(RankBadge, { rankName: "No Rank" })), /Unranked/);
  assert.match(renderToString(createElement(RankBadge, { rankName: "Gold", color: "#e0b840" })), /Gold/);
  assert.equal(countryFlag("nl").length > 0, true);
  assert.equal(countryFlag("x"), "");
});

test("the KovaaK's download page links the beta installer and explains SmartScreen", () => {
  const html = render("/app/kovaaks", createElement(AimModPage));
  assert.ok(html.includes(BETA_SETUP_URL.replace(/&/g, "&amp;")));
  assert.match(html, /More info/);
  assert.match(html, /Run anyway/);
  assert.match(html, /Uninstall/);
  assert.doesNotMatch(html, /Download Stable/, "no stable button before a stable release exists");
  const claim = render("/app/kovaaks?claim=76561190000002001", createElement(AimModPage));
  assert.match(claim, /id="link-account"/);
});
