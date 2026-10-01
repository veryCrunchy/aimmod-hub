// Helpers for the search palette: where results go, how they group, and the
// short list of recent searches kept in this browser only.

export type QuickResultLike = {
  kind: string;
  title: string;
  userHandle?: string;
  scenarioSlug?: string;
  scenarioName?: string;
  benchmarkId?: number;
  steamId?: string;
};

export function quickResultHref(result: QuickResultLike): string {
  switch (result.kind) {
    case "player":
      return `/profiles/${encodeURIComponent(result.userHandle ?? "")}`;
    case "kovaaks_player":
      return `/u/${encodeURIComponent(result.steamId ?? "")}`;
    case "scenario":
      return `/scenarios/${encodeURIComponent(result.scenarioSlug ?? "")}`;
    case "kovaaks_scenario":
      // Scenarios nobody uploaded to AimMod still have a KovaaK's board.
      return `/scenarios/${encodeURIComponent(result.scenarioSlug ?? "")}?tab=kovaaks&name=${encodeURIComponent(result.scenarioName ?? result.title)}`;
    case "benchmark":
      return `/benchmarks/${result.benchmarkId ?? 0}`;
    default:
      return `/search?q=${encodeURIComponent(result.title)}`;
  }
}

export const quickKindLabels: Record<string, string> = {
  player: "Players",
  kovaaks_player: "KovaaK's players",
  scenario: "Scenarios",
  kovaaks_scenario: "KovaaK's scenarios",
  benchmark: "Benchmarks",
};

const kindOrder = ["player", "scenario", "benchmark", "kovaaks_player", "kovaaks_scenario"];

/**
 * Groups ranked results by kind. Groups are ordered by their best result so
 * the most relevant section comes first; within a group, server order holds.
 */
export function groupQuickResults<T extends QuickResultLike & { relevance?: number }>(results: readonly T[]): { kind: string; label: string; items: T[] }[] {
  const groups = new Map<string, T[]>();
  for (const result of results) {
    const list = groups.get(result.kind) ?? [];
    list.push(result);
    groups.set(result.kind, list);
  }
  return [...groups.entries()]
    .map(([kind, items]) => ({ kind, label: quickKindLabels[kind] ?? kind, items }))
    .sort((a, b) => (b.items[0]?.relevance ?? 0) - (a.items[0]?.relevance ?? 0) || kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind));
}

type StorageLike = Pick<Storage, "getItem" | "setItem">;
const recentKey = "aimmod-recent-searches-v1";

export type RecentSearch = { title: string; href: string; kind: string };

export function loadRecentSearches(storage: StorageLike | undefined): RecentSearch[] {
  try {
    const raw = storage?.getItem(recentKey);
    if (!raw) return [];
    const value = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    return value.filter((v) => v && typeof v.title === "string" && typeof v.href === "string" && v.href.startsWith("/")).slice(0, 6);
  } catch {
    return [];
  }
}

export function rememberSearch(storage: StorageLike | undefined, entry: RecentSearch): RecentSearch[] {
  const next = [entry, ...loadRecentSearches(storage).filter((v) => v.href !== entry.href)].slice(0, 6);
  try {
    storage?.setItem(recentKey, JSON.stringify(next));
  } catch {
    // Recent searches are optional.
  }
  return next;
}
