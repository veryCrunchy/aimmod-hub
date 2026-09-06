import { BeatmapDifficulty, Ruleset } from "../gen/aimmod/osu/v1/osu_pb";

export type PpResult = { pp: number; maxPp: number; stars: number; error?: string };
export type PpSettings = { query: string; low: string; high: string; accuracy: number; mods: string; lazer: boolean; sort: string };
export const ppMods = ["NM", "HD", "HR", "HDHR", "DT", "HDDT", "HT"];
export const candidatePrefix = "aimmod-pp-candidates-v1:";
export const calculationPrefix = "aimmod-pp-osu-2026.730.0-fc-v1:";
export const candidateTTL = 15 * 60_000;
const calculationTTL = 86400_000;
type StorageLike = Pick<Storage, "getItem" | "setItem" | "removeItem" | "key" | "length">;

export function readPpSettings(params: URLSearchParams): PpSettings {
  const bound = (key: string, fallback: string) => {
    const value = params.get(key);
    if (value === null) return fallback;
    if (value === "") return "";
    const number = Number(value);
    return Number.isFinite(number) && number >= 0 && number <= 10 ? String(Math.round(number * 10) / 10) : fallback;
  };
  let low = bound("min", "3"), high = bound("max", "7");
  if (low && high && Number(low) > Number(high)) [low, high] = [high, low];
  const accuracy = Number(params.get("acc") ?? 98);
  return {
    query: (params.get("q") ?? "").slice(0, 256), low, high,
    accuracy: Number.isFinite(accuracy) && accuracy >= 80 && accuracy <= 100 ? Math.round(accuracy * 10) / 10 : 98,
    mods: ppMods.includes(params.get("mods") ?? "") ? params.get("mods")! : "NM",
    lazer: params.get("scoring") !== "stable",
    sort: ["pp", "max", "stars"].includes(params.get("sort") ?? "") ? params.get("sort")! : "pp",
  };
}

export function candidateKey(settings: Pick<PpSettings, "query" | "low" | "high">): string {
  return candidatePrefix + JSON.stringify([settings.query.trim(), settings.low, settings.high, "osu", "ranked", "plays_desc", 12]);
}

export function validChecksum(checksum: string): boolean { return /^[a-f0-9]{32}$/i.test(checksum); }
export function validPpResult(value: unknown): value is PpResult {
  const result = value as PpResult | null;
  return !!result && !result.error && [result.pp, result.maxPp, result.stars].every(number => typeof number === "number" && Number.isFinite(number) && number >= 0);
}

export function browserPpCache(): PpTargetCache {
  try { return new PpTargetCache(window.localStorage); } catch { return new PpTargetCache(); }
}

export class PpTargetCache {
  constructor(private storage?: StorageLike, private now = Date.now) {}

  private keys(prefix: string): string[] {
    const keys: string[] = [];
    for (let i = 0; i < (this.storage?.length ?? 0); i++) {
      const key = this.storage!.key(i);
      if (key?.startsWith(prefix)) keys.push(key);
    }
    return keys;
  }

  private read(key: string, ttl: number, maxBytes: number): any {
    try {
      const raw = this.storage?.getItem(key);
      if (!raw) return;
      if (raw.length * 2 > maxBytes) { this.storage?.removeItem(key); return; }
      const value = JSON.parse(raw);
      if (!Number.isFinite(value?.expires) || value.expires <= this.now() || value.expires > this.now() + ttl) {
        this.storage?.removeItem(key); return;
      }
      return value;
    } catch { return; }
  }

  private write(key: string, value: unknown, prefix: string, limit: number, bytes: number): void {
    try {
      const raw = JSON.stringify(value);
      if (raw.length * 2 > bytes) return;
      this.storage?.removeItem(key);
      const entries = this.keys(prefix).map(key => ({ key, raw: this.storage!.getItem(key) ?? "" }));
      entries.sort((a, b) => {
        const expiry = (raw: string) => { try { return Number(JSON.parse(raw).expires) || 0; } catch { return 0; } };
        return expiry(a.raw) - expiry(b.raw);
      });
      let total = raw.length * 2 + entries.reduce((sum, entry) => sum + entry.raw.length * 2, 0);
      while (entries.length >= limit || total > bytes) {
        const oldest = entries.shift();
        if (!oldest) break;
        this.storage?.removeItem(oldest.key); total -= oldest.raw.length * 2;
      }
      this.storage?.setItem(key, raw);
    } catch { /* Storage is optional; fresh results remain usable. */ }
  }

  getCandidates(key: string): BeatmapDifficulty[] | undefined {
    const value = this.read(key, candidateTTL, 2 << 20);
    if (!value || value.version !== 1 || !Array.isArray(value.maps) || value.maps.length > 512) return;
    try {
      const maps = value.maps.map((map: unknown) => BeatmapDifficulty.fromJson(map as Parameters<typeof BeatmapDifficulty.fromJson>[0]));
      const ids = new Set<string>();
      for (const map of maps) {
        if (!/^[1-9]\d{0,9}$/.test(map.beatmapId) || !/^[1-9]\d{0,9}$/.test(map.beatmapsetId) || map.ruleset !== Ruleset.OSU || ids.has(map.beatmapId)
          || (map.checksum !== "" && !validChecksum(map.checksum)) || ![map.stars, map.bpm, map.lengthSeconds].every(n => Number.isFinite(n) && n >= 0)) return;
        ids.add(map.beatmapId);
      }
      return maps;
    } catch { return; }
  }

  setCandidates(key: string, maps: BeatmapDifficulty[]): void {
    if (maps.length > 512) return;
    try {
      // A newer discovery invalidates other searches containing the older map
      // revision. Never relabel old calculated PP with the new checksum.
      const revisions = new Map(maps.map(map => [map.beatmapId, map.checksum]));
      for (const oldKey of this.keys(candidatePrefix)) {
        const old = this.getCandidates(oldKey);
        if (!old || old.some(map => revisions.has(map.beatmapId) && revisions.get(map.beatmapId) !== map.checksum)) this.storage?.removeItem(oldKey);
      }
      this.write(key, { version: 1, expires: this.now() + candidateTTL, maps: maps.map(map => map.toJson()) }, candidatePrefix, 16, 2 << 20);
    } catch { /* Ignore unavailable storage or malformed metadata. */ }
  }

  deleteCandidates(key: string): void { try { this.storage?.removeItem(key); } catch { /* Optional storage. */ } }

  private resultKey(map: BeatmapDifficulty, settings: Pick<PpSettings, "accuracy" | "mods" | "lazer">): string {
    return `${calculationPrefix}${map.beatmapId}:${map.checksum}:${settings.accuracy}:${settings.mods}:${settings.lazer}`;
  }

  getResult(map: BeatmapDifficulty, settings: PpSettings): PpResult | undefined {
    if (!validChecksum(map.checksum)) return;
    const result = this.read(this.resultKey(map, settings), calculationTTL, 4096);
    return validPpResult(result) ? { pp: result.pp, maxPp: result.maxPp, stars: result.stars } : undefined;
  }

  setResult(map: BeatmapDifficulty, settings: PpSettings, result: PpResult): void {
    if (!validChecksum(map.checksum) || !validPpResult(result)) return;
    this.write(this.resultKey(map, settings), { ...result, expires: this.now() + calculationTTL }, calculationPrefix, 400, 512 << 10);
  }
}
