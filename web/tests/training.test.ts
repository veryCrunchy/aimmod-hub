import assert from "node:assert/strict";
import { test } from "node:test";
import { fetchTraining, formatPracticeTime, setTrainingVisibility, timingChange, trainingActivityDays } from "../src/lib/training";
test("practice timeline preserves inactive UTC days", () => {
  const days = trainingActivityDays([{ date: "2026-08-31", sessions: 2, seconds: 120 }], 3, new Date("2026-09-02T01:00:00Z"));
  assert.deepEqual(days.map(day => day.date), ["2026-08-30", "2026-08-31", "2026-09-01", "2026-09-02"]);
  assert.deepEqual(days.map(day => day.seconds), [0, 120, 0, 0]);
});
test("training time and improvement direction stay unambiguous", () => {
  assert.equal(formatPracticeTime(3599), "59m"); assert.equal(formatPracticeTime(3660), "1h 1m");
  assert.equal(timingChange(-3.2), "3.2 ms tighter"); assert.equal(timingChange(4), "4.0 ms wider"); assert.equal(timingChange(0.001), "Timing unchanged");
});
test("public lookup omits credentials while owner and sharing requests authenticate", async () => {
  const original = globalThis.fetch; const calls: { url: string; options?: RequestInit }[] = [];
  globalThis.fetch = async (url, options) => { calls.push({ url: String(url), options }); return new Response(JSON.stringify({ sessions: 0 }), { status: 200 }); };
  try {
    await fetchTraining("practice/player", 90, "aim"); await fetchTraining("", 30, "", true); await setTrainingVisibility("synthetic-session", "private");
    assert.ok(calls[0].url.includes("practice%2Fplayer?days=90&mode=aim")); assert.equal(calls[0].options?.credentials, "omit");
    assert.equal(calls[1].options?.credentials, "include"); assert.equal(calls[2].options?.method, "POST");
    assert.deepEqual(JSON.parse(String(calls[2].options?.body)), { id: "synthetic-session", visibility: "private" });
  } finally { globalThis.fetch = original; }
});
test("failed training requests remain errors rather than empty progress", async () => {
  const original = globalThis.fetch; globalThis.fetch = async () => new Response("unavailable", { status: 503 });
  try { await assert.rejects(fetchTraining("practice-player", 30, ""), /could not load/); await assert.rejects(setTrainingVisibility("synthetic", "public"), /could not be changed/); }
  finally { globalThis.fetch = original; }
});
