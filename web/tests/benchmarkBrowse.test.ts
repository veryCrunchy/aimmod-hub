import { test } from "node:test";
import assert from "node:assert/strict";
import { browseBenchmarkGroups } from "../src/lib/benchmarkGroups";

const item = (benchmarkId: number, benchmarkName: string, playerCount: number) => ({
  benchmarkId, benchmarkName, playerCount, benchmarkIconUrl: "", benchmarkAuthor: "Synthetic Author", benchmarkType: "benchmark",
});

const catalog = [
  item(1, "Synthetic Set - Novice", 2),
  item(2, "Synthetic Set - Advanced", 5),
  item(3, "Quiet Benchmark", 0),
  item(4, "Busy Benchmark", 4),
];

test("benchmarks with the most ranked players come first and variants stay grouped", () => {
  const groups = browseBenchmarkGroups(catalog, "", "players", false);
  assert.deepEqual(groups.map((g) => g.base), ["Synthetic Set", "Busy Benchmark", "Quiet Benchmark"]);
  assert.equal(groups[0].variants.length, 2);
});

test("ranked-only hides benchmarks without Hub players and search matches names", () => {
  assert.deepEqual(browseBenchmarkGroups(catalog, "", "players", true).map((g) => g.base), ["Synthetic Set", "Busy Benchmark"]);
  assert.deepEqual(browseBenchmarkGroups(catalog, "quiet", "name", false).map((g) => g.base), ["Quiet Benchmark"]);
  assert.deepEqual(browseBenchmarkGroups(catalog, "", "name", false).map((g) => g.base), ["Busy Benchmark", "Quiet Benchmark", "Synthetic Set"]);
});

test("placeholder ranks are not shown as ranks", async () => {
  const { hasRank } = await import("../src/components/BenchmarkCards");
  for (const rankName of ["", "No rank", "Unranked", "0/5", "0 / 12"]) assert.equal(hasRank({ rankName }), false, rankName);
  for (const rankName of ["Iron", "Gold", "3/5"]) assert.equal(hasRank({ rankName }), true, rankName);
});
