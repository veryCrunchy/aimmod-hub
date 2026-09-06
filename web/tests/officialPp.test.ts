import assert from "node:assert/strict";
import test from "node:test";
import { calculateOfficialPp, ppEngine } from "../src/lib/officialPp";

test("FC requests keep scoring and combined mods; stale engines are rejected", async () => {
  const original = globalThis.fetch;
  let engine = ppEngine;
  const requests: Record<string, any>[] = [];
  globalThis.fetch = async (_url, init) => {
    requests.push(JSON.parse(String(init?.body)));
    return new Response(JSON.stringify({ engine, pp: 100, maxPp: 120, stars: 4, objectCount: 100 }));
  };
  try {
    for (const lazer of [true, false]) await calculateOfficialPp(new Uint8Array([1, 2, 3]), "a".repeat(32), { accuracy: 98, mods: "HDHR", lazer });
    assert.deepEqual(requests.map(request => request.lazer), [true, false]);
    assert.deepEqual(requests[0].mods, [{ acronym: "HD" }, { acronym: "HR" }]);
    assert.equal(requests[0].map, "AQID");
    engine = "old-engine";
    await assert.rejects(calculateOfficialPp(new Uint8Array(), "", { accuracy: 98, mods: "NM", lazer: true }), /invalid result/);
  } finally { globalThis.fetch = original; }
});

test("actual requests preserve lazer slider judgements and custom mod settings", async () => {
  const original = globalThis.fetch;
  const input = { version: 1, beatmapId: 123, beatmapChecksum: "a".repeat(32), rulesetId: 0, lazer: true,
    mods: [{ acronym: "DT", settings: { speed_change: 1.2 } }], statistics: { great: 90, miss: 10, slider_tail_hit: 20, large_tick_miss: 2 },
    maximumStatistics: { great: 100 }, maxCombo: 42, accuracy: 0.9, passed: true, totalScore: 123456, legacyTotalScore: null };
  globalThis.fetch = async (_url, init) => {
    const sent = JSON.parse(String(init?.body));
    assert.deepEqual(sent.input, input);
    return new Response(JSON.stringify({ error: "Exact map unavailable" }), { status: 400 });
  };
  try { await assert.rejects(calculateOfficialPp(new Uint8Array(), input.beatmapChecksum, input), /Exact map unavailable/); }
  finally { globalThis.fetch = original; }
});
