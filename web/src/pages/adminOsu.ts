import { API_BASE_URL } from "../lib/config";

export interface OsuAdminSummary {
  scores: number; public: number; unlisted: number; private: number;
  uploaded: number; pending: number; replayBytes: number; profiles: number;
  publicProfiles: number; activeCredentials: number; connectedAccounts: number; pendingDevices: number;
}
export interface OsuAdminShare {
  id: number; handle: string; username: string; title: string; difficulty: string;
  visibility: string; status: string; byteSize: number; createdAt: string; uploadedAt: string | null;
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
