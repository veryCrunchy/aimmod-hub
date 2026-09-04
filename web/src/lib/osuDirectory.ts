import type { OsuSharedReplay } from "./osuCommunity";

export function groupOsuPlays(items: OsuSharedReplay[], kind: "beatmaps" | "players") {
  const groups = new Map<string, OsuSharedReplay[]>();
  for (const item of items) {
    const key = kind === "players" ? item.hubHandle : String(item.beatmapId);
    if (!key || (kind === "beatmaps" && item.beatmapId <= 0)) continue;
    const group = groups.get(key) ?? [];
    group.push(item);
    groups.set(key, group);
  }
  return [...groups.entries()].map(([id, plays]) => {
    const latest = [...plays].sort((a, b) => Date.parse(b.playedAt) - Date.parse(a.playedAt))[0];
    const pp = plays.flatMap(play => play.performancePoints == null ? [] : [play.performancePoints]);
    return { id, latest, plays, bestPP: pp.length ? Math.max(...pp) : null };
  });
}
