// Small, pure helpers that turn Hub API data into player-facing numbers.

export type ScoredRun = {
  score: number;
  accuracy: number;
  playedAtIso: string;
  durationMs?: bigint | number;
  userHandle?: string;
  userDisplayName?: string;
  runId?: string;
  sessionId?: string;
};

/** Time played as "12h 30m", "45m" or "under a minute". */
export function formatPlaytime(ms: bigint | number | null | undefined): string {
  const total = Math.max(0, Math.floor(Number(ms ?? 0) / 60_000));
  if (total < 1) return "under a minute";
  const hours = Math.floor(total / 60);
  const minutes = total % 60;
  if (hours === 0) return `${minutes}m`;
  if (hours >= 100 || minutes === 0) return `${hours.toLocaleString()}h`;
  return `${hours}h ${minutes}m`;
}

export function formatScore(score: number | null | undefined): string {
  return score == null || !Number.isFinite(score) ? "—" : Math.round(score).toLocaleString();
}

export function formatAccuracy(accuracy: number | null | undefined): string {
  return accuracy == null || !Number.isFinite(accuracy) ? "—" : `${accuracy.toFixed(1)}%`;
}

export function formatClock(seconds: number | null | undefined): string {
  if (seconds == null || !Number.isFinite(seconds)) return "—";
  const whole = Math.max(0, Math.round(seconds));
  return `${Math.floor(whole / 60)}:${(whole % 60).toString().padStart(2, "0")}`;
}

function time(iso: string): number {
  const value = new Date(iso).getTime();
  return Number.isFinite(value) ? value : 0;
}

export function newestFirst<T extends { playedAtIso: string }>(runs: readonly T[]): T[] {
  return [...runs].sort((a, b) => time(b.playedAtIso) - time(a.playedAtIso));
}

export type Trend = { recent: number; previous: number; changePct: number };

/**
 * Compares the average score of the newest `window` runs with the `window`
 * runs before them. Returns null when there are not enough runs to compare.
 */
export function scoreTrend(runs: readonly ScoredRun[], window = 10): Trend | null {
  const sorted = newestFirst(runs);
  const size = Math.min(window, Math.floor(sorted.length / 2));
  if (size < 3) return null;
  const avg = (items: readonly ScoredRun[]) => items.reduce((sum, run) => sum + run.score, 0) / items.length;
  const recent = avg(sorted.slice(0, size));
  const previous = avg(sorted.slice(size, size * 2));
  if (previous <= 0) return null;
  return { recent, previous, changePct: ((recent - previous) / previous) * 100 };
}

/**
 * Accuracy of the newest `window` runs against the runs before them, in
 * percentage points. Accuracy compares across scenarios; scores do not.
 */
export function accuracyTrend(runs: readonly ScoredRun[], window = 10): { recent: number; previous: number; changePts: number } | null {
  const sorted = newestFirst(runs);
  const size = Math.min(window, Math.floor(sorted.length / 2));
  if (size < 3) return null;
  const avg = (items: readonly ScoredRun[]) => items.reduce((sum, run) => sum + run.accuracy, 0) / items.length;
  const recent = avg(sorted.slice(0, size));
  const previous = avg(sorted.slice(size, size * 2));
  return { recent, previous, changePts: recent - previous };
}

export function formatPoints(changePts: number): string {
  const rounded = Math.round(changePts * 10) / 10;
  if (rounded === 0) return "steady";
  return `${rounded > 0 ? "+" : ""}${rounded.toFixed(1)} pts`;
}

export function formatChange(changePct: number): string {
  const rounded = Math.round(changePct * 10) / 10;
  if (rounded === 0) return "no change";
  return `${rounded > 0 ? "+" : ""}${rounded.toFixed(1)}%`;
}

/** Distinct days with at least one run in the `days` before `nowMs`. */
export function activeDays(runs: readonly { playedAtIso: string }[], days: number, nowMs = Date.now()): number {
  const since = nowMs - days * 86_400_000;
  const seen = new Set<string>();
  for (const run of runs) {
    const at = time(run.playedAtIso);
    if (at >= since && at <= nowMs) seen.add(new Date(at).toISOString().slice(0, 10));
  }
  return seen.size;
}

export type PlayerBest<T extends ScoredRun> = {
  handle: string;
  displayName: string;
  best: T;
  runs: number;
  averageScore: number;
};

/** One row per player, ranked by their best score. */
export function bestPerPlayer<T extends ScoredRun>(runs: readonly T[]): PlayerBest<T>[] {
  const byPlayer = new Map<string, { best: T; runs: number; total: number; displayName: string }>();
  for (const run of runs) {
    const handle = run.userHandle || run.userDisplayName || "";
    if (!handle) continue;
    const current = byPlayer.get(handle);
    if (!current) {
      byPlayer.set(handle, { best: run, runs: 1, total: run.score, displayName: run.userDisplayName || handle });
      continue;
    }
    current.runs += 1;
    current.total += run.score;
    if (run.score > current.best.score) current.best = run;
  }
  return [...byPlayer.entries()]
    .map(([handle, entry]) => ({ handle, displayName: entry.displayName, best: entry.best, runs: entry.runs, averageScore: entry.total / entry.runs }))
    .sort((a, b) => b.best.score - a.best.score || b.runs - a.runs);
}

/** Share of `scores` that `score` beats or ties, as 0-100. */
export function percentileOf(score: number, scores: readonly number[]): number | null {
  if (scores.length === 0) return null;
  const atOrBelow = scores.filter((value) => value <= score).length;
  return Math.round((atOrBelow / scores.length) * 100);
}

/** Rank as a share of the field, e.g. rank 3 of 40 is the top 8%. */
export function topPercent(rank: number, total: number): number | null {
  if (rank < 1 || total < 1 || rank > total) return null;
  return Math.max(1, Math.ceil((rank / total) * 100));
}

/** 1-based rank of `score` within a list sorted by score descending, or null when it falls outside the list. */
export function rankWithin(score: number, ranked: readonly { score: number }[]): number | null {
  if (ranked.length === 0) return null;
  const index = ranked.findIndex((run) => score >= run.score);
  return index === -1 ? null : index + 1;
}

/** Players ordered by how many scenario records they hold. */
export function recordHolders(records: readonly { userHandle?: string; userDisplayName?: string }[]) {
  const counts = new Map<string, { handle: string; name: string; records: number }>();
  for (const record of records) {
    const handle = record.userHandle || record.userDisplayName;
    if (!handle) continue;
    const entry = counts.get(handle) ?? { handle, name: record.userDisplayName || handle, records: 0 };
    entry.records += 1;
    counts.set(handle, entry);
  }
  return [...counts.values()].sort((a, b) => b.records - a.records || a.name.localeCompare(b.name));
}
