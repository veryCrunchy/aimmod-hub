import { md5 } from "replayviewer-js";
import { scorePpValidationReason, type ScorePpWorkerRequest } from "./scorePp";
import { calculateOfficialPp } from "./officialPp";

self.onmessage = async (event: MessageEvent<ScorePpWorkerRequest>) => {
  const { id, input, url: source } = event.data;
  try {
    const reason = scorePpValidationReason(input);
    if (reason) throw new Error(reason);
    const url = new URL(source, self.location.origin);
    if (!["https:", "http:"].includes(url.protocol) || url.username || url.password || !/^\/api\/osu\/v1\/playback\/beatmaps\/[1-9]\d*\/file$/.test(url.pathname)) throw new Error("Invalid beatmap file URL");
    const response = await fetch(url, { signal: AbortSignal.timeout(15000), credentials: "omit", redirect: "error" });
    if (!response.ok) throw new Error("Map file unavailable");
    const bytes = new Uint8Array(await response.arrayBuffer());
    if (bytes.length > 4 * 1024 * 1024 || md5(bytes).toLowerCase() !== input.beatmapChecksum.toLowerCase()) throw new Error("The exact beatmap revision is unavailable.");
    self.postMessage({ id, ...await calculateOfficialPp(bytes, input.beatmapChecksum, input) });
  } catch (error) { self.postMessage({ id, error: error instanceof Error ? error.message : "PP calculation unavailable." }); }
};
