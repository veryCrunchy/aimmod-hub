import assert from "node:assert/strict";
import { test } from "node:test";
import { computeModDifficulty } from "replayviewer-js";
import { parseOsuPlayback } from "../src/lib/osuPlaybackDecode";
import { osuPlaybackAudioUrl } from "../src/lib/osuPlayback";
import { createPlaybackReplay, playbackBeatmap } from "./fixtures/osuPlaybackFixture";

const exactBuffer = (bytes: Uint8Array) => Uint8Array.from(bytes).buffer;

test("song requests identify the exact difficulty, set and replay checksum", () => {
  const checksum = "A".repeat(32);
  assert.ok(osuPlaybackAudioUrl(42, 12, checksum)?.endsWith(`/beatmaps/42/audio?beatmapsetId=12&checksum=${checksum.toLowerCase()}`));
  assert.equal(osuPlaybackAudioUrl(42, undefined, checksum), undefined);
  assert.equal(osuPlaybackAudioUrl(42, -1, checksum), undefined);
  assert.equal(osuPlaybackAudioUrl(1.5, 12, checksum), undefined);
  assert.equal(osuPlaybackAudioUrl(42, 12, "wrong"), undefined);
});

test("actual LZMA replay inputs and curved repeating sliders survive decoding", async () => {
  const result = await parseOsuPlayback(exactBuffer(await createPlaybackReplay()), new TextEncoder().encode(playbackBeatmap).buffer);
  assert.equal(result.replay.username, "AimMod QA");
  assert.ok(result.replay.frames.length > 500);
  assert.ok(result.replay.frames.some(frame => frame.keys !== 0));
  assert.equal(result.beatmap.hitObjects.length, 5);
  assert.deepEqual(result.beatmap.hitObjects.filter(object => object.type === "slider").map(object => [object.curveType, object.slides]), [["B", 1], ["P", 2]]);
});

test("an updated or different difficulty is refused instead of showing a false replay", async () => {
  await assert.rejects(parseOsuPlayback(exactBuffer(await createPlaybackReplay()), new TextEncoder().encode(playbackBeatmap.replace("CircleSize:4", "CircleSize:5")).buffer), /changed since the replay/);
});

test("replay rate and HR come from the actual replay mod data", async () => {
  const result = await parseOsuPlayback(exactBuffer(await createPlaybackReplay(16 | 64)), new TextEncoder().encode(playbackBeatmap).buffer);
  const difficulty = computeModDifficulty(result.beatmap, result.replay);
  assert.equal(difficulty.speed, 1.5);
  assert.equal(difficulty.isHR, true);
});
