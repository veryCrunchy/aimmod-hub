import type { PpResult } from "./ppTargetCache";

export function readPpGoal(params: URLSearchParams) {
  const read = (key: string) => {
    const raw = params.get(key);
    const value = Number(raw);
    return raw?.trim() && Number.isFinite(value) && value >= 0 && value <= 10000 ? value : undefined;
  };
  let min = read("ppMin"), max = read("ppMax");
  if (min !== undefined && max !== undefined && min > max) [min, max] = [max, min];
  return { min, max };
}

export function matchesPpGoal(result: PpResult | undefined, goal: ReturnType<typeof readPpGoal>) {
  if (goal.min === undefined && goal.max === undefined) return true;
  if (!result || result.error || !Number.isFinite(result.pp)) return false;
  return (goal.min === undefined || result.pp >= goal.min) && (goal.max === undefined || result.pp <= goal.max);
}
