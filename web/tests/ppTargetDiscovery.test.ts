import { test } from "node:test";
import assert from "node:assert/strict";
import { BeatmapItem, ProviderStatus, Ruleset } from "../src/gen/aimmod/osu/v1/osu_pb";
import { matchingPpDifficulties, needsPpDetails, ppResultsTitle, ppSearchError } from "../src/lib/ppTargetDiscovery";

const set = () => new BeatmapItem({ beatmapCount: 4, difficulties: [
  { beatmapId: "1", ruleset: Ruleset.OSU, stars: 3, checksum: "a".repeat(32) },
  { beatmapId: "2", ruleset: Ruleset.OSU, stars: 7, checksum: "b".repeat(32) },
  { beatmapId: "3", ruleset: Ruleset.OSU, stars: 8, checksum: "c".repeat(32) },
  { beatmapId: "4", ruleset: Ruleset.MANIA, stars: 4, checksum: "d".repeat(32) },
] });

test("complete search sets need no detail requests and preserve inclusive star and mode filtering", () => {
  const item = set();
  assert.equal(needsPpDetails(item, "3", "7"), false);
  assert.deepEqual(matchingPpDifficulties(item, "3", "7").map(map => map.beatmapId), ["2", "1"]);
  assert.deepEqual(item.difficulties.map(map => map.beatmapId), ["1", "2", "3", "4"]);
  assert.equal(needsPpDetails(item, "9", "10"), false);
});

test("incomplete search metadata still fetches details, without fetching irrelevant checksums", () => {
  const item = set();
  item.difficulties[2].checksum = "";
  assert.equal(needsPpDetails(item, "3", "7"), false);
  item.difficulties[0].checksum = "";
  assert.equal(needsPpDetails(item, "3", "7"), true);
  assert.equal(needsPpDetails(new BeatmapItem(), "3", "7"), true);
  const partial = set(); partial.beatmapCount = 5;
  assert.equal(needsPpDetails(partial, "3", "7"), true);
});

test("failed search is not presented as a successful zero-result search", () => {
  assert.equal(ppSearchError([new ProviderStatus({ available: true })]), undefined);
  assert.equal(ppSearchError([new ProviderStatus({ message: "context deadline exceeded" })]), "Beatmap search took too long. Please try again.");
  assert.equal(ppResultsTitle(true, false, 0, 0), "Results unavailable");
  assert.equal(ppResultsTitle(false, true, 0, 0), "Finding beatmaps…");
  assert.equal(ppResultsTitle(false, false, 0, 0), "0 beatmaps · 0 difficulties");
  assert.equal(ppResultsTitle(true, false, 2, 4), "2 beatmaps · 4 difficulties");
});
