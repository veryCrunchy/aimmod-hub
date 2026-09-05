import assert from "node:assert/strict";
import test from "node:test";
import { replayHeaderScorePp } from "../src/lib/replayHeaderPp";
import { scorePpPerformanceArgs } from "../src/lib/scorePp";

const header = { mode: 0, gameVersion: 20200101, beatmapHash: "a".repeat(32), count300: 90, count100: 5, count50: 2, countMiss: 3, maxCombo: 42, score: 123456, mods: 8 };
const context = { beatmapId: 1, objectCount: 100, passed: true };
test("stable header supplies exact counts/combo/mods without retaining replay identity", () => {
  const result = replayHeaderScorePp({ ...header, username: "Synthetic player" } as typeof header, context);
  assert.ok(result.input);
  assert.equal(result.input.lazer, false);
  assert.deepEqual(result.input.mods, [{ acronym: "HD" }]);
  assert.deepEqual(result.input.statistics, { great: 90, ok: 5, meh: 2, miss: 3 });
  assert.equal(result.input.maxCombo, 42);
  assert.equal(result.input.legacyTotalScore, 123456);
  assert.equal(JSON.stringify(result.input).includes("Synthetic player"), false);
});
test("Nightcore and Perfect do not double-apply their base bit flags", () => {
  const result = replayHeaderScorePp({ ...header, mods: 512 | 64 | 16384 | 32 }, context);
  assert.deepEqual(result.input?.mods, [{ acronym: "NC" }, { acronym: "PF" }]);
});
test("failed progress preserves header results and does not infer pass from perfect flag", () => {
  const result = replayHeaderScorePp(header, { ...context, passed: false, objectCount: 200 });
  assert.ok(result.input);
  assert.equal(scorePpPerformanceArgs(result.input).passedObjects, 100);
  assert.equal(replayHeaderScorePp(header, { ...context, objectCount: 200 }).input, undefined);
  assert.equal(replayHeaderScorePp(header, { ...context, passed: false, objectCount: 99 }).input, undefined);
});
test("lazer exports and appended settings are explicitly outside stable fallback", () => {
  for (const value of [{ ...header, gameVersion: 30000000 }, { ...header, gameVersion: 30000019 }, { ...header, scoreInfo: { mods: [] } }]) {
    const result = replayHeaderScorePp(value, context);
    assert.equal(result.input, undefined);
    assert.match(result.reason!, /full lazer score statistics and mod settings/);
  }
});
test("unknown mods, conflicting mods, corrupt counts and missing hashes never produce PP inputs", () => {
  for (const patch of [{ mods: 128 }, { mods: 536870912 }, { mods: 2 ** 32 }, { mods: 64 | 256 }, { mods: 2 | 16 },
    { beatmapHash: "" }, { count300: -1 }, { countMiss: 1.5 }, { maxCombo: 65536 }, { score: -1 }, { gameVersion: 0 }, { mode: 3 }]) {
    assert.equal(replayHeaderScorePp({ ...header, ...patch }, context).input, undefined);
  }
});
test("zero counts cannot silently generate an SS", () => {
  assert.equal(replayHeaderScorePp({ ...header, count300: 0, count100: 0, count50: 0, countMiss: 0 }, context).input, undefined);
});
