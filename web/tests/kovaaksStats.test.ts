import { test } from "node:test";
import assert from "node:assert/strict";
import {
  accuracyTrend,
  activeDays,
  bestPerPlayer,
  formatChange,
  formatPlaytime,
  formatPoints,
  percentileOf,
  rankWithin,
  recordHolders,
  scoreTrend,
  topPercent,
} from "../src/lib/kovaaksStats";

const day = (n: number) => new Date(Date.UTC(2026, 5, 1 + n, 18)).toISOString();

test("time played reads in hours and minutes", () => {
  assert.equal(formatPlaytime(0), "under a minute");
  assert.equal(formatPlaytime(45 * 60_000), "45m");
  assert.equal(formatPlaytime(BigInt(2 * 3_600_000 + 5 * 60_000)), "2h 5m");
  assert.equal(formatPlaytime(3 * 3_600_000), "3h");
  assert.equal(formatPlaytime(250 * 3_600_000 + 60_000), "250h");
});

test("score trend compares the newest runs with the ones before them", () => {
  const runs = Array.from({ length: 12 }, (_, i) => ({ score: i < 6 ? 100 : 110, accuracy: i < 6 ? 70 : 75, playedAtIso: day(i) }));
  const trend = scoreTrend(runs, 6);
  assert.ok(trend);
  assert.equal(trend.recent, 110);
  assert.equal(trend.previous, 100);
  assert.equal(formatChange(trend.changePct), "+10.0%");
  assert.equal(scoreTrend(runs.slice(0, 5)), null, "too few runs to compare");
  const acc = accuracyTrend(runs, 6);
  assert.ok(acc);
  assert.equal(formatPoints(acc.changePts), "+5.0 pts");
  assert.equal(formatPoints(0.01), "steady");
});

test("best per player keeps one row per player, best first", () => {
  const rows = bestPerPlayer([
    { score: 50, accuracy: 70, playedAtIso: day(1), userHandle: "demo-a" },
    { score: 90, accuracy: 80, playedAtIso: day(2), userHandle: "demo-b" },
    { score: 70, accuracy: 75, playedAtIso: day(3), userHandle: "demo-a" },
    { score: 10, accuracy: 10, playedAtIso: day(4) },
  ]);
  assert.deepEqual(rows.map((r) => [r.handle, r.best.score, r.runs]), [["demo-b", 90, 1], ["demo-a", 70, 2]]);
  assert.equal(rows[1].averageScore, 60);
});

test("ranks and percentiles", () => {
  const board = [{ score: 300 }, { score: 200 }, { score: 100 }];
  assert.equal(rankWithin(250, board), 2);
  assert.equal(rankWithin(300, board), 1);
  assert.equal(rankWithin(50, board), null);
  assert.equal(percentileOf(200, [100, 200, 300, 400]), 50);
  assert.equal(percentileOf(1, []), null);
  assert.equal(topPercent(3, 40), 8);
  assert.equal(topPercent(1, 1000), 1);
  assert.equal(topPercent(0, 10), null);
});

test("active days counts distinct days inside the window", () => {
  const now = Date.parse(day(10));
  assert.equal(activeDays([{ playedAtIso: day(9) }, { playedAtIso: day(9) }, { playedAtIso: day(8) }, { playedAtIso: day(1) }], 7, now), 2);
});

test("record holders are ordered by records held", () => {
  const holders = recordHolders([{ userHandle: "demo-a" }, { userHandle: "demo-b" }, { userHandle: "demo-a" }, { userDisplayName: "" }]);
  assert.deepEqual(holders.map((h) => [h.handle, h.records]), [["demo-a", 2], ["demo-b", 1]]);
});
