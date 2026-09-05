import { API_BASE_URL } from "./config";
import type { BeatmapData, ReplayData } from "replayviewer-js";

export interface ParsedOsuPlayback { replay: ReplayData; beatmap: BeatmapData }

export function osuPlaybackBeatmapUrl(beatmapId: number): string {
  if (!Number.isSafeInteger(beatmapId) || beatmapId <= 0 || beatmapId > 2147483647) throw new Error("Select a beatmap difficulty.");
  return `${API_BASE_URL}/api/osu/v1/playback/beatmaps/${beatmapId}/file`;
}

export function osuPlaybackAudioUrl(beatmapId: number, beatmapsetId: number | undefined, checksum: string): string | undefined {
  if (!Number.isSafeInteger(beatmapsetId) || !beatmapsetId || beatmapsetId <= 0 || beatmapsetId > 2147483647) return;
  if (!Number.isSafeInteger(beatmapId) || beatmapId <= 0 || beatmapId > 2147483647 || !/^[a-f0-9]{32}$/i.test(checksum)) return;
  return `${API_BASE_URL}/api/osu/v1/playback/beatmaps/${beatmapId}/audio?beatmapsetId=${beatmapsetId}&checksum=${checksum.toLowerCase()}`;
}

export async function fetchPlaybackBytes(url: string, maximumBytes: number, signal: AbortSignal): Promise<ArrayBuffer> {
  const parsed = new URL(url, typeof location === "undefined" ? "http://localhost" : location.href);
  if (!["https:", "http:", "blob:"].includes(parsed.protocol)) throw new Error("This playback source is not supported.");
  const response = await fetch(url, { signal, credentials: "same-origin" });
  if (!response.ok) throw new Error(response.status === 404 ? "This replay or beatmap file is no longer available." : "The playback files could not be loaded. Try again.");
  if (Number(response.headers.get("Content-Length")) > maximumBytes) throw new Error("This file exceeds the playback size limit.");
  if (!response.body) throw new Error("The playback file was empty.");
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > maximumBytes) throw new Error("This file exceeds the playback size limit.");
      chunks.push(value);
    }
  } finally { await reader.cancel().catch(() => undefined); reader.releaseLock(); }
  if (!length) throw new Error("The playback file was empty.");
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
  return bytes.buffer;
}

export function decodeOsuPlayback(replay: ArrayBuffer, beatmap: ArrayBuffer, signal: AbortSignal): Promise<ParsedOsuPlayback> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) { reject(new DOMException("Cancelled", "AbortError")); return; }
    const worker = new Worker(new URL("./osuPlayback.worker.ts", import.meta.url), { type: "module" });
    const finish = () => { clearTimeout(timeout); signal.removeEventListener("abort", abort); worker.terminate(); };
    const abort = () => { finish(); reject(new DOMException("Cancelled", "AbortError")); };
    const timeout = setTimeout(() => { finish(); reject(new Error("This replay took too long to decode.")); }, 30000);
    signal.addEventListener("abort", abort, { once: true });
    worker.onmessage = (event: MessageEvent<{ data?: ParsedOsuPlayback; error?: string }>) => {
      finish();
      if (event.data.data) resolve(event.data.data);
      else reject(new Error(event.data.error || "The replay could not be decoded."));
    };
    worker.onerror = () => { finish(); reject(new Error("The replay could not be decoded.")); };
    worker.postMessage({ replay, beatmap }, [replay, beatmap]);
  });
}

export function playbackTimeLabel(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
