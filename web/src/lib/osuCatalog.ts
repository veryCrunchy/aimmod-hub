import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { OsuService } from "../gen/aimmod/osu/v1/osu_connect";
import { NumberRange, SkinProvider } from "../gen/aimmod/osu/v1/osu_pb";
import { API_BASE_URL } from "./config";

export const osuClient = createClient(OsuService, createConnectTransport({ baseUrl: API_BASE_URL, useBinaryFormat: false }));
export const rulesets = [["0", "All modes"], ["1", "osu!"], ["2", "osu!taiko"], ["3", "osu!catch"], ["4", "osu!mania"]] as const;
export function modeName(mode: number) { return rulesets.find(([id]) => Number(id) === mode)?.[1] ?? "Unknown mode"; }

export function beatmapLinks(id: string) {
  if (!/^[1-9]\d{0,9}$/.test(id) || Number(id) > 2147483647) return null;
  return { osu: `osu://dl/${id}`, aimmod: `aimmod-osu://beatmapsets/${id}`, source: `https://osu.ppy.sh/beatmapsets/${id}` };
}

export function skinSource(provider: SkinProvider) {
  if (provider === SkinProvider.OSU_SKINS) return { name: "osuskins.net", slug: "osuskins", url: "https://osuskins.net" };
  if (provider === SkinProvider.OSUCK) return { name: "skins.osuck.net", slug: "osuck", url: "https://skins.osuck.net" };
  return null;
}

export function skinLinks(provider: SkinProvider, id: string) {
  const source = skinSource(provider);
  if (!source || !/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/.test(id)) return null;
  if (provider === SkinProvider.OSU_SKINS && !/^[A-Za-z0-9]{7}$/.test(id)) return null;
  return { aimmod: `aimmod-osu://skins/${source.slug}/${id}`, source: provider === SkinProvider.OSU_SKINS ? `${source.url}/skin/${id}` : source.url };
}

export function mediaUrl(value: string) {
  try { const url = new URL(value.startsWith("//") ? `https:${value}` : value); return url.protocol === "https:" && !url.username && !url.password ? url.href : undefined; } catch { return undefined; }
}

export function numberRange(minimum: string, maximum: string): NumberRange | undefined {
  if (!minimum && !maximum) return undefined;
  const min = minimum ? Number(minimum) : undefined;
  const max = maximum ? Number(maximum) : undefined;
  if ([min, max].some(value => value !== undefined && (!Number.isFinite(value) || value < 0)) || (min !== undefined && max !== undefined && min > max)) throw new Error("Enter a valid range, with minimum no greater than maximum.");
  return new NumberRange({ minimum: min, maximum: max });
}

export class CatalogCache {
  private entries = new Map<string, { value: unknown; expires: number }>();
  constructor(private limit = 64, private ttl = 120_000, private now = Date.now) {}
  get<T>(key: string): T | undefined {
    const entry = this.entries.get(key);
    if (!entry) return undefined;
    if (entry.expires <= this.now()) { this.entries.delete(key); return undefined; }
    return entry.value as T;
  }
  delete(key: string) { this.entries.delete(key); }
  set<T>(key: string, value: T) {
    this.entries.delete(key);
    this.entries.set(key, { value, expires: this.now() + this.ttl });
    if (this.entries.size > this.limit) this.entries.delete(this.entries.keys().next().value!);
  }
}
export const catalogCache = new CatalogCache();

export async function catalogRequest<T>(key: string, signal: AbortSignal, request: (signal: AbortSignal) => Promise<T>, cache = catalogCache): Promise<T> {
  signal.throwIfAborted();
  const cached = cache.get<T>(key);
  if (cached !== undefined) return cached;
  const result = await request(signal);
  signal.throwIfAborted();
  const status = result as { providers?: { available: boolean }[]; provider?: { available: boolean } };
  if (!status.providers?.some(provider => !provider.available) && status.provider?.available !== false) cache.set(key, result);
  return result;
}
