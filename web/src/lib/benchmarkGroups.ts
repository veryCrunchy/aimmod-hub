// Shared grouping logic for benchmark series (Voltaic S5 Novice/Intermediate/Advanced, etc.)

const DIFFICULTY_DASH_RE =
  /\s*[-–]\s*(novice|intermediate|advanced|expert|beginner|easy|medium|hard(?:er)?|eas(?:y|ier)|s\d+[-–]?\s*(?:novice|intermediate|advanced|expert))\s*$/i;
const DIFFICULTY_WORD_RE =
  /\s+(novice|intermediate|advanced|expert|beginner|easier|harder|medium)\s*$/i;

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
    const key = item.benchmarkIconUrl
      ? `${base.toLowerCase()}:::${item.benchmarkIconUrl}`
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
