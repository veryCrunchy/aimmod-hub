import assert from "node:assert/strict";
import test from "node:test";
import { ScorePpCache, scorePpCacheKey, scorePpPerformanceArgs, scorePpValidationReason, canCalculateScorePp, validateScorePpObjectCount, scorePpTTL, type ScorePpInput } from "../src/lib/scorePp";

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

test("stable actual-score args preserve counts and combo without accuracy or FC substitution", () => {
  assert.deepEqual(scorePpPerformanceArgs(input()), { mods: [], lazer: false, combo: 42, n300: 90, n100: 5, n50: 2, misses: 3, legacyTotalScore: 123456 });
});
test("lazer and Classic preserve complete mod settings and separate slider results", () => {
  for (const mods of [[{ acronym: "DT", settings: { speed_change: 1.2 } }], [{ acronym: "CL", settings: { no_slider_head_accuracy: true } }]]) {
    const args = scorePpPerformanceArgs(input({ lazer: true, mods, legacyTotalScore: null,
      statistics: { great: 90, ok: 5, meh: 2, miss: 3, large_tick_hit: 18, large_tick_miss: 4, small_tick_hit: 9, small_tick_miss: 2, slider_tail_hit: 7 } }));
    assert.deepEqual(args.mods, mods);
    assert.equal(args.largeTickHits, 18);
    assert.equal(args.smallTickHits, 9);
    assert.equal(args.sliderEndHits, 7);
    assert.equal(args.n300, 90);
    assert.equal(args.misses, 3);
  }
});
test("failed scores use only judged top-level objects for progress; sparse dictionary means zero", () => {
  const args = scorePpPerformanceArgs(input({ passed: false, lazer: true, statistics: { great: 3, miss: 1, large_tick_hit: 20, slider_tail_hit: 10 } }));
  assert.equal(args.passedObjects, 4);
  assert.equal(args.n100, 0);
  assert.equal(args.n50, 0);
  assert.equal(args.smallTickHits, 0);
  assert.equal(scorePpPerformanceArgs(input()).passedObjects, undefined);
});
test("unknown scoring, absent statistics, invalid counts and checksum cannot calculate or cache", () => {
  const invalid: Partial<ScorePpInput>[] = [ { lazer: null }, { statistics: null }, { mods: null }, { beatmapChecksum: "" },
    { rulesetId: 3 }, { version: 2 }, { accuracy: NaN }, { maxCombo: -1 }, { statistics: { great: 1.5 } }, { statistics: { miss: Infinity } } ];
  for (const patch of invalid) {
    const value = input(patch);
    assert.equal(canCalculateScorePp(value), false);
    assert.ok(scorePpValidationReason(value));
    assert.equal(scorePpCacheKey(value), "");
    assert.throws(() => scorePpPerformanceArgs(value));
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
  store.setItem(key.replace("rosu4.0.1", "rosu3.0.0"), JSON.stringify({ pp: 30, expires: 1000 }));
  assert.equal(cache.get(input()), undefined);
});
test("disabled storage is optional", () => {
  const store = { ...storage(), getItem: () => { throw new Error("disabled"); }, setItem: () => { throw new Error("full"); } };
  const cache = new ScorePpCache(store);
  assert.equal(cache.get(input()), undefined);
  assert.doesNotThrow(() => cache.set(input(), 10));
});
test("safe integer total scores can exceed uint32; rosu legacy u32 limitation is explicit", () => {
  assert.equal(canCalculateScorePp(input({ totalScore: 2 ** 40 })), true);
  assert.equal(canCalculateScorePp(input({ totalScore: Number.MAX_SAFE_INTEGER + 1 })), false);
  assert.match(scorePpValidationReason(input({ legacyTotalScore: 2 ** 40 }))!, /engine's supported range/);
});
test("passed scores must account for every map object and fails cannot exceed object count", () => {
  assert.doesNotThrow(() => validateScorePpObjectCount(input(), 100));
  assert.throws(() => validateScorePpObjectCount(input({ statistics: {} }), 100));
  assert.throws(() => validateScorePpObjectCount(input(), 99));
  assert.throws(() => validateScorePpObjectCount(input(), 101));
  assert.doesNotThrow(() => validateScorePpObjectCount(input({ passed: false }), 101));
  assert.throws(() => validateScorePpObjectCount(input({ passed: false }), 99));
});
test("actual rosu retains the supplied miss/count state rather than generating an FC", async () => {
  const rosu = await import("rosu-pp-js");
  const text = "osu file format v14\n\n[General]\nMode:0\n\n[Difficulty]\nHPDrainRate:5\nCircleSize:4\nOverallDifficulty:8\nApproachRate:9\nSliderMultiplier:1.4\nSliderTickRate:1\n\n[TimingPoints]\n0,500,4,2,1,50,1,0\n\n[HitObjects]\n" + Array.from({ length: 100 }, (_, i) => `${64 + i % 2 * 320},192,${1000 + i * 150},1,0,0:0:0:0:`).join("\n");
  const map = new rosu.Beatmap(text);
  try {
    const calculator = new rosu.Performance(scorePpPerformanceArgs(input()));
    try {
      const result = calculator.calculate(map);
      try {
        const state = result.state;
        assert.equal(state.n300, 90);
        assert.equal(state.n100, 5);
        assert.equal(state.n50, 2);
        assert.equal(state.misses, 3);
        assert.equal(state.maxCombo, 42);
        assert.ok(Number.isFinite(result.pp) && result.pp >= 0);
      } finally { result.free(); }
    } finally { calculator.free(); }
  } finally { map.free(); }
});


test("full modded map stars stay constant across fails while PP follows actual progress", async () => {
  const rosu = await import("rosu-pp-js");
  const { calculateScorePp } = await import("../src/lib/scorePpCalculation");
  const { formatScorePp } = await import("../src/lib/scorePp");
  const text = "osu file format v14\n\n[General]\nMode:0\n\n[Difficulty]\nHPDrainRate:5\nCircleSize:4\nOverallDifficulty:8\nApproachRate:9\nSliderMultiplier:1.4\nSliderTickRate:1\n\n[TimingPoints]\n0,500,4,2,1,50,1,0\n\n[HitObjects]\n" + Array.from({ length: 100 }, (_, i) => `${64 + i % 2 * 320},192,${1000 + i * 150},1,0,0:0:0:0:`).join("\n");
  const map = new rosu.Beatmap(text);
  try {
    const early = input({ passed: false, maxCombo: 1, statistics: { great: 1 }, mods: [{ acronym: "DT" }], legacyTotalScore: null });
    const later = { ...early, maxCombo: 90, statistics: { great: 90 } };
    const first = calculateScorePp(rosu, map, early), last = calculateScorePp(rosu, map, later);
    const normal = calculateScorePp(rosu, map, { ...later, mods: [] });
    assert.equal(first.objectCount, 100);
    assert.equal(first.stars, last.stars);
    assert.ok(last.stars! > normal.stars!);
    assert.ok(last.pp > first.pp);
    assert.equal(formatScorePp(0.2), "<1pp");
    assert.equal(formatScorePp(0), "0pp");
    const cache = new ScorePpCache(storage()); cache.setResult(early, first);
    assert.deepEqual(cache.getResult(early), first);
  } finally { map.free(); }
});
