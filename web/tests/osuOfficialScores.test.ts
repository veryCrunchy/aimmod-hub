import assert from "node:assert/strict";
import test from "node:test";
import { fetchOsuOfficialScore, fetchOsuScoreHistory, osuScorePath, type OsuSharedReplay } from "../src/lib/osuCommunity";

test("official history retains merged upload identity and selected ruleset", async () => {
  const original = globalThis.fetch;
  let requested = "";
  globalThis.fetch = async input => {
    requested = String(input);
    return Response.json({ profile: {}, coverage: { best: { status: "available" }, recent: { status: "available" } }, items: [{ shareId: "osu_shared", source: "merged", officialScoreId: "123", performancePoints: 0, analysis: { judgements: [] } }] });
  };
  try {
    const result = await fetchOsuScoreHistory("player", "mania");
    assert.ok(requested.includes("mode=mania"));
    assert.equal(result.items[0].performancePoints, 0);
    assert.equal(osuScorePath(result.items[0]), "/osu/replays/osu_shared");
    assert.equal(osuScorePath({ source: "official", officialScoreId: "123" } as OsuSharedReplay), "/osu/scores/123");
  } finally { globalThis.fetch = original; }
});

test("official replay existence is separate from uploaded replay availability", async () => {
  const original = globalThis.fetch;
  globalThis.fetch = async () => Response.json({ item: { source: "official", officialScoreId: "123", hasReplayFile: false }, replay: { exists: true, downloadAvailable: false } });
  try {
    const result = await fetchOsuOfficialScore("123");
    assert.equal(result.hasReplayFile, false);
    assert.equal(result.officialReplayExists, true);
  } finally { globalThis.fetch = original; }
});
