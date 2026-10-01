import { test } from "node:test";
import assert from "node:assert/strict";
import type { LiveHubActivity } from "../src/lib/api";
import { liveTimer, liveView, ownSessionNote, sortLive } from "../src/lib/liveActivity";

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
