import assert from "node:assert/strict";
import test from "node:test";
import { ScorePpCache, scorePpCacheKey, scorePpValidationReason, canCalculateScorePp, validateScorePpObjectCount, scorePpTTL, isUnsupportedScorePpRuleset, type ScorePpInput } from "../src/lib/scorePp";

function input(overrides: Partial<ScorePpInput> = {}): ScorePpInput {
  return { version: 1, beatmapId: 123, beatmapChecksum: "a".repeat(32), rulesetId: 0,
    lazer: false, mods: [], statistics: { great: 90, ok: 5, meh: 2, miss: 3 }, maximumStatistics: { great: 100 },
    maxCombo: 42, accuracy: 0.9266, passed: true, totalScore: 123456, legacyTotalScore: 123456, ...overrides };
}
function storage() {
  const entries = new Map<string, string>();
  return { entries, get length() { return entries.size; }, key: (i: number) => [...entries.keys()][i] ?? null,
    getItem: (key: string) => entries.get(key) ?? null, setItem: (key: string, value: string) => { entries.set(key, value); }, removeItem: (key: string) => { entries.delete(key); } };
}

test("unsupported modes are distinguished from invalid standard score data", () => {
  for (const rulesetId of [1, 2, 3]) {
    assert.equal(isUnsupportedScorePpRuleset(input({ rulesetId })), true);
    assert.equal(canCalculateScorePp(input({ rulesetId })), false);
  }
  assert.equal(isUnsupportedScorePpRuleset(input()), false);
  assert.equal(isUnsupportedScorePpRuleset(input({ version: 2 })), false);
  assert.equal(isUnsupportedScorePpRuleset(input({ rulesetId: 99 })), false);
});

test("unknown scoring, absent statistics, invalid counts and checksum cannot calculate or cache", () => {
  const invalid: Partial<ScorePpInput>[] = [ { lazer: null }, { statistics: null }, { mods: null }, { beatmapChecksum: "" },
    { rulesetId: 3 }, { version: 2 }, { accuracy: NaN }, { maxCombo: -1 }, { statistics: { great: 1.5 } }, { statistics: { miss: Infinity } } ];
  for (const patch of invalid) {
    const value = input(patch);
    assert.equal(canCalculateScorePp(value), false);
    assert.ok(scorePpValidationReason(value));
    assert.equal(scorePpCacheKey(value), "");
  }
});
test("cache key tracks all raw score inputs and mod settings, ignores identities and property order", () => {
  const base = input();
  const key = scorePpCacheKey(base);
  for (const patch of [{ beatmapChecksum: "b".repeat(32) }, { maxCombo: 43 }, { accuracy: 0.99 }, { totalScore: 999 }, { legacyTotalScore: null },
    { lazer: true }, { passed: false }, { mods: [{ acronym: "DT", settings: { speed_change: 1.2 } }] },
    { statistics: { great: 89, ok: 6, meh: 2, miss: 3 } }, { maximumStatistics: { great: 101 } }]) {
    assert.notEqual(scorePpCacheKey(input(patch)), key);
  }
  assert.equal(scorePpCacheKey(input({ beatmapChecksum: "A".repeat(32), statistics: { miss: 3, meh: 2, ok: 5, great: 90 } })), key);
  assert.equal(scorePpCacheKey({ ...base, userName: "private-user", url: "https://example.invalid" } as ScorePpInput), key);
  assert.equal(key.includes("private-user"), false);
});
test("cache accepts zero, never stores errors/nonfinite/negative PP and expires absolutely after 24 hours", () => {
  const store = storage();
  let now = 100;
  const cache = new ScorePpCache(store, () => now);
  cache.set(input(), 0);
  assert.equal(cache.get(input()), 0);
  for (const pp of [-1, NaN, Infinity, { error: "failed" } as unknown as number]) cache.set(input(), pp);
  assert.equal(cache.get(input()), 0);
  now += scorePpTTL;
  assert.equal(cache.get(input()), undefined);
  assert.equal(store.length, 0);
});
test("cache is bounded to 400 entries and rejects malformed, future and engine-mismatched entries", () => {
  const store = storage();
  const cache = new ScorePpCache(store, () => 100);
  for (let i = 0; i < 405; i++) cache.set(input({ maxCombo: i }), i);
  assert.equal(store.length, 400);
  assert.equal(cache.get(input({ maxCombo: 0 })), undefined);
  assert.equal(cache.get(input({ maxCombo: 404 })), 404);
  const key = scorePpCacheKey(input());
  for (const raw of ["bad-json", JSON.stringify({ pp: 20, expires: 100 + scorePpTTL + 1 }), JSON.stringify({ pp: -1, expires: 1000 })]) {
    store.setItem(key, raw);
    assert.equal(cache.get(input()), undefined);
  }
  store.removeItem(key);
  store.setItem(key.replace("aimmod-osu-2026.730.0-v1", "rosu4.0.1"), JSON.stringify({ pp: 30, expires: 1000 }));
  assert.equal(cache.get(input()), undefined);
});
test("disabled storage is optional", () => {
  const store = { ...storage(), getItem: () => { throw new Error("disabled"); }, setItem: () => { throw new Error("full"); } };
  const cache = new ScorePpCache(store);
  assert.equal(cache.get(input()), undefined);
  assert.doesNotThrow(() => cache.set(input(), 10));
});
test("safe integer total and legacy scores are retained", () => {
  assert.equal(canCalculateScorePp(input({ totalScore: 2 ** 40 })), true);
  assert.equal(canCalculateScorePp(input({ totalScore: Number.MAX_SAFE_INTEGER + 1 })), false);
  assert.equal(canCalculateScorePp(input({ legacyTotalScore: 2 ** 40 })), true);
});
test("passed scores must account for every map object and fails cannot exceed object count", () => {
  assert.doesNotThrow(() => validateScorePpObjectCount(input(), 100));
  assert.throws(() => validateScorePpObjectCount(input({ statistics: {} }), 100));
  assert.throws(() => validateScorePpObjectCount(input(), 99));
  assert.throws(() => validateScorePpObjectCount(input(), 101));
  assert.doesNotThrow(() => validateScorePpObjectCount(input({ passed: false }), 101));
  assert.throws(() => validateScorePpObjectCount(input({ passed: false }), 99));
});
