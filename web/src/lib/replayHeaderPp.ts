import type { ReplayData } from "replayviewer-js";
import { scorePpValidationReason, validateScorePpObjectCount, type ScorePpInput } from "./scorePp";

type ReplayHeader = Pick<ReplayData, "mode" | "gameVersion" | "beatmapHash" | "count300" | "count100" | "count50" | "countMiss" | "maxCombo" | "score" | "mods" | "scoreInfo">;
export type ReplayHeaderPpResult = { input: ScorePpInput; reason?: never } | { input?: never; reason: string };
const stableMods: readonly [number, string][] = [[1, "NF"], [2, "EZ"], [4, "TD"], [8, "HD"], [16, "HR"], [32, "SD"], [64, "DT"], [256, "HT"], [512, "NC"], [1024, "FL"], [4096, "SO"], [16384, "PF"]];
const supportedBits = stableMods.reduce((bits, [flag]) => bits | flag, 0);

// Call only after the playback decoder verifies the .osu bytes against the .osr hash.
// osu! LegacyScoreEncoder/Decoder define FIRST_LAZER_VERSION as 30000000.
export function replayHeaderScorePp(header: ReplayHeader, context: { beatmapId: number; objectCount: number; passed: boolean }): ReplayHeaderPpResult {
  if (header.mode !== 0) return { reason: "Replay PP calculation is available only for osu!standard." };
  if (!Number.isSafeInteger(header.gameVersion) || header.gameVersion <= 0) return { reason: "The replay's scoring version is unavailable." };
  if (header.gameVersion >= 30000000 || header.scoreInfo !== undefined) {
    return { reason: "This replay needs full lazer score statistics and mod settings to calculate PP." };
  }
  if (!Number.isInteger(header.mods) || header.mods < 0 || header.mods > 0x7fffffff || (header.mods & ~supportedBits) !== 0) return { reason: "PP calculation for these replay mods is not supported." };
  if (((header.mods & (64 | 512)) && (header.mods & 256)) || ((header.mods & 2) && (header.mods & 16))) return { reason: "The replay contains conflicting difficulty mods." };
  const counts = [header.count300, header.count100, header.count50, header.countMiss];
  if (![...counts, header.maxCombo].every(value => Number.isInteger(value) && value >= 0 && value <= 65535)) return { reason: "The replay's hit counts are invalid." };
  const total = counts.reduce((sum, value) => sum + value, 0);
  if (total === 0) return { reason: "The replay has no judged objects." };
  const mods = stableMods.filter(([flag]) => (header.mods & flag) !== 0)
    .filter(([flag]) => !(flag === 64 && (header.mods & 512)) && !(flag === 32 && (header.mods & 16384)))
    .map(([, acronym]) => ({ acronym }));
  const input: ScorePpInput = {
    version: 1, beatmapId: context.beatmapId, beatmapChecksum: header.beatmapHash, rulesetId: 0, lazer: false,
    mods, statistics: { great: header.count300, ok: header.count100, meh: header.count50, miss: header.countMiss },
    maximumStatistics: null, maxCombo: header.maxCombo,
    accuracy: (header.count300 * 300 + header.count100 * 100 + header.count50 * 50) / (total * 300),
    passed: context.passed, totalScore: header.score, legacyTotalScore: header.score,
  };
  const reason = scorePpValidationReason(input);
  if (reason) return { reason };
  try { validateScorePpObjectCount(input, context.objectCount); }
  catch { return { reason: "The replay's judgement counts do not match the verified beatmap." }; }
  return { input };
}
