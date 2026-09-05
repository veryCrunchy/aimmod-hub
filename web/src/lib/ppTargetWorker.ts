import { loadRosu } from "virtual:rosu-browser";
import { md5 } from "replayviewer-js";

self.onmessage = async (event: MessageEvent<{ id: number; url: string; checksum: string; accuracy: number; mods: string; lazer: boolean }>) => {
  const { id, url, checksum, accuracy, mods, lazer } = event.data;
  try {
    const response = await fetch(url, { signal: AbortSignal.timeout(15000) });
    if (!response.ok) throw new Error("Map file unavailable");
    const bytes = new Uint8Array(await response.arrayBuffer());
    if (checksum && md5(bytes).toLowerCase() !== checksum.toLowerCase()) throw new Error("The map changed. Refresh your search to calculate the current difficulty.");
    const text = new TextDecoder().decode(bytes);
    if (!text.startsWith("osu file format") || text.length > 4 * 1024 * 1024) throw new Error("Invalid map file");
    const rosu = await loadRosu();
    const map = new rosu.Beatmap(text);
    try {
      if (map.mode !== 0) throw new Error("Only osu!standard maps are supported");
      const atAccuracy = new rosu.Performance({ accuracy, mods: mods === "NM" ? 0 : mods, lazer, misses: 0 });
      const perfect = new rosu.Performance({ accuracy: 100, mods: mods === "NM" ? 0 : mods, lazer, misses: 0 });
      try {
        const expected = atAccuracy.calculate(map);
        const ss = perfect.calculate(map);
        try {
          const difficulty = ss.difficulty;
          try { self.postMessage({ id, pp: expected.pp, maxPp: ss.pp, stars: difficulty.stars }); }
          finally { difficulty.free(); }
        }
        finally { expected.free(); ss.free(); }
      } finally { atAccuracy.free(); perfect.free(); }
    } finally { map.free(); }
  } catch (error) { self.postMessage({ id, error: error instanceof Error ? error.message : "Calculation unavailable" }); }
};
