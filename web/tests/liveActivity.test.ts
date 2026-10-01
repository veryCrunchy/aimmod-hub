import { test } from "node:test";
import assert from "node:assert/strict";
import type { LiveHubActivity } from "../src/lib/api";
import { isLiveHealthy, liveClient, liveHealthDetail, liveSessionDetail } from "../src/lib/liveActivity";

const base: LiveHubActivity = { active: true, userHandle: "synthetic-player" };

test("companion payloads keep their bridge wording", () => {
  const healthy = { ...base, runtimeLoaded: true, bridgeConnected: true };
  assert.equal(liveClient(healthy), "companion");
  assert.equal(isLiveHealthy(healthy), true);
  assert.equal(liveHealthDetail(healthy), null);
  assert.equal(
    liveHealthDetail({ ...base, runtimeLoaded: true, bridgeConnected: false }),
    "Bridge status: runtime loaded · bridge reconnecting",
  );
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
});

test("session progress is shown only when sent", () => {
  assert.equal(liveSessionDetail(base), null);
  assert.equal(liveSessionDetail({ ...base, sessionRunCount: 0 }), "0 runs this session");
  assert.equal(liveSessionDetail({ ...base, sessionRunCount: 1 }), "1 run this session");
  assert.equal(liveSessionDetail({ ...base, sessionRunCount: 12 }), "12 runs this session");
});
