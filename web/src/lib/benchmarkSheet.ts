// Pure helpers for benchmark sheets: rank progress, grouping, sorting and
// summaries. Shared by AimMod profiles and KovaaK's player pages.

export type SheetRank = { rankIndex: number; rankName: string; iconUrl?: string; color?: string };
export type SheetThreshold = { rankIndex: number; rankName: string; iconUrl?: string; color?: string; score: number };
export type SheetScenario = {
  scenarioName: string;
  scenarioSlug?: string;
  categoryName: string;
  score: number;
  leaderboardRank?: number;
  leaderboardId?: number;
  scenarioRank?: SheetRank;
  thresholds: SheetThreshold[];
  scoreSource?: string;
};
export type SheetCategory = { categoryName: string; categoryRank?: number; benchmarkProgress?: number; scenarios: SheetScenario[] };

export function hasRankName(name?: string | null) {
  const n = name?.trim().toLowerCase();
  return Boolean(n && n !== "no rank");
}

/** Where a score sits on a scenario's rank ladder. */
export type RankProgress = {
  /** Highest rank reached, or null when below the first threshold. */
  current: SheetThreshold | null;
  next: SheetThreshold | null;
  /** Points still needed for the next rank (0 at the top rank). */
  pointsToNext: number;
  /** 0-100 progress from the current rank's threshold to the next one. */
  pctToNext: number;
  /** Percent above the top threshold, once it is reached. */
  pctOverTop: number;
  maxed: boolean;
};

export function rankProgress(score: number, thresholds: readonly SheetThreshold[]): RankProgress {
  const sorted = [...thresholds].sort((a, b) => a.rankIndex - b.rankIndex);
  let current: SheetThreshold | null = null;
  let next: SheetThreshold | null = null;
  for (const t of sorted) {
    if (score >= t.score) current = t;
    else { next = t; break; }
  }
  if (!next) {
    const top = sorted[sorted.length - 1];
    const over = top && top.score > 0 ? Math.max(0, ((score - top.score) / top.score) * 100) : 0;
    return { current, next: null, pointsToNext: 0, pctToNext: 100, pctOverTop: Math.round(over), maxed: Boolean(top) };
  }
  const floor = current ? current.score : 0;
  const span = next.score - floor;
  const pct = span > 0 ? ((score - floor) / span) * 100 : 0;
  return {
    current,
    next,
    pointsToNext: Math.max(0, next.score - score),
    pctToNext: Math.max(0, Math.min(100, Math.round(pct))),
    pctOverTop: 0,
    maxed: false,
  };
}

/**
 * KovaaK's category names put the family last: "Smoothness Tracking" →
 * parent "Tracking", sub "Smoothness". A single word stays as the parent.
 */
export function splitCategory(name: string): { parent: string; sub: string | null } {
  const parts = name.trim().split(/\s+/);
  if (parts.length <= 1) return { parent: parts[0] ?? "", sub: null };
  return { parent: parts[parts.length - 1], sub: parts.slice(0, -1).join(" ") };
}

export type CategoryGroup = { parent: string; categories: SheetCategory[]; rows: number };

/** Groups categories that share a first word, keeping sheet order. */
export function groupCategories(categories: readonly SheetCategory[]): CategoryGroup[] {
  const groups: CategoryGroup[] = [];
  for (const category of categories) {
    if (!category.scenarios.length) continue;
    const { parent } = splitCategory(category.categoryName);
    let group = groups.find((g) => g.parent === parent);
    if (!group) {
      group = { parent, categories: [], rows: 0 };
      groups.push(group);
    }
    group.categories.push(category);
    group.rows += category.scenarios.length;
  }
  return groups;
}

export type SheetSort = "sheet" | "closest" | "weakest";

/**
 * Flat scenario order for a sort mode:
 * - sheet: the author's order
 * - closest: nearest to the next rank first (by share of the gap left), maxed last
 * - weakest: lowest rank first, then least progress
 */
export function sortScenarios(categories: readonly SheetCategory[], sort: SheetSort): SheetScenario[] {
  // Sheet order follows the grouped table: families together, author order within.
  const all = groupCategories(categories).flatMap((g) => g.categories.flatMap((c) => c.scenarios));
  if (sort === "sheet") return all;
  const keyed = all.map((s, i) => ({ s, i, p: rankProgress(s.score, s.thresholds) }));
  if (sort === "closest") {
    keyed.sort((a, b) => {
      if (a.p.maxed !== b.p.maxed) return a.p.maxed ? 1 : -1;
      return b.p.pctToNext - a.p.pctToNext || a.i - b.i;
    });
  } else {
    keyed.sort((a, b) => {
      const ra = a.p.current?.rankIndex ?? 0;
      const rb = b.p.current?.rankIndex ?? 0;
      return ra - rb || a.p.pctToNext - b.p.pctToNext || a.i - b.i;
    });
  }
  return keyed.map((k) => k.s);
}

/** The rank columns of a sheet, from the first scenario that has thresholds. */
export function sheetColumns(categories: readonly SheetCategory[]): SheetThreshold[] {
  for (const category of categories) {
    for (const scenario of category.scenarios) {
      if (scenario.thresholds.length) return [...scenario.thresholds].sort((a, b) => a.rankIndex - b.rankIndex);
    }
  }
  return [];
}

export type RankCount = { rankIndex: number; rankName: string; color?: string; iconUrl?: string; count: number };

/** How many scenarios sit at each rank, highest first, plus unranked. */
export function rankCounts(categories: readonly SheetCategory[]): { counts: RankCount[]; unranked: number; total: number } {
  const map = new Map<number, RankCount>();
  let unranked = 0;
  let total = 0;
  for (const scenario of categories.flatMap((c) => c.scenarios)) {
    total++;
    const { current } = rankProgress(scenario.score, scenario.thresholds);
    if (!current) { unranked++; continue; }
    const entry = map.get(current.rankIndex) ?? { rankIndex: current.rankIndex, rankName: current.rankName, color: current.color, iconUrl: current.iconUrl, count: 0 };
    entry.count++;
    map.set(current.rankIndex, entry);
  }
  return { counts: [...map.values()].sort((a, b) => b.rankIndex - a.rankIndex), unranked, total };
}

/** Readable name for a rank colour; falls back for blank or white provider colours. */
export function rankColor(color: string | undefined, fallbackIndex = 0): string {
  const c = color?.trim();
  if (c && /^#[0-9a-f]{3,8}$/i.test(c) && !/^#f{3}(f{3})?$/i.test(c)) return c;
  const palette = ["#8a8f98", "#b07840", "#b8c0c8", "#e0b840", "#5ec8d8", "#7aa8ff", "#c070f0", "#f07070"];
  return palette[Math.abs(fallbackIndex) % palette.length];
}

/** Compact score label for threshold cells. */
export function compactScore(n: number): string {
  if (!Number.isFinite(n)) return "–";
  if (Math.abs(n) >= 10_000) return `${(n / 1000).toFixed(n >= 100_000 ? 0 : 1).replace(/\.0$/, "")}k`;
  if (Math.abs(n) >= 1000) return n.toLocaleString("en-US", { maximumFractionDigits: 0 });
  return Number.isInteger(n) ? String(n) : n.toFixed(1).replace(/\.0$/, "");
}
