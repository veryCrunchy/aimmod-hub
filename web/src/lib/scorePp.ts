
export type ScorePpInput = {
  version: number; beatmapId: number; beatmapChecksum: string; rulesetId: number;
  lazer: boolean | null; mods: { acronym: string; settings?: Record<string, unknown> }[] | null;
  statistics: Record<string, number> | null; maximumStatistics: Record<string, number> | null;
  maxCombo: number; accuracy: number; passed: boolean; totalScore: number; legacyTotalScore: number | null;
};
export type ScorePpWorkerRequest = { id: string; input: ScorePpInput; url: string };
export type ScorePpResult = { pp: number; stars?: number; objectCount?: number };
export type ScorePpWorkerResponse = ({ id: string } & ScorePpResult) | { id: string; error: string };
export const scorePpVersion = "aimmod-osu-2026.730.0-v2-actual";
export const scorePpTTL = 24 * 60 * 60_000;
const prefix = `aimmod-score-pp-${scorePpVersion}:`;

function canonical(value: unknown, depth = 0): string {
  if (depth > 8) throw new Error("Calculation inputs are too deeply nested");
  if (value === null || typeof value === "boolean") return JSON.stringify(value);
  if (typeof value === "string" && value.length <= 256) return JSON.stringify(value);
  if (typeof value === "number" && Number.isFinite(value)) return JSON.stringify(value);
  if (Array.isArray(value) && value.length <= 64) return `[${value.map(item => canonical(item, depth + 1)).join(",")}]`;
  if (value && typeof value === "object" && Object.getPrototypeOf(value) === Object.prototype) {
    const entries = Object.entries(value).sort(([a], [b]) => a.localeCompare(b));
    if (entries.length <= 64) return `{${entries.map(([key, item]) => `${JSON.stringify(key)}:${canonical(item, depth + 1)}`).join(",")}}`;
  }
  throw new Error("Invalid calculation inputs");
}

export function validScorePp(pp: unknown): pp is number {
  return typeof pp === "number" && Number.isFinite(pp) && pp >= 0;
}

export function isUnsupportedScorePpRuleset(input: ScorePpInput): boolean {
  return ![0, 1, 2, 3].includes(input.rulesetId);
}

export function scorePpValidationReason(input: ScorePpInput): string | null {
  try {
    if (!input || input.version !== 1 || isUnsupportedScorePpRuleset(input)) return "This score mode or input version is not supported";
    if (!/^[a-f0-9]{32}$/i.test(input.beatmapChecksum) || !Number.isSafeInteger(input.beatmapId) || input.beatmapId <= 0) return "The exact beatmap revision is unavailable";
    if (typeof input.lazer !== "boolean") return "The score's stable or lazer rules are unknown";
    if (!Array.isArray(input.mods) || input.mods.some(mod => !mod || typeof mod.acronym !== "string" || !/^[A-Z0-9]{1,8}$/.test(mod.acronym))) return "The score's full mods are unavailable";
    if (!input.statistics || Array.isArray(input.statistics) || typeof input.statistics !== "object") return "Score judgements are unavailable";
    if (typeof input.passed !== "boolean") return "Score completion is unknown";
    const count = (value: unknown) => typeof value === "number" && Number.isInteger(value) && value >= 0 && value <= 0xffffffff;
    if (!count(input.maxCombo) || !Number.isSafeInteger(input.totalScore) || input.totalScore < 0 || (input.legacyTotalScore !== null && (!Number.isSafeInteger(input.legacyTotalScore) || input.legacyTotalScore < 0))) return "Score totals are invalid";
    if (!Number.isFinite(input.accuracy) || input.accuracy < 0 || input.accuracy > 1) return "Score accuracy is invalid";
    for (const stats of [input.statistics, input.maximumStatistics]) {
      if (stats === null) continue;
      if (Array.isArray(stats) || typeof stats !== "object" || !Object.values(stats).every(count)) return "Score judgements are invalid";
    }
    if (canonical(input).length > 8192) return "Score inputs are too large";
    return null;
  } catch { return "Score inputs are invalid"; }
}
export function canCalculateScorePp(input: ScorePpInput): boolean { return scorePpValidationReason(input) === null; }

