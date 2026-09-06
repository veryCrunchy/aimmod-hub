import { md5 } from "replayviewer-js";
import { calculateOfficialPp } from "./officialPp";

self.onmessage = async (event: MessageEvent<{ id: number; url: string; checksum: string; accuracy: number; mods: string; lazer: boolean }>) => {
  const { id, url, checksum, accuracy, mods, lazer } = event.data;
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(15000) });
    if (!response.ok) throw new Error("Map file unavailable");
    const bytes = new Uint8Array(await response.arrayBuffer());
    if (bytes.length > 4 * 1024 * 1024 || md5(bytes).toLowerCase() !== checksum.toLowerCase()) throw new Error("The exact beatmap revision is unavailable.");
    const result = await calculateOfficialPp(bytes, checksum, { accuracy, mods, lazer });
    self.postMessage({ id, ...result });
  } catch (error) { self.postMessage({ id, error: error instanceof Error ? error.message : "Calculation unavailable" }); }
};
