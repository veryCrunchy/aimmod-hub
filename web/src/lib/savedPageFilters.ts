const key = "aimmod-page-filters-v1";
export function filterChoice<T extends string>(value: string | null, choices: readonly T[], fallback: T): T {
  return choices.includes(value as T) ? value as T : fallback;
}
export function updateFilterQuery(current: URLSearchParams, changes: Record<string, string | null>): URLSearchParams {
  const next = new URLSearchParams(current);
  for (const [name, value] of Object.entries(changes)) {
    if (value === null || value === "") next.delete(name);
    else next.set(name, value);
  }
  return next;
}
const allowed = new Set(["q", "source", "replay", "mods", "result", "period", "sort", "beatmap", "starsMin", "starsMax", "accMin", "accMax", "ppMin", "ppMax", "mode", "provider", "order", "creator", "player", "status", "item", "itemProvider", "topic", "level", "video", "min", "max", "acc", "scoring", "bpmMin", "bpmMax", "lengthSecondsMin", "lengthSecondsMax", "approachRateMin", "approachRateMax", "circleSizeMin", "circleSizeMax", "overallDifficultyMin", "overallDifficultyMax"]);
export function supportsSavedFilters(path: string) {
  if (path === "/branding") return true;
  if (["/learn", "/live"].includes(path) || /^\/scenarios\/[^/]+$/.test(path)) return true;
  return ["/search", "/community", "/replays", "/leaderboard", "/benchmarks", "/osu", "/osu/beatmaps", "/osu/skins", "/osu/players", "/osu/community", "/osu/replays", "/osu/pp-targets", "/osu/learn"].includes(path)
    || /^\/(?:osu\/)?profiles\/[^/]+$/.test(path);
}
for (const name of ["direction", "view", "tab", "scenarioType", "category", "iconSize"]) allowed.add(name);
export function filterPreferenceQuery(search: string) {
  const clean = new URLSearchParams();
  for (const [name, value] of new URLSearchParams(search)) {
    if (allowed.has(name) && value.length <= 256 && !clean.has(name)) {
      clean.set(name, value);
      if (clean.toString().length > 4096) clean.delete(name);
    }
  }
  return clean.toString();
}
type StorageLike = Pick<Storage, "getItem" | "setItem">;
function read(storage: StorageLike): Record<string, string> {
  try {
    const raw = storage.getItem(key);
    if (!raw || raw.length > 200000) return {};
    const value = JSON.parse(raw);
    return value && typeof value === "object" && !Array.isArray(value) ? value : {};
  } catch { return {}; }
}
export function loadPageFilters(storage: StorageLike, path: string) {
  if (!supportsSavedFilters(path)) return "";
  const value = read(storage)[path];
  return typeof value === "string" ? filterPreferenceQuery(value) : "";
}
export function savePageFilters(storage: StorageLike, path: string, search: string) {
  if (!supportsSavedFilters(path)) return;
  try {
    const entries = Object.entries(read(storage)).filter(([name, value]) => name !== path && supportsSavedFilters(name) && typeof value === "string").slice(-39);
    storage.setItem(key, JSON.stringify(Object.fromEntries([...entries, [path, filterPreferenceQuery(search)]])));
  } catch { /* Preferences are optional when storage is disabled or full. */ }
}
