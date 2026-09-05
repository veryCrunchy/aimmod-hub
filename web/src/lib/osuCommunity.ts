import { API_BASE_URL } from "./config";
import { normalizeOsuReplayAnalysis } from "./osuReplayAnalysis";

export type OsuJudgementSummary = {
  great?: number;
  ok?: number;
  meh?: number;
  miss?: number;
  sliderBreaks?: number;
  other?: number;
};

export type OsuReplayJudgement = {
  objectIndex?: number | null;
  objectType?: string;
  startTimeMs?: number;
  result?: string;
  timeOffsetMs?: number;
  missAnalysis?: {
    reason?: string;
    confidence?: number;
    closestDistance?: number;
    distanceAtPress?: number | null;
    pressTimeOffsetMs?: number | null;
  } | null;
};

export type OsuReplayAnalysis = {
  timeBasis?: string;
  headlessAudioMuted?: boolean;
  summary?: OsuJudgementSummary;
  judgements?: OsuReplayJudgement[];
};

export type OsuSharedReplay = {
  source?: "local" | "official" | "merged";
  officialScoreId?: string;
  officialScoreUrl?: string;
  ppSource?: string;
  onlineScoreId?: number;
  officialReplayExists?: boolean;
  shareId: string;
  visibility: "public" | "unlisted";
  hubHandle: string;
  hubDisplayName: string;
  osuUserId: number;
  osuUsername: string;
  countryCode: string;
  avatarUrl: string;
  beatmapSetId: number;
  beatmapId: number;
  title: string;
  artist: string;
  creator: string;
  coverUrl: string;
  difficulty: string;
  ruleset: string;
  starRating: number;
  bpm: number;
  lengthMs: number;
  playedAt: string;
  totalScore: number;
  performancePoints?: number | null;
  accuracy: number;
  maxCombo: number;
  count300: number;
  count100: number;
  count50: number;
  countMiss: number;
  mods: string[];
  passed: boolean;
  hasReplayFile: boolean;
  analysisSchema: number;
  analysisEngine: string;
  analysis?: OsuReplayAnalysis;
};

export type OsuPublicProfile = {
  hubHandle: string;
  hubDisplayName: string;
  osuUserId: number;
  osuUsername: string;
  countryCode: string;
  avatarUrl: string;
  globalRank?: number | null;
  performancePoints?: number | null;
  playCount: number;
  playTimeSeconds: number;
  sharedReplayCount: number;
  recentReplays: OsuSharedReplay[];
};

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${path}`, { headers: { Accept: "application/json" } });
  if (!response.ok) {
    const detail = (await response.text()).trim();
    throw new Error(detail || `Request failed (${response.status}).`);
  }
  return response.json() as Promise<T>;
}

export async function fetchOsuCommunity(limit = 36): Promise<OsuSharedReplay[]> {
  const response = await fetchJSON<{ items?: OsuSharedReplay[] }>(`/api/osu/v1/community?limit=${Math.max(1, Math.min(100, limit))}`);
  return (response.items ?? []).map(normalizeReplay);
}

export async function fetchOsuProfile(handle: string, limit = 36): Promise<OsuPublicProfile> {
  const profile = await fetchJSON<OsuPublicProfile>(`/api/osu/v1/profiles/${encodeURIComponent(handle)}?limit=${Math.max(1, Math.min(100, limit))}`);
  return { ...profile, recentReplays: (profile.recentReplays ?? []).map(normalizeReplay) };
}

export async function fetchOsuReplay(shareId: string): Promise<OsuSharedReplay> {
  return normalizeReplay(await fetchJSON<OsuSharedReplay>(`/api/osu/v1/replays/${encodeURIComponent(shareId)}`));
}

export type OsuScoreHistory = {
  profile: OsuPublicProfile;
  items: OsuSharedReplay[];
  coverage: { best: { status: string; fetched: number }; recent: { status: string; fetched: number }; completeHistory: false };
  hasMore: boolean;
};
export async function fetchOsuScoreHistory(handle: string, mode = "osu"): Promise<OsuScoreHistory> {
  const history = await fetchJSON<OsuScoreHistory>(`/api/osu/v1/profile-scores/${encodeURIComponent(handle)}?mode=${encodeURIComponent(mode)}&limit=100`);
  return { ...history, items: (history.items ?? []).map(normalizeReplay) };
}
export async function fetchOsuOfficialScore(id: string): Promise<OsuSharedReplay> {
  const response = await fetchJSON<{ item?: OsuSharedReplay; replay?: { exists: boolean } }>(`/api/osu/v1/official-scores/${encodeURIComponent(id)}`);
  if (!response.item) throw new Error("This score is unavailable.");
  return normalizeReplay({ ...response.item, officialReplayExists: response.replay?.exists === true });
}

export function osuScorePath(replay: OsuSharedReplay): string {
  return replay.source === "official" && replay.officialScoreId
    ? `/osu/scores/${encodeURIComponent(replay.officialScoreId)}`
    : `/osu/replays/${encodeURIComponent(replay.shareId)}`;
}

function normalizeReplay(replay: OsuSharedReplay): OsuSharedReplay {
  return { ...replay, analysis: normalizeOsuReplayAnalysis(replay.analysis) };
}

export function osuReplayDownloadUrl(shareId: string): string {
  return `${API_BASE_URL}/media/osu-replays/${encodeURIComponent(shareId)}.osr`;
}

export function osuOfficialReplayDownloadUrl(id: string): string {
  return `${API_BASE_URL}/api/osu/v1/official-scores/${encodeURIComponent(id)}/replay`;
}

export function formatOsuAccuracy(value: number): string {
  return `${(Math.max(0, Math.min(1, value)) * 100).toFixed(2)}%`;
}

export function formatOsuMods(mods: string[]): string {
  return mods.length > 0 ? mods.join("") : "NM";
}

export function formatOsuDuration(milliseconds: number): string {
  const seconds = Math.max(0, Math.round(milliseconds / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}
