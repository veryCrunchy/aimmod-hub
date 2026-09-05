import { parseOsuPlayback } from "./osuPlaybackDecode";

self.onmessage = async (event: MessageEvent<{ replay: ArrayBuffer; beatmap: ArrayBuffer }>) => {
  try { self.postMessage({ data: await parseOsuPlayback(event.data.replay, event.data.beatmap) }); }
  catch (error) { self.postMessage({ error: error instanceof Error ? error.message : "This replay could not be decoded." }); }
};
