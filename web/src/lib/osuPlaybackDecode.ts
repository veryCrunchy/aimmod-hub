import { applyStacking, computeModDifficulty, md5, parseBeatmap, parseReplay } from "replayviewer-js";
import type { ParsedOsuPlayback } from "./osuPlayback";

export async function parseOsuPlayback(replayBytes: ArrayBuffer, beatmapBytes: ArrayBuffer): Promise<ParsedOsuPlayback> {
  if (replayBytes.byteLength > 64 * 1024 * 1024 || beatmapBytes.byteLength > 4 * 1024 * 1024) throw new Error("This file exceeds the playback size limit.");
  const replay = await parseReplay(replayBytes);
  if (replay.mode !== 0) throw new Error("This player currently supports osu!standard replays.");
  if (!replay.frames.length || replay.frames.length > 2_000_000) throw new Error("This replay does not contain a supported input timeline.");
  if (!/^[a-f0-9]{32}$/i.test(replay.beatmapHash) || md5(new Uint8Array(beatmapBytes)).toLowerCase() !== replay.beatmapHash.toLowerCase()) {
    throw new Error("This beatmap has changed since the replay was recorded. Choose the original .osu file to watch this play.");
  }
  const beatmap = parseBeatmap(new TextDecoder().decode(beatmapBytes));
  if (beatmap.mode !== 0 || !beatmap.hitObjects.length || beatmap.hitObjects.length > 25000) throw new Error("This beatmap cannot be played here.");
  beatmap.rawOsu = new Uint8Array(beatmapBytes);
  applyStacking(beatmap, computeModDifficulty(beatmap, replay));
  return { replay, beatmap };
}
