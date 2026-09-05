import { test } from "node:test";
import assert from "node:assert/strict";
import { BeatmapDifficulty, Ruleset } from "../src/gen/aimmod/osu/v1/osu_pb";
import { PpTargetCache, candidateKey, candidatePrefix, calculationPrefix, candidateTTL, readPpSettings } from "../src/lib/ppTargetCache";

class MemoryStorage {
  entries = new Map<string, string>();
  get length() { return this.entries.size; }
  key(index: number) { return [...this.entries.keys()][index] ?? null; }
  getItem(key: string) { return this.entries.get(key) ?? null; }
  setItem(key: string, value: string) { this.entries.set(key, value); }
  removeItem(key: string) { this.entries.delete(key); }
}
const settings = readPpSettings(new URLSearchParams());
const map = (checksum = "a".repeat(32)) => new BeatmapDifficulty({ beatmapId: "42", beatmapsetId: "12", ruleset: Ruleset.OSU, stars: 4, checksum, name: "Test" });
const result = { pp: 123, maxPp: 145, stars: 4.5 };

test("candidate discovery survives a new cache instance with protobuf defaults and absolute expiry", () => {
  const storage = new MemoryStorage(); let now = 1000;
  const cache = new PpTargetCache(storage, () => now); const key = candidateKey(settings);
  cache.setCandidates(key, [map()]);
  now += candidateTTL - 1;
  const restored = new PpTargetCache(storage, () => now).getCandidates(key)!;
  assert.ok(restored[0] instanceof BeatmapDifficulty);
  assert.equal(restored[0].title, ""); assert.equal(restored[0].checksum, "a".repeat(32));
  now++; assert.equal(cache.getCandidates(key), undefined);
});

test("candidate identity isolates query and star bounds, not calculation options", () => {
  assert.notEqual(candidateKey(settings), candidateKey({ ...settings, query: "other" }));
  assert.notEqual(candidateKey(settings), candidateKey({ ...settings, high: "8" }));
  assert.equal(candidateKey(settings), candidateKey({ ...settings, query: "  " }));
  const storage = new MemoryStorage(); const cache = new PpTargetCache(storage);
  cache.setCandidates(candidateKey(settings), []);
  assert.deepEqual(cache.getCandidates(candidateKey(settings)), []);
});

test("new checksum discovery evicts other searches containing the old revision", () => {
  const storage = new MemoryStorage(); const cache = new PpTargetCache(storage);
  const first = candidateKey(settings), second = candidateKey({ ...settings, query: "new" });
  cache.setCandidates(first, [map()]); cache.setResult(map(), settings, result);
  cache.setCandidates(second, [map("b".repeat(32))]);
  assert.equal(cache.getCandidates(first), undefined);
  assert.equal(cache.getCandidates(second)![0].checksum, "b".repeat(32));
  assert.equal(cache.getResult(map("b".repeat(32)), settings), undefined);
});

test("calculation reuse requires exact checksum, engine namespace and every scoring parameter", () => {
  const storage = new MemoryStorage(); let now = 1000; const cache = new PpTargetCache(storage, () => now);
  cache.setResult(map(), settings, result);
  assert.deepEqual(new PpTargetCache(storage, () => now).getResult(map(), settings), result);
  for (const changed of [{ ...settings, accuracy: 99 }, { ...settings, mods: "HD" }, { ...settings, lazer: false }]) assert.equal(cache.getResult(map(), changed), undefined);
  assert.equal(cache.getResult(map("b".repeat(32)), settings), undefined);
  const key = [...storage.entries.keys()][0];
  storage.setItem(key.replace(calculationPrefix, "aimmod-pp-other-engine:"), storage.getItem(key)!);
  storage.removeItem(key); assert.equal(cache.getResult(map(), settings), undefined);
  cache.setResult(map(), settings, result); now += 86400_000; assert.equal(cache.getResult(map(), settings), undefined);
});

test("missing checksums, error results and invalid numbers never become successful cache entries", () => {
  const storage = new MemoryStorage(); const cache = new PpTargetCache(storage);
  cache.setResult(map(""), settings, result);
  cache.setResult(map(), settings, { ...result, error: "Map changed" });
  cache.setResult(map(), settings, { ...result, pp: NaN });
  cache.setResult(map(), settings, { ...result, maxPp: -1 });
  assert.equal(storage.length, 0);
  cache.setCandidates(candidateKey(settings), [map("")]);
  assert.equal(cache.getCandidates(candidateKey(settings))![0].checksum, "");
  assert.equal(cache.getResult(map(""), settings), undefined);
});

test("storage is bounded and never evicts unrelated application data", () => {
  const storage = new MemoryStorage(); let now = 1;
  const cache = new PpTargetCache(storage, () => now++);
  storage.setItem("user-preference", "keep");
  for (let i = 0; i < 25; i++) cache.setCandidates(candidateKey({ ...settings, query: String(i) }), [map()]);
  assert.equal([...storage.entries.keys()].filter(key => key.startsWith(candidatePrefix)).length, 16);
  for (let i = 0; i < 410; i++) cache.setResult(new BeatmapDifficulty({ ...map(), beatmapId: String(i + 1) }), settings, result);
  assert.equal([...storage.entries.keys()].filter(key => key.startsWith(calculationPrefix)).length, 400);
  assert.equal(storage.getItem("user-preference"), "keep");
  assert.ok([...storage.entries.values()].reduce((sum, raw) => sum + raw.length * 2, 0) < 2.5 * 1024 * 1024);
});

test("malformed cache and disabled storage fall back to discovery", () => {
  const storage = new MemoryStorage(); const cache = new PpTargetCache(storage, () => 1000); const key = candidateKey(settings);
  for (const raw of ["{", "null", JSON.stringify({ version: 1, expires: 9999999999, maps: [] }), JSON.stringify({ version: 1, expires: 1100, maps: [{ beatmapId: "../42" }] })]) {
    storage.setItem(key, raw); assert.equal(cache.getCandidates(key), undefined);
  }
  const blocked = new PpTargetCache({ get length() { throw Error("blocked"); }, key() { throw Error("blocked"); }, getItem() { throw Error("blocked"); }, setItem() { throw Error("quota"); }, removeItem() { throw Error("blocked"); } });
  assert.doesNotThrow(() => { blocked.setCandidates(key, [map()]); blocked.setResult(map(), settings, result); });
  assert.equal(blocked.getCandidates(key), undefined);
});

test("URL state validates numeric ranges, mods and scoring without losing encoded queries", () => {
  assert.deepEqual(readPpSettings(new URLSearchParams("q=a%26b&min=8&max=2&acc=99.14&mods=HDDT&scoring=stable&sort=stars")), { query: "a&b", low: "2", high: "8", accuracy: 99.1, mods: "HDDT", lazer: false, sort: "stars" });
  assert.deepEqual(readPpSettings(new URLSearchParams("min=NaN&max=100&acc=Infinity&mods=RX&scoring=invalid")), settings);
  assert.equal(readPpSettings(new URLSearchParams("min=&max=")).low, "");
  assert.equal(readPpSettings(new URLSearchParams("min=&max=")).high, "");
});
