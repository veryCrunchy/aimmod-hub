import { API_BASE_URL } from "../lib/config";

export interface OsuAdminSummary {
  scores: number; public: number; unlisted: number; private: number;
  uploaded: number; pending: number; replayBytes: number; profiles: number;
  publicProfiles: number; activeCredentials: number; connectedAccounts: number; pendingDevices: number;
}
export interface OsuAdminShare {
  id: number; handle: string; username: string; title: string; difficulty: string;
  visibility: string; status: string; byteSize: number; createdAt: string; uploadedAt: string | null;
  userId: number; difficultyKey: string; accuracy: number; totalScore: number; performancePoints: number | null;
  maxCombo: number; misses: number; mods: string[]; passed: boolean; playedAt: string;
}
export interface OsuAdminAccount { provider: string; username: string; verified: boolean; createdAt: string }
export interface OsuAdminPlayer {
  userId: number; handle: string; displayName: string; osuUserId: number; username: string; country: string;
  createdAt: string; profileUpdatedAt: string | null; lastScoreAt: string | null;
  scores: number; public: number; unlisted: number; private: number; replays: number; replayBytes: number;
  activeCredentials: number; lastCredentialUse: string | null; accounts: OsuAdminAccount[];
}
export interface OsuAdminBeatmap {
  key: string; onlineId: number; setOnlineId: number; title: string; artist: string; creator: string; version: string;
  ruleset: string; stars: number; bpm: number; lengthMs: number; updatedAt: string; lastScoreAt: string | null;
  scores: number; players: number; public: number; unlisted: number; private: number; replays: number; replayBytes: number;
}
export interface OsuAdminRecords<T> { items: T[]; total: number }
export interface OsuAdminScope { label: string; userId?: number; difficultyKey?: string }

export function adminScoreParams(search: string, visibility: string, status: string, offset: number, scope?: OsuAdminScope): URLSearchParams {
  const params = new URLSearchParams({ q: search, visibility, status, offset: String(offset) });
  if (scope?.userId) params.set("userId", String(scope.userId));
  if (scope?.difficultyKey) params.set("difficultyKey", scope.difficultyKey);
  return params;
}
export interface OsuAdminOverview { summary: OsuAdminSummary; items: OsuAdminShare[]; total: number }
export interface OsuAdminProvider { name: string; configured: boolean; available: boolean; checkedAt: string; browserOnly: boolean }

export async function fetchOsuAdmin<T>(path: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(`${API_BASE_URL}/admin/osu/${path}`, { credentials: "include", cache: "no-store", signal });
  if (!response.ok) throw new Error(response.status === 401 || response.status === 403 ? "Admin access is required. Sign in again to continue." : "Could not load osu admin data. Try again.");
  return response.json() as Promise<T>;
}

export function adminBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 3);
  return `${(bytes / 1024 ** unit).toLocaleString(undefined, { maximumFractionDigits: 1 })} ${["B", "KiB", "MiB", "GiB"][unit]}`;
}
