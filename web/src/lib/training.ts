import { API_BASE_URL } from "./config";
export const trainingModes = { steady: "Tapping accuracy", alternating: "Alternating", bursts: "Bursts", rhythm: "Rhythm changes", aim: "Aim control", reading: "Reading", reaction: "Reaction" } as const;
export type TrainingMode = keyof typeof trainingModes;
export type TrainingSession = {
  id: string; completedAt: string; visibility: "public" | "private";
  setup: { mode: TrainingMode; bpm: number; seconds: number; engine: string; randomized: boolean; musicSource: string };
  notes: number; hits: number; within25: number; extras: number; accuracy: number | null;
  meanMs: number | null; spreadMs: number | null; driftMs: number | null; responseMs: number | null;
  playedSeconds: number; peakNps: number | null;
};
export type TrainingSkill = { mode: TrainingMode; sessions: number; seconds: number; cleanSessions: number; cleanPeakNps: number | null; medianResponseMs: number | null; medianSpreadMs: number | null; responseChange: number | null; accuracyChange: number | null; spreadChange: number | null; comparedSessions: number; comparisonBpm: number };
export type TrainingProfile = { sessions: number; seconds: number; activeDays: number; days: number; truncated: boolean; skills: TrainingSkill[]; activity: { date: string; sessions: number; seconds: number }[]; recent: TrainingSession[] };
export async function fetchTraining(handle: string, days: number, mode: string, owner = false, signal?: AbortSignal): Promise<TrainingProfile> {
  const path = owner ? "me" : `profiles/${encodeURIComponent(handle)}`;
  const response = await fetch(`${API_BASE_URL}/api/osu/v1/training/${path}?${new URLSearchParams({ days: String(days), mode })}`, { signal, credentials: owner ? "include" : "omit" });
  if (!response.ok) throw new Error(response.status === 401 ? "Sign in to view your training." : "Training could not load. Please try again.");
  return response.json();
}
export async function setTrainingVisibility(id: string, visibility: "public" | "private") {
  const response = await fetch(`${API_BASE_URL}/api/osu/v1/training/visibility`, { method: "POST", credentials: "include", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ id, visibility }) });
  if (!response.ok) throw new Error("The sharing setting could not be changed. Please try again.");
}
export function formatPracticeTime(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  return minutes >= 60 ? `${Math.floor(minutes / 60)}h ${minutes % 60}m` : `${minutes}m`;
}
export function timingChange(value: number): string {
  return Math.abs(value) < 0.05 ? "Timing unchanged" : `${Math.abs(value).toFixed(1)} ms ${value < 0 ? "tighter" : "wider"}`;
}
export function trainingActivityDays(activity: TrainingProfile["activity"], days: number, now = new Date()) {
  const byDate = new Map(activity.map(day => [day.date, day]));
  const today = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  // A rolling window includes part of its first UTC date as well as today.
  return Array.from({ length: days + 1 }, (_, index) => {
    const date = new Date(today - (days - index) * 86_400_000).toISOString().slice(0, 10);
    return byDate.get(date) ?? { date, sessions: 0, seconds: 0 };
  });
}
