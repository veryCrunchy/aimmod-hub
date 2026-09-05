import type { OsuSharedReplay } from "./osuCommunity";

export const scoreFilterKeys = ["q", "source", "replay", "mods", "result", "period", "sort", "beatmap", "starsMin", "starsMax", "accMin", "accMax", "ppMin", "ppMax"];
const bound = (params: URLSearchParams, key: string) => {
  const raw = params.get(key);
  if (!raw?.trim()) return undefined;
  const value = Number(raw);
  return Number.isFinite(value) && value >= 0 ? value : undefined;
};
function inRange(value: number | null | undefined, low?: number, high?: number) {
  if (low === undefined && high === undefined) return true;
  return value != null && Number.isFinite(value) && (low === undefined || value >= low) && (high === undefined || value <= high);
}
export function filterOsuScores(items: readonly OsuSharedReplay[], params: URLSearchParams, defaultReplay = "all", now = Date.now()) {
  const term = (params.get("q") ?? "").trim().toLowerCase();
  const source = params.get("source") ?? "all";
  const replayFilter = params.get("replay") ?? defaultReplay;
  const mod = params.get("mods") ?? "all";
  const days = ["7", "30", "90", "365"].includes(params.get("period") ?? "") ? Number(params.get("period")) : 0;
  const bounds = ["stars", "acc", "pp"].map(key => [bound(params, `${key}Min`), bound(params, `${key}Max`)] as const);
  const result = items.filter(item => {
    const isUpload = item.source !== "official";
    const isOfficial = item.source === "official" || item.source === "merged";
    const hasReplay = item.hasReplayFile || item.officialReplayExists === true;
    const mods = (item.mods ?? []).map(value => value.toUpperCase()).filter(value => value !== "NM");
    const matchingMod = mod === "all" || (mod === "NM" ? mods.length === 0 : mod === "DT" ? mods.some(value => ["DT", "NC"].includes(value)) : mod === "HT" ? mods.some(value => ["HT", "DC"].includes(value)) : mods.includes(mod));
    return (!params.get("beatmap") || String(item.beatmapId) === params.get("beatmap"))
      && (source !== "uploads" || isUpload) && (source !== "official" || isOfficial) && (source !== "merged" || item.source === "merged")
      && (!["file", "available"].includes(replayFilter) || hasReplay) && (replayFilter !== "none" || !hasReplay)
      && (replayFilter !== "analysis" || Boolean(item.analysis?.judgements?.length))
      && (params.get("result") !== "passed" || item.passed) && (params.get("result") !== "failed" || !item.passed)
      && (!days || Date.parse(item.playedAt) >= now - days * 86400000)
      && matchingMod && inRange(item.starRating, ...bounds[0]) && inRange(item.accuracy * 100, ...bounds[1]) && inRange(item.performancePoints, ...bounds[2])
      && (!term || [item.title, item.artist, item.difficulty, item.creator, item.osuUsername, item.hubHandle].some(value => value?.toLowerCase().includes(term)));
  });
  const numeric = (value: number | undefined | null) => value != null && Number.isFinite(value) ? value : -Infinity;
  return result.sort((a, b) => {
    const key = params.get("sort");
    if (key === "pp") return numeric(b.performancePoints) - numeric(a.performancePoints);
    if (key === "accuracy") return numeric(b.accuracy) - numeric(a.accuracy);
    if (key === "stars") return numeric(b.starRating) - numeric(a.starRating);
    if (key === "score") return numeric(b.totalScore) - numeric(a.totalScore);
    return (Date.parse(b.playedAt) || 0) - (Date.parse(a.playedAt) || 0);
  });
}
