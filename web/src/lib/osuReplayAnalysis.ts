import type { OsuReplayAnalysis, OsuReplayJudgement, OsuJudgementSummary } from "./osuCommunity";

// AimMod.Osu.Runtime.Contracts.ReplayMissReason is serialized numerically by native uploads.
const missReasons = ["Unknown", "EarlyClick", "LateClick", "Undershoot", "Overshoot", "OnTargetNoClick", "AimDeviation"] as const;

export function normalizeOsuMissReason(value: unknown): string {
  if (typeof value === "number") return Number.isInteger(value) ? missReasons[value] ?? "Unknown" : "Unknown";
  if (typeof value !== "string" || !value.trim()) return "Unknown";
  const text = value.trim();
  if (/^-?\d+$/.test(text)) return normalizeOsuMissReason(Number(text));
  const key = text.replace(/[ _-]/g, "").toLowerCase();
  return missReasons.find(reason => reason.toLowerCase() === key) ?? text;
}

export function formatOsuMissReason(value: unknown): string {
  const reason = normalizeOsuMissReason(value);
  if (reason === "Unknown") return "Unclassified miss";
  return reason.replace(/([a-z])([A-Z])/g, "$1 $2").replaceAll("_", " ").replace(/^./, letter => letter.toUpperCase());
}

function record(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

function number(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function text(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

export function normalizeOsuReplayAnalysis(value: unknown): OsuReplayAnalysis | undefined {
  const analysis = record(value);
  if (!analysis) return undefined;
  const rawSummary = record(analysis.summary);
  let summary: OsuJudgementSummary | undefined;
  if (rawSummary) {
    summary = {};
    for (const key of ["great", "ok", "meh", "miss", "sliderBreaks", "other"] as const) summary[key] = number(rawSummary[key]);
  }
  const judgements: OsuReplayJudgement[] = [];
  for (const value of Array.isArray(analysis.judgements) ? analysis.judgements : []) {
    const item = record(value);
    if (!item) continue;
    const miss = record(item.missAnalysis);
    const confidence = number(miss?.confidence);
    judgements.push({
      objectIndex: item.objectIndex === null ? null : number(item.objectIndex),
      objectType: text(item.objectType),
      startTimeMs: number(item.startTimeMs),
      result: text(item.result),
      timeOffsetMs: number(item.timeOffsetMs),
      missAnalysis: miss ? {
        reason: normalizeOsuMissReason(miss.reason),
        confidence: confidence === undefined ? undefined : Math.max(0, Math.min(1, confidence)),
        closestDistance: number(miss.closestDistance),
        distanceAtPress: miss.distanceAtPress === null ? null : number(miss.distanceAtPress),
        pressTimeOffsetMs: miss.pressTimeOffsetMs === null ? null : number(miss.pressTimeOffsetMs),
      } : null,
    });
  }
  return { timeBasis: text(analysis.timeBasis), headlessAudioMuted: typeof analysis.headlessAudioMuted === "boolean" ? analysis.headlessAudioMuted : undefined, summary, judgements };
}
