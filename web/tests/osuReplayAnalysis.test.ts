import assert from "node:assert/strict";
import test from "node:test";
import { fetchOsuCommunity, fetchOsuProfile, fetchOsuReplay } from "../src/lib/osuCommunity";
import { formatOsuMissReason, normalizeOsuMissReason, normalizeOsuReplayAnalysis } from "../src/lib/osuReplayAnalysis";

// Minimal projection of the public production failure: string judgement, numeric native reason.
const analysis = { judgements: [{ objectIndex: 148, objectType: "HitCircle", startTimeMs: 56858, result: "Miss",
  missAnalysis: { reason: 2, confidence: .9, pressTimeOffsetMs: 26, closestDistance: 30.7865 } }] };

test("normalizes every native numeric miss reason without losing its meaning", () => {
  const names = ["Unknown", "EarlyClick", "LateClick", "Undershoot", "Overshoot", "OnTargetNoClick", "AimDeviation"];
  names.forEach((name, index) => {
    assert.equal(normalizeOsuMissReason(index), name);
    assert.equal(normalizeOsuMissReason(String(index)), name);
    assert.equal(normalizeOsuMissReason(name), name);
  });
  assert.equal(formatOsuMissReason(2), "Late Click");
  assert.equal(formatOsuMissReason("late_click"), "Late Click");
  for (const unknown of [null, undefined, {}, [], 99, -1, 1.5, NaN, ""]) assert.equal(formatOsuMissReason(unknown), "Unclassified miss");
  assert.equal(formatOsuMissReason("FutureReason"), "Future Reason");
});

test("production numeric reason is normalized before miss filtering and grouping", () => {
  const normalized = normalizeOsuReplayAnalysis(analysis)!;
  const misses = normalized.judgements!.filter(j => j.result?.toLowerCase() === "miss");
  assert.equal(misses.length, 1);
  assert.equal(misses[0].missAnalysis?.reason, "LateClick");
  assert.equal(formatOsuMissReason(misses[0].missAnalysis?.reason), "Late Click");
  assert.equal(misses[0].missAnalysis?.pressTimeOffsetMs, 26);
  assert.equal(misses[0].missAnalysis?.confidence, .9);
  assert.equal(analysis.judgements[0].missAnalysis.reason, 2);
});

test("malformed optional analysis cannot introduce unsafe strings or nonfinite metrics", () => {
  assert.equal(normalizeOsuReplayAnalysis(null), undefined);
  assert.deepEqual(normalizeOsuReplayAnalysis({ judgements: {} })?.judgements, []);
  const normalized = normalizeOsuReplayAnalysis({ judgements: [null, [], 4, { result: 2, startTimeMs: Infinity,
    missAnalysis: { reason: {}, confidence: "high", pressTimeOffsetMs: null } }] })!;
  assert.equal(normalized.judgements!.length, 1);
  assert.equal(normalized.judgements![0].result, undefined);
  assert.equal(normalized.judgements![0].startTimeMs, undefined);
  assert.equal(normalized.judgements![0].missAnalysis?.reason, "Unknown");
  assert.equal(normalized.judgements![0].missAnalysis?.confidence, undefined);
  assert.equal(normalized.judgements![0].missAnalysis?.pressTimeOffsetMs, null);
});

test("all public replay fetch boundaries normalize uploaded analysis", async t => {
  t.mock.method(globalThis, "fetch", async (url: string) => {
    const replay = { shareId: "test", analysis };
    return new Response(JSON.stringify(url.includes("/community?") ? { items: [replay] }
      : url.includes("/profiles/") ? { recentReplays: [replay] } : replay));
  });
  const replays = [await fetchOsuReplay("test"), ...(await fetchOsuCommunity()), ...(await fetchOsuProfile("player")).recentReplays];
  for (const replay of replays) assert.equal(replay.analysis?.judgements?.[0].missAnalysis?.reason, "LateClick");
});

test("a shared score without an attachment can use its matching official replay", async t => {
  const item = { shareId: "test", onlineScoreId: 123, osuUserId: 4, beatmapId: 42, ruleset: "osu", hasReplayFile: false };
  t.mock.method(globalThis, "fetch", async (url: string) => new Response(JSON.stringify(url.includes("official-scores")
    ? { item, replay: { exists: true } } : item)));
  const result = await fetchOsuReplay("test");
  assert.equal(result.officialReplayExists, true);
  assert.equal(result.officialScoreId, "123");
  assert.equal(result.hasReplayFile, false);
});

test("an official replay from a different player cannot be attached to a shared score", async t => {
  const item = { shareId: "test", onlineScoreId: 123, osuUserId: 4, beatmapId: 42, ruleset: "osu", hasReplayFile: false };
  t.mock.method(globalThis, "fetch", async (url: string) => new Response(JSON.stringify(url.includes("official-scores")
    ? { item: { ...item, osuUserId: 5 }, replay: { exists: true } } : item)));
  assert.equal((await fetchOsuReplay("test")).officialReplayExists, undefined);
});
