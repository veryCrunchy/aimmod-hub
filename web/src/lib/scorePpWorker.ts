import { loadRosu } from "virtual:rosu-browser";
import { md5 } from "replayviewer-js";
import type { Beatmap } from "rosu-pp-js";
import { scorePpPerformanceArgs, validateScorePpObjectCount, validScorePp, type ScorePpWorkerRequest, type ScorePpWorkerResponse } from "./scorePp";

const maps = new Map<string, Beatmap>();
const maxMapBytes = 4 * 1024 * 1024;
let queue = Promise.resolve();
self.onmessage = (event: MessageEvent<ScorePpWorkerRequest>) => {
  queue = queue.then(() => calculate(event.data));
};

async function calculate(request: ScorePpWorkerRequest) {
  const { id, input } = request;
  const reply = (value: ScorePpWorkerResponse) => self.postMessage(value);
  try {
    if (typeof id !== "string") throw new Error("Invalid calculation request");
    const params = scorePpPerformanceArgs(input);
    const checksum = input.beatmapChecksum;
    const url = new URL(request.url, self.location.origin);
    if (!["https:", "http:"].includes(url.protocol) || url.username || url.password || !/^\/api\/osu\/v1\/playback\/beatmaps\/[1-9]\d*\/file$/.test(url.pathname)) throw new Error("Invalid beatmap file URL");
    const key = checksum.toLowerCase();
    const rosu = await loadRosu();
    let map = maps.get(key);
    if (!map) {
      const response = await fetch(url, { signal: AbortSignal.timeout(15000), credentials: "omit", redirect: "error" });
      if (!response.ok || !response.body) throw new Error("Map file unavailable");
      const reader = response.body.getReader();
      const chunks: Uint8Array[] = [];
      let size = 0;
      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          size += value.byteLength;
          if (size > maxMapBytes) { await reader.cancel(); throw new Error("Map file is too large"); }
          chunks.push(value);
        }
      } finally { reader.releaseLock(); }
      const bytes = new Uint8Array(size);
      let offset = 0;
      for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
      if (md5(bytes).toLowerCase() !== key) throw new Error("Map checksum mismatch; the score cannot be calculated against this revision");
      const text = new TextDecoder().decode(bytes);
      if (!text.startsWith("osu file format")) throw new Error("Invalid map file");
      map = new rosu.Beatmap(text);
      if (map.mode !== 0) { map.free(); throw new Error("Only osu!standard scores are supported"); }
      maps.set(key, map);
      while (maps.size > 8) {
        const oldest = maps.keys().next().value!;
        maps.get(oldest)!.free();
        maps.delete(oldest);
      }
    } else {
      maps.delete(key);
      maps.set(key, map);
    }
    validateScorePpObjectCount(input, map.nObjects);
    const performance = new rosu.Performance(params);
    try {
      const result = performance.calculate(map);
      try {
        if (!validScorePp(result.pp)) throw new Error("Calculation returned invalid PP");
        reply({ id, pp: result.pp });
      } finally { result.free(); }
    } finally { performance.free(); }
  } catch (error) {
    reply({ id, error: error instanceof Error ? error.message : "Score PP calculation unavailable" });
  }
}
