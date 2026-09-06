import { API_BASE_URL } from "./config";
import type { ScorePpInput } from "./scorePp";

export const ppEngine = "aimmod-osu-2026.730.0-v2";
export async function calculateOfficialPp(bytes: Uint8Array, checksum: string, settings: { accuracy: number; mods: string; lazer: boolean } | ScorePpInput) {
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 8192) binary += String.fromCharCode(...bytes.subarray(offset, offset + 8192));
  const actual = "statistics" in settings;
  const response = await fetch(`${API_BASE_URL}/api/osu/v1/pp/calculate`, {
    method: "POST", headers: { "Content-Type": "application/json" }, signal: AbortSignal.timeout(25000),
    body: JSON.stringify({ map: btoa(binary), checksum, lazer: settings.lazer, accuracy: actual ? settings.accuracy * 100 : settings.accuracy,
      mods: actual ? settings.mods : settings.mods === "NM" ? [] : settings.mods.match(/.{2}/g)!.map(acronym => ({ acronym })), input: actual ? settings : undefined }),
  });
  const result = await response.json().catch(() => null);
  if (!response.ok) throw new Error(result?.error || "PP calculation is temporarily unavailable. Please retry.");
  if (result?.engine !== ppEngine || ![result.pp, result.maxPp, result.stars].every(value => typeof value === "number" && Number.isFinite(value) && value >= 0)) throw new Error("PP calculation returned an invalid result.");
  return result as { pp: number; maxPp: number; stars: number; objectCount: number; accuracy: number; lazer: boolean; engine: string };
}