export function validateScorePpObjectCount(input: ScorePpInput, objectCount: number): void {
  const reason = scorePpValidationReason(input);
  if (reason) throw new Error(reason);
  const stats = input.statistics!;
  const keys = input.rulesetId === 2 ? ["great", "large_tick_hit", "large_tick_miss", "small_tick_hit", "small_tick_miss", "miss"] : input.rulesetId === 3 ? ["perfect", "great", "good", "ok", "meh", "miss"] : ["great", "ok", "meh", "miss"];
  const judged = keys.reduce((sum, key) => sum + (stats[key] ?? 0), 0);
  if (!Number.isSafeInteger(objectCount) || objectCount < 0 || judged > objectCount || (input.passed && judged !== objectCount)) {
    throw new Error("Score judgements do not match this beatmap revision");
  }
}

export function scorePpCacheKey(input: ScorePpInput): string {
  if (!canCalculateScorePp(input)) return "";
  // Project explicit score inputs only: no user identity, URL, or unrelated response fields.
  const { version, beatmapChecksum, rulesetId, lazer, mods, statistics, maximumStatistics, maxCombo, accuracy, passed, totalScore, legacyTotalScore } = input;
  return prefix + canonical({ version, beatmapChecksum: beatmapChecksum.toLowerCase(), rulesetId, lazer, mods, statistics, maximumStatistics, maxCombo, accuracy, passed, totalScore, legacyTotalScore });
}

type StorageLike = Pick<Storage, "length" | "key" | "getItem" | "setItem" | "removeItem">;
export class ScorePpCache {
  constructor(private storage?: StorageLike, private now = Date.now) {}
  get(calculation: ScorePpInput): number | undefined { return this.getResult(calculation)?.pp; }
  getResult(calculation: ScorePpInput): ScorePpResult | undefined {
    const key = scorePpCacheKey(calculation);
    if (!key) return;
    try {
      const raw = this.storage?.getItem(key);
      if (!raw) return;
      if (raw.length > 256) { this.storage?.removeItem(key); return; }
      const value = JSON.parse(raw);
      if (!validScorePp(value.pp) || !Number.isFinite(value.expires) || value.expires <= this.now() || value.expires > this.now() + scorePpTTL) {
        this.storage?.removeItem(key); return;
      }
      return { pp: value.pp, stars: validScorePp(value.stars) ? value.stars : undefined, objectCount: Number.isSafeInteger(value.objectCount) && value.objectCount >= 0 ? value.objectCount : undefined };
    } catch { return; }
  }
  set(calculation: ScorePpInput, pp: number): void { this.setResult(calculation, { pp }); }
  setResult(calculation: ScorePpInput, result: ScorePpResult): void {
    const { pp, stars, objectCount } = result;
    const key = scorePpCacheKey(calculation);
    if (!key || !validScorePp(pp) || !this.storage) return;
    try {
      this.storage.removeItem(key);
      const entries: { key: string; expires: number; bytes: number }[] = [];
      for (let i = 0; i < this.storage.length; i++) {
        const other = this.storage.key(i);
        if (!other?.startsWith(prefix)) continue;
        const raw = this.storage.getItem(other) ?? "";
        let expires = 0;
        try { expires = Number(JSON.parse(raw).expires) || 0; } catch { /* Evict corrupt entries first. */ }
        entries.push({ key: other, expires, bytes: (other.length + raw.length) * 2 });
      }
      entries.sort((a, b) => a.expires - b.expires);
      const raw = JSON.stringify({ pp, stars, objectCount, expires: this.now() + scorePpTTL });
      let bytes = entries.reduce((sum, entry) => sum + entry.bytes, (key.length + raw.length) * 2);
      while (entries.length >= 400 || bytes > 2 * 1024 * 1024) {
        const oldest = entries.shift();
        if (!oldest) return;
        this.storage.removeItem(oldest.key);
        bytes -= oldest.bytes;
      }
      this.storage.setItem(key, raw);
    } catch { /* Calculation remains usable when browser storage is unavailable. */ }
  }
}
export function browserScorePpCache(): ScorePpCache {
  try { return new ScorePpCache(window.localStorage); } catch { return new ScorePpCache(); }
}
export function getCachedScorePp(input: ScorePpInput): number | undefined { return browserScorePpCache().get(input); }
export function setCachedScorePp(input: ScorePpInput, pp: number): void { browserScorePpCache().set(input, pp); }

export function getCachedScorePpResult(input: ScorePpInput): ScorePpResult | undefined { return browserScorePpCache().getResult(input); }
export function setCachedScorePpResult(input: ScorePpInput, result: ScorePpResult): void { browserScorePpCache().setResult(input, result); }

export function formatScorePp(pp: number): string {
  return pp > 0 && pp < 1 ? "<1pp" : `${Math.round(pp)}pp`;
}
