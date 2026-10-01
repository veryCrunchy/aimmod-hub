import { test } from "node:test";
import assert from "node:assert/strict";
import type { LiveHubActivity } from "../src/lib/api";
import { activityLabel, isLiveHealthy, liveClient, liveHealthDetail, liveSessionDetail, liveTimer, liveView, ownSessionNote, sortLive } from "../src/lib/liveActivity";

const now = Date.parse("2026-06-01T12:00:10Z");
const updatedAt = "2026-06-01T12:00:00Z";

test("timers count down from the last update unless paused", () => {
  assert.equal(liveTimer({ active: true, updatedAt, timeRemainingSecs: 40 }, now), "0:30 left");
  assert.equal(liveTimer({ active: true, updatedAt, timeRemainingSecs: 40, paused: true }, now), "0:40 left");
  assert.equal(liveTimer({ active: true, updatedAt, elapsedSecs: 50 }, now), "1:00 in");
  assert.equal(liveTimer({ active: true, updatedAt }, now), null);
});

test("players in menus show no scenario numbers", () => {
  const view = liveView({ active: true, userHandle: "demo-a", gameState: "In menus", score: 100 }, now);
  assert.equal(view.phase, "idle");
  assert.equal(view.title, "In menus");
  assert.equal(view.score, null);
  assert.equal(view.name, "demo-a");
});

test("players in a scenario come first, newest update first", () => {
  const items: LiveHubActivity[] = [
    { active: true, userHandle: "menu", updatedAt: "2026-06-01T12:00:09Z" },
    { active: true, userHandle: "old", scenarioName: "Synthetic A", updatedAt: "2026-06-01T11:59:00Z" },
    { active: true, userHandle: "new", scenarioName: "Synthetic B", updatedAt: "2026-06-01T12:00:05Z" },
    { active: true, userHandle: "paused", scenarioName: "Synthetic C", paused: true, updatedAt: "2026-06-01T12:00:08Z" },
  ];
  assert.deepEqual(sortLive(items).map((i) => i.userHandle), ["new", "old", "paused", "menu"]);
});

test("connection notes are only for a real dropped connection", () => {
  assert.equal(ownSessionNote({ active: true, runtimeLoaded: true, bridgeConnected: true }), null);
  assert.equal(ownSessionNote({ active: true, runtimeLoaded: false, bridgeConnected: false }), null, "senders that omit the fields say nothing");
  assert.match(ownSessionNote({ active: true, runtimeLoaded: true, bridgeConnected: false }) ?? "", /Reconnecting/);
});

const base: LiveHubActivity = { active: true, userHandle: "synthetic-player" };

test("companion payloads use plain connection wording", () => {
  const healthy = { ...base, runtimeLoaded: true, bridgeConnected: true };
  assert.equal(liveClient(healthy), "companion");
  assert.equal(isLiveHealthy(healthy), true);
  assert.equal(liveHealthDetail(healthy), null);
  const dropped = { ...base, runtimeLoaded: true, bridgeConnected: false };
  assert.equal(liveHealthDetail(dropped), "Reconnecting to your game");
  assert.doesNotMatch(ownSessionNote(dropped) ?? "", /bridge|runtime/i);
  // Responses from a Hub without the computed flag still work.
  assert.equal(isLiveHealthy({ ...base }), true);
});

test("in-game mod payloads use mod wording and the computed health flag", () => {
  const mod = { ...base, client: "in-game", runtimeLoaded: false, bridgeConnected: false, healthy: false };
  assert.equal(liveClient(mod), "in-game");
  assert.equal(liveHealthDetail(mod), "AimMod is starting in KovaaK's");
  assert.equal(liveHealthDetail({ ...mod, runtimeLoaded: true }), "AimMod is reconnecting to KovaaK's");
  assert.equal(liveHealthDetail({ ...mod, runtimeLoaded: true, bridgeConnected: true, healthy: true }), null);
  assert.doesNotMatch(liveHealthDetail(mod) ?? "", /bridge|runtime/i);
  // Unlike the companion, the mod always reports its checks, so both false is a real state.
  assert.match(ownSessionNote(mod) ?? "", /^AimMod is starting in KovaaK's\./);
  assert.match(ownSessionNote({ ...mod, runtimeLoaded: true }) ?? "", /^AimMod is reconnecting to KovaaK's\./);
});

test("session progress is shown only when sent", () => {
  assert.equal(liveSessionDetail(base), null);
  assert.equal(liveSessionDetail({ ...base, sessionRunCount: 0 }), "0 runs this session");
  assert.equal(liveSessionDetail({ ...base, sessionRunCount: 1 }), "1 run this session");
  assert.equal(liveSessionDetail({ ...base, sessionRunCount: 12 }), "12 runs this session");
  assert.equal(liveView({ ...base, sessionRunCount: 3 }, now).session, "3 runs this session");
  assert.equal(liveView(base, now).session, null);
});

test("activity kinds label players outside a scenario", () => {
  assert.equal(activityLabel(base), null);
  assert.equal(activityLabel({ ...base, activity: "replay" }), "Watching a replay");
  assert.equal(liveView({ ...base, activity: "lobby" }, now).status, "In a lobby");
  assert.equal(liveView({ ...base, activity: "lobby" }, now).title, "In a lobby");
  assert.equal(liveView(base, now).status, "In menus");
  assert.equal(liveView({ ...base, scenarioName: "Synthetic A", activity: "scenario" }, now).status, "Playing");
});
