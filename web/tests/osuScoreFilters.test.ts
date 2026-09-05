import assert from "node:assert/strict";
import test from "node:test";
import { filterOsuScores } from "../src/lib/osuScoreFilters";
import type { OsuSharedReplay } from "../src/lib/osuCommunity";

const scores = [
  { shareId: "a", source: "local", hasReplayFile: true, accuracy: .98, starRating: 5, performancePoints: 120, mods: ["HD"], passed: true },
  { shareId: "b", source: "official", officialReplayExists: true, accuracy: .99, starRating: 6, performancePoints: 220, mods: ["NC"], passed: true },
  { shareId: "c", source: "merged", hasReplayFile: false, officialReplayExists: false, accuracy: .8, starRating: 3, performancePoints: null, mods: [], passed: false },
].map(score => ({ ...score, title: "Map", beatmapId: 42, playedAt: "2026-09-05T00:00:00Z" })) as OsuSharedReplay[];
const ids = (query: string) => filterOsuScores(scores, new URLSearchParams(query)).map(item => item.shareId);

test("source filters retain merged plays in either source without duplicating them", () => {
  assert.deepEqual(ids("source=uploads"), ["a", "c"]);
  assert.deepEqual(ids("source=official"), ["b", "c"]);
  assert.deepEqual(ids("source=merged"), ["c"]);
});
test("replay availability includes official replay files without Hub attachments", () => {
  assert.deepEqual(ids("replay=file"), ["a", "b"]);
  assert.deepEqual(ids("replay=none"), ["c"]);
});
test("accuracy percent, PP and difficulty ranges combine and missing PP is not zero", () => {
  assert.deepEqual(ids("accMin=98&starsMin=5&ppMax=150"), ["a"]);
  assert.deepEqual(ids("ppMin=0"), ["a", "b"]);
  assert.deepEqual(ids("mods=DT"), ["b"]);
  assert.deepEqual(ids("mods=NM&result=failed"), ["c"]);
  assert.deepEqual(ids("sort=pp"), ["b", "a", "c"]);
  assert.equal(scores[0].shareId, "a");
});
