import { test } from "node:test";
import assert from "node:assert/strict";
import { gameForPath } from "../src/lib/hubGame";
import { groupOsuPlays } from "../src/lib/osuDirectory";
import type { OsuSharedReplay } from "../src/lib/osuCommunity";

test("explicit game routes take precedence and shared account pages remain neutral", () => {
  for (const path of ["/osu", "/osu/replays/a", "/osu/players", "/app/osu"]) assert.equal(gameForPath(path), "osu");
  for (const path of ["/", "/profiles/a", "/app/kovaaks", "/learn", "/replays"]) assert.equal(gameForPath(path), "kovaaks");
  for (const path of ["/account", "/app", "/link-device", "/osumaps"]) assert.equal(gameForPath(path), null);
});

test("beatmap directories preserve separate difficulties and missing PP", () => {
  const plays = [
    { beatmapId: 10, beatmapSetId: 1, hubHandle: "a", playedAt: "2026-09-01", performancePoints: null },
    { beatmapId: 11, beatmapSetId: 1, hubHandle: "a", playedAt: "2026-09-02", performancePoints: 150 },
    { beatmapId: 11, beatmapSetId: 1, hubHandle: "b", playedAt: "2026-09-03", performancePoints: 170 },
    { beatmapId: 0, beatmapSetId: 0, hubHandle: "b", playedAt: "2026-09-04", performancePoints: 20 },
  ] as OsuSharedReplay[];
  const maps = groupOsuPlays(plays, "beatmaps");
  assert.equal(maps.length, 2);
  assert.equal(maps[0].bestPP, null);
  assert.equal(maps[1].plays.length, 2);
  assert.equal(maps[1].bestPP, 170);
  assert.equal(maps[1].latest.hubHandle, "b");
  const players = groupOsuPlays(plays, "players");
  assert.equal(players.length, 2);
  assert.equal(players[0].plays.length, 2);
  assert.equal(plays[0].playedAt, "2026-09-01");
});

test("public player groups include non-AimMod players and merge linked identities by osu ID", () => {
 const plays = [
  { osuUserId:42, hubHandle:"", beatmapId:9, playedAt:"2026-09-01", performancePoints:100 },
  { osuUserId:42, hubHandle:"example-linked", beatmapId:10, playedAt:"2026-09-02", performancePoints:200 },
  { osuUserId:43, hubHandle:"", beatmapId:11, playedAt:"2026-09-03", performancePoints:50 },
 ] as OsuSharedReplay[];
 const groups=groupOsuPlays(plays,"players");
 assert.equal(groups.length,2);
 assert.equal(groups[0].id,"42");
 assert.equal(groups[0].plays.length,2);
 assert.equal(groups[1].id,"43");
});
