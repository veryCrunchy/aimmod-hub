import assert from "node:assert/strict";
import { test } from "node:test";
import { computeModDifficulty } from "replayviewer-js";
import { parseOsuPlayback } from "../src/lib/osuPlaybackDecode";
import { osuPlaybackAudioUrl } from "../src/lib/osuPlayback";
import { playbackAnalysis } from "../src/lib/osuPlaybackAnalysis";
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

test("browser judgements retain misses and slider breaks without inventing miss timing or causes", async () => {
  const parsed = await parseOsuPlayback(exactBuffer(await createPlaybackReplay()), new TextEncoder().encode(playbackBeatmap).buffer);
  const analysis = playbackAnalysis([
    { objectIndex: 0, judgement: 0, time: 1300, x: 0, y: 0, hitSound: 0, comboBreak: true },
    { objectIndex: 1, judgement: 0, time: 2400, x: 0, y: 0, hitSound: 0, comboBreak: true, isSliderSub: true },
    { objectIndex: 1, judgement: 300, time: 2300, x: 0, y: 0, hitSound: 0, comboBreak: false, isSliderSub: true },
  ], parsed.beatmap);
  assert.equal(analysis.judgements?.length, 2);
  assert.equal(analysis.judgements?.[0].result, "Miss");
  assert.equal(analysis.judgements?.[0].startTimeMs, parsed.beatmap.hitObjects[0].time);
  assert.equal(analysis.judgements?.[0].timeOffsetMs, undefined);
  assert.equal(analysis.judgements?.[0].missAnalysis, undefined);
  assert.equal(analysis.judgements?.[1].result, "SliderBreak");
  assert.equal(analysis.judgements?.[1].startTimeMs, 2400);
});
