import { BeatmapDifficulty, BeatmapItem, ProviderStatus, Ruleset } from "../gen/aimmod/osu/v1/osu_pb";
import { validChecksum } from "./ppTargetCache";

export function matchingPpDifficulties(item: BeatmapItem, low: string, high: string): BeatmapDifficulty[] {
  return item.difficulties.filter(map => map.ruleset === Ruleset.OSU
    && (!low || map.stars >= Number(low)) && (!high || map.stars <= Number(high)))
    .sort((a, b) => b.stars - a.stars);
}

// Search already includes full difficulty metadata. Only hydrate incomplete sets.
export function needsPpDetails(item: BeatmapItem, low: string, high: string): boolean {
  return !item.difficulties.length || item.beatmapCount > item.difficulties.length
    || matchingPpDifficulties(item, low, high).some(map => !validChecksum(map.checksum));
}

export function ppSearchError(providers: ProviderStatus[]): string | undefined {
  const failed = providers.find(provider => !provider.available);
  if (!failed) return;
  if (/deadline|timeout|timed out/i.test(failed.message)) return "Beatmap search took too long. Please try again.";
  return "Beatmap search is temporarily unavailable. Please try again.";
}

export function ppResultsTitle(failed: boolean, busy: boolean, sets: number, difficulties: number): string {
  if (!difficulties && failed) return "Results unavailable";
  if (!difficulties && busy) return "Finding beatmaps…";
  return `${sets} beatmaps · ${difficulties} difficulties`;
}
