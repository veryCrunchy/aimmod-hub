// Shared grouping logic for benchmark series (Voltaic S5 Novice/Intermediate/Advanced, etc.)

const DIFFICULTY_DASH_RE =
  /\s*[-–]\s*(novice|intermediate|advanced|expert|beginner|easy|easier|medium|hard(?:er)?|s\d+[-–]?\s*(?:novice|intermediate|advanced|expert))\s*$/i;
const DIFFICULTY_WORD_RE =
  /\s+(novice|intermediate|advanced|expert|beginner|easy|easier|medium|hard(?:er)?)\s*$/i;

export const DIFFICULTY_ORDER = [
  "beginner", "novice", "easy", "easier",
  "intermediate", "medium",
  "advanced", "hard", "harder", "expert",
];

export function extractDifficulty(name: string): { base: string; difficulty: string | null } {
  let m = name.match(DIFFICULTY_DASH_RE);
  if (m) return { base: name.slice(0, m.index).trim(), difficulty: m[1] };
  m = name.match(DIFFICULTY_WORD_RE);
  if (m) return { base: name.slice(0, m.index).trim(), difficulty: m[1] };
  return { base: name, difficulty: null };
}

export type BenchmarkVariant<T> = {
  item: T;
  difficulty: string | null;
};

export type BenchmarkGroup<T> = {
  base: string;
  iconUrl: string;
  author: string;
  type: string;
  variants: BenchmarkVariant<T>[];
};

/** Group a list of benchmarks by series (same base name + same icon URL). */
export function groupBenchmarks<T extends {
  benchmarkId: number;
  benchmarkName: string;
  benchmarkIconUrl: string;
  benchmarkAuthor: string;
  benchmarkType: string;
}>(items: T[]): BenchmarkGroup<T>[] {
  const groups = new Map<string, BenchmarkGroup<T>>();
  for (const item of items) {
    const { base, difficulty } = extractDifficulty(item.benchmarkName);
    // Group by base name + author so variants without icons still cluster.
    // Include author to avoid merging same-named benchmarks from different creators.
    const key = difficulty !== null
      ? `${base.toLowerCase()}:::${item.benchmarkAuthor.toLowerCase()}`
      : `__solo__${item.benchmarkId}`;
    if (!groups.has(key)) {
      groups.set(key, {
        base,
        iconUrl: item.benchmarkIconUrl,
        author: item.benchmarkAuthor,
        type: item.benchmarkType,
        variants: [],
      });
    }
    groups.get(key)!.variants.push({ item, difficulty });
  }
  for (const group of groups.values()) {
    group.variants.sort((a, b) => {
      const ai = DIFFICULTY_ORDER.indexOf(a.difficulty?.toLowerCase() ?? "");
      const bi = DIFFICULTY_ORDER.indexOf(b.difficulty?.toLowerCase() ?? "");
      return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
    });
  }
  return [...groups.values()];
}

export type BenchmarkSort = "players" | "name" | "kovaaks";
type BenchmarkLike = Parameters<typeof groupBenchmarks>[0][number] & { playerCount: number; benchmarkName: string; kovaaksPlayers?: bigint | number; hidden?: boolean };

export function groupPlayers(group: BenchmarkGroup<{ playerCount?: number }>) {
  return group.variants.reduce((sum, v) => sum + (v.item.playerCount ?? 0), 0);
}

/** Most KovaaK's players across a series' difficulties. */
export function groupKovaaksPlayers(group: BenchmarkGroup<{ kovaaksPlayers?: bigint | number }>) {
  return group.variants.reduce((most, v) => Math.max(most, Number(v.item.kovaaksPlayers ?? 0)), 0);
}

/** A series is hidden only when every difficulty in it is hidden. */
export function groupHidden(group: BenchmarkGroup<{ hidden?: boolean }>) {
  return group.variants.every((v) => v.item.hidden);
}

/**
 * Groups ordered for browsing. Hidden benchmarks (empty, test-like or barely
 * played on KovaaK's) are left out unless includeHidden is set.
 */
export function browseBenchmarkGroups<T extends BenchmarkLike>(items: readonly T[], query: string, sort: BenchmarkSort, rankedOnly: boolean, includeHidden = true) {
  const q = query.trim().toLowerCase();
  return groupBenchmarks([...items])
    .filter((group) => includeHidden || !groupHidden(group))
    .filter((group) => !q || `${group.base} ${group.author} ${group.type} ${group.variants.map((v) => v.item.benchmarkName).join(" ")}`.toLowerCase().includes(q))
    .filter((group) => !rankedOnly || groupPlayers(group) > 0)
    .sort((a, b) => {
      if (sort === "name") return a.base.localeCompare(b.base);
      if (sort === "kovaaks") return groupKovaaksPlayers(b) - groupKovaaksPlayers(a) || groupPlayers(b) - groupPlayers(a) || a.base.localeCompare(b.base);
      return groupPlayers(b) - groupPlayers(a) || groupKovaaksPlayers(b) - groupKovaaksPlayers(a) || a.base.localeCompare(b.base);
    });
}
