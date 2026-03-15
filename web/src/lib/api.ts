import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { API_BASE_URL } from "./config";
import {
  GetBenchmarkLeaderboardRequest,
  GetBenchmarkPageRequest,
  GetMousePathRequest,
  GetOverviewRequest,
  GetProfileRequest,
  GetReplayMediaRequest,
  GetRunRequest,
  GetScenarioPageRequest,
  ListBenchmarksRequest,
  ListReplaysRequest,
  SearchRequest,
  SessionSummaryValue,
} from "../gen/aimmod/hub/v1/hub_pb";
import { HubService } from "../gen/aimmod/hub/v1/hub_connect";

const transport = createConnectTransport({
  baseUrl: API_BASE_URL
});

export const hubClient = createClient(HubService, transport);

export function summaryValueToString(value?: SessionSummaryValue) {
  if (!value?.kind) return null;
  switch (value.kind.case) {
    case "stringValue":
      return value.kind.value;
    case "numberValue":
      return value.kind.value.toString();
    case "boolValue":
      return value.kind.value ? "Yes" : "No";
    default:
      return null;
  }
}

export function summaryValueToNumber(value?: SessionSummaryValue) {
  return value?.kind?.case === "numberValue" ? value.kind.value : null;
}

export function formatDurationMs(durationMs: bigint | number) {
  const totalSeconds = Math.max(0, Number(durationMs) / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = Math.round(totalSeconds % 60);
  if (minutes <= 0) return `${seconds}s`;
  return `${minutes}m ${seconds.toString().padStart(2, "0")}s`;
}

export function slugifyScenarioName(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

const SCENARIO_TYPE_LABELS: Record<string, string> = {
  Tracking: "Tracking",
  TargetSwitching: "Target switching",
  StaticClicking: "Static clicking",
  DynamicClicking: "Dynamic clicking",
  MovingClicking: "Dynamic clicking",
  OneShotClicking: "Static clicking",
  MultiHitClicking: "Target switching",
  ReactiveClicking: "Dynamic clicking",
  AccuracyDrill: "Accuracy drill",
};

export const SCENARIO_TYPE_ACCENTS: Record<string, string> = {
  Tracking: "border-cyan/25 text-cyan",
  TargetSwitching: "border-gold/25 text-gold",
  StaticClicking: "border-mint/25 text-mint",
  DynamicClicking: "border-violet/25 text-violet",
  MovingClicking: "border-violet/25 text-violet",
  OneShotClicking: "border-mint/25 text-mint",
  MultiHitClicking: "border-gold/25 text-gold",
  ReactiveClicking: "border-violet/25 text-violet",
  AccuracyDrill: "border-line text-muted",
};

export function displayScenarioType(value?: string | null): string | null {
  const normalized = value?.trim();
  if (!normalized || normalized === "Unknown") return null;
  return SCENARIO_TYPE_LABELS[normalized] ?? normalized;
}

export function scenarioTypeAccent(value?: string | null): string {
  const normalized = value?.trim();
  if (!normalized) return "border-line text-muted";
  return SCENARIO_TYPE_ACCENTS[normalized] ?? "border-line text-muted";
}

export type HubSearchScenario = {
  scenarioName: string;
  scenarioSlug: string;
  scenarioType: string;
  runCount: number;
};

export type HubSearchProfile = {
  userHandle: string;
  userDisplayName: string;
  avatarURL: string;
  isVerified: boolean;
  runCount: number;
  scenarioCount: number;
  primaryScenarioType: string;
};

export type HubSearchRun = {
  publicRunID: string;
  sessionID: string;
  scenarioSlug: string;
  scenarioName: string;
  scenarioType: string;
  playedAt: string;
  score: number;
  accuracy: number;
  durationMS: number;
  userHandle: string;
  userDisplayName: string;
  hasVideo: boolean;
  hasMousePath: boolean;
  replayQuality: string;
};

export type HubSearchBenchmark = {
  benchmarkId: number;
  benchmarkName: string;
  benchmarkIconUrl: string;
  benchmarkAuthor: string;
  benchmarkType: string;
  playerCount: number;
};

export type HubSearchResponse = {
  query: string;
  scenarios: HubSearchScenario[];
  profiles: HubSearchProfile[];
  runs: HubSearchRun[];
  replays: HubSearchRun[];
  benchmarks: HubSearchBenchmark[];
};

function mapSearchProfileResult(
  profile: {
    userHandle?: string;
    userDisplayName?: string;
    avatarUrl?: string;
    isVerified?: boolean;
    runCount?: number;
    scenarioCount?: number;
    primaryScenarioType?: string;
  },
): HubSearchProfile {
  return {
    userHandle: profile.userHandle ?? "",
    userDisplayName: profile.userDisplayName ?? "",
    avatarURL: profile.avatarUrl ?? "",
    isVerified: Boolean(profile.isVerified),
    runCount: profile.runCount ?? 0,
    scenarioCount: profile.scenarioCount ?? 0,
    primaryScenarioType: profile.primaryScenarioType ?? "",
  };
}

function mapReplayPreview(
  run: {
    publicRunId?: string;
    sessionId?: string;
    scenarioSlug?: string;
    scenarioName?: string;
    scenarioType?: string;
    playedAtIso?: string;
    score?: number;
    accuracy?: number;
    durationMs?: bigint | number;
    userHandle?: string;
    userDisplayName?: string;
    hasVideo?: boolean;
    hasMousePath?: boolean;
    replayQuality?: string;
  },
): HubSearchRun {
  return {
    publicRunID: run.publicRunId ?? "",
    sessionID: run.sessionId ?? "",
    scenarioSlug: run.scenarioSlug ?? "",
    scenarioName: run.scenarioName ?? "",
    scenarioType: run.scenarioType ?? "",
    playedAt: run.playedAtIso ?? "",
    score: run.score ?? 0,
    accuracy: run.accuracy ?? 0,
    durationMS: Number(run.durationMs ?? 0),
    userHandle: run.userHandle ?? "",
    userDisplayName: run.userDisplayName ?? "",
    hasVideo: Boolean(run.hasVideo),
    hasMousePath: Boolean(run.hasMousePath),
    replayQuality: run.replayQuality ?? "",
  };
}

export type ReplayListResponse = {
  query: string;
  scenarioName: string;
  userHandle: string;
  items: HubSearchRun[];
};

export type ReplayMediaMeta = {
  available: boolean;
  runId?: string;
  quality?: string;
  contentType?: string;
  byteSize?: number;
  mediaUrl?: string;
};

export type MousePathPoint = {
  x: number;
  y: number;
  timestampMs: number;
  isClick: boolean;
};

export type MousePathMeta = {
  available: boolean;
  points: MousePathPoint[];
  hitTimestampsMs: number[];
  playbackOffsetMs: number;
  videoOffsetMs: number;
};

export type AdminVersionBreakdown = {
  label: string;
  runCount: number;
};

export type AdminScenarioIssue = {
  scenarioName: string;
  scenarioSlug: string;
  runCount: number;
};

export type AdminRecentIngest = {
  publicRunId: string;
  sessionId: string;
  sourceSessionId: string;
  scenarioName: string;
  scenarioSlug: string;
  scenarioType: string;
  userHandle: string;
  userDisplayName: string;
  playedAt: string;
  ingestedAt: string;
  score: number;
};

export type AdminOverviewResponse = {
  totalRuns: number;
  totalPlayers: number;
  totalScenarios: number;
  unknownTypeRuns: number;
  missingSummaryRuns: number;
  missingFeatureRuns: number;
  missingTimelineRuns: number;
  missingContextRuns: number;
  zeroScoreRuns: number;
  missingSourceSessionRuns: number;
  appVersions: AdminVersionBreakdown[];
  schemaVersions: AdminVersionBreakdown[];
  topUnknownScenarios: AdminScenarioIssue[];
  recentIngests: AdminRecentIngest[];
  userSyncHealth: {
    userHandle: string;
    userDisplayName: string;
    runCount: number;
    unknownTypeRuns: number;
    missingTimelineRuns: number;
    missingContextRuns: number;
    zeroScoreRuns: number;
    lastPlayedAt: string;
    lastIngestedAt: string;
  }[];
  recentFailures: {
    id: number;
    userExternalId: string;
    userHandle: string;
    userDisplayName: string;
    sessionId: string;
    publicRunId: string;
    scenarioName: string;
    errorMessage: string;
    createdAt: string;
  }[];
};

export type AdminUserDetailResponse = {
  userHandle: string;
  userDisplayName: string;
  aimmodUserId: string;
  legacyExternalId: string;
  profileHandle: string;
  avatarUrl: string;
  isVerified: boolean;
  runCount: number;
  scenarioCount: number;
  unknownTypeRuns: number;
  missingTimelineRuns: number;
  missingContextRuns: number;
  zeroScoreRuns: number;
  lastPlayedAt: string;
  lastIngestedAt: string;
  lastAppVersion: string;
  lastSchemaVersion: number;
  appVersions: AdminVersionBreakdown[];
  schemaVersions: AdminVersionBreakdown[];
  linkedAccounts: {
    provider: string;
    providerAccountId: string;
    username: string;
    displayName: string;
    avatarUrl: string;
    verified: boolean;
    createdAt: string;
    updatedAt: string;
  }[];
  topUnknownScenarios: AdminScenarioIssue[];
  recentFailures: {
    id: number;
    userExternalId: string;
    userHandle: string;
    userDisplayName: string;
    sessionId: string;
    publicRunId: string;
    scenarioName: string;
    errorMessage: string;
    createdAt: string;
  }[];
  recentRuns: {
    publicRunId: string;
    scenarioName: string;
    scenarioSlug: string;
    scenarioType: string;
    playedAt: string;
    score: number;
    accuracy: number;
    durationMs: number;
  }[];
};

export async function fetchRun(runId: string) {
  return hubClient.getRun(new GetRunRequest({ runId }));
}

export async function fetchOverview() {
  return hubClient.getOverview(new GetOverviewRequest());
}

export async function fetchScenarioPage(slug: string) {
  return hubClient.getScenarioPage(new GetScenarioPageRequest({ slug }));
}

export async function fetchProfile(handle: string) {
  return hubClient.getProfile(new GetProfileRequest({ handle }));
}

export async function fetchBenchmarkPage(handle: string, benchmarkId: number) {
  return hubClient.getBenchmarkPage(new GetBenchmarkPageRequest({ handle, benchmarkId }));
}

export async function fetchBenchmarkList() {
  return hubClient.listBenchmarks(new ListBenchmarksRequest());
}

export async function fetchBenchmarkLeaderboard(benchmarkId: number) {
  return hubClient.getBenchmarkLeaderboard(new GetBenchmarkLeaderboardRequest({ benchmarkId }));
}

export function formatRelativeTime(dateStr: string | undefined | null): string {
  if (!dateStr) return "";
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return "";
  const diffMs = Date.now() - date.getTime();
  const diffSecs = Math.floor(diffMs / 1000);
  if (diffSecs < 60) return "just now";
  const diffMins = Math.floor(diffSecs / 60);
  if (diffMins < 60) return `${diffMins}m ago`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) return `${diffDays}d ago`;
  const diffMonths = Math.floor(diffDays / 30);
  if (diffMonths < 12) return `${diffMonths}mo ago`;
  return `${Math.floor(diffMonths / 12)}y ago`;
}

export async function searchHub(query: string): Promise<HubSearchResponse> {
  const payload = await hubClient.search(new SearchRequest({ query: query.trim() }));
  return {
    query: payload.query ?? query,
    scenarios: Array.isArray(payload.scenarios) ? payload.scenarios : [],
    profiles: Array.isArray(payload.profiles) ? payload.profiles.map(mapSearchProfileResult) : [],
    runs: Array.isArray(payload.runs) ? payload.runs.map(mapReplayPreview) : [],
    replays: Array.isArray(payload.replays) ? payload.replays.map(mapReplayPreview) : [],
    benchmarks: (payload.benchmarks ?? []).map((b) => ({
      benchmarkId: b.benchmarkId ?? 0,
      benchmarkName: b.benchmarkName ?? "",
      benchmarkIconUrl: b.benchmarkIconUrl ?? "",
      benchmarkAuthor: b.benchmarkAuthor ?? "",
      benchmarkType: b.benchmarkType ?? "",
      playerCount: b.playerCount ?? 0,
    })),
  };
}

export async function fetchReplayHub(params?: {
  query?: string;
  scenarioName?: string;
  handle?: string;
  limit?: number;
}): Promise<ReplayListResponse> {
  const payload = await hubClient.listReplays(
    new ListReplaysRequest({
      query: params?.query?.trim() ?? "",
      scenarioName: params?.scenarioName?.trim() ?? "",
      handle: params?.handle?.trim() ?? "",
      limit: params?.limit ?? 0,
    }),
  );
  return {
    query: payload.query ?? params?.query ?? "",
    scenarioName: payload.scenarioName ?? params?.scenarioName ?? "",
    userHandle: payload.userHandle ?? params?.handle ?? "",
    items: Array.isArray(payload.items) ? payload.items.map(mapReplayPreview) : [],
  };
}

export async function fetchReplayMediaMeta(runId: string): Promise<ReplayMediaMeta> {
  const payload = await hubClient.getReplayMedia(new GetReplayMediaRequest({ runId }));
  return {
    available: Boolean(payload.available),
    runId: payload.runId,
    quality: payload.quality,
    contentType: payload.contentType,
    byteSize: payload.byteSize ? Number(payload.byteSize) : undefined,
    mediaUrl: payload.mediaUrl ? resolveHubMediaUrl(payload.mediaUrl) : undefined,
  };
}

export async function deleteReplayMedia(runId: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/media/replays/delete?runId=${encodeURIComponent(runId)}`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error((await response.text()) || "Could not delete replay media.");
  }
}

export async function fetchMousePath(runId: string): Promise<MousePathMeta> {
  const payload = await hubClient.getMousePath(new GetMousePathRequest({ runId }));
  const result = {
    available: Boolean(payload.available),
    points: Array.isArray(payload.points)
      ? payload.points.map((point) => ({
          x: point.x ?? 0,
          y: point.y ?? 0,
          timestampMs: Number(point.timestampMs ?? 0),
          isClick: Boolean(point.isClick),
        }))
      : [],
    hitTimestampsMs: Array.isArray(payload.hitTimestampsMs)
      ? payload.hitTimestampsMs.map((value) => Number(value))
      : [],
    playbackOffsetMs: Number(payload.playbackOffsetMs ?? 0),
    videoOffsetMs: Number(payload.videoOffsetMs ?? 0),
  };
  const firstPointMs = result.points.length > 0 ? result.points[0]?.timestampMs ?? 0 : 0;
  const lastPointMs =
    result.points.length > 0 ? result.points[result.points.length - 1]?.timestampMs ?? 0 : 0;
  const firstHitMs = result.hitTimestampsMs.length > 0 ? result.hitTimestampsMs[0] ?? 0 : 0;
  const lastHitMs =
    result.hitTimestampsMs.length > 0
      ? result.hitTimestampsMs[result.hitTimestampsMs.length - 1] ?? 0
      : 0;
  console.info("[aimmod-hub] fetchMousePath", {
    runId,
    available: result.available,
    pointCount: result.points.length,
    hitCount: result.hitTimestampsMs.length,
    playbackOffsetMs: result.playbackOffsetMs,
    videoOffsetMs: result.videoOffsetMs,
    firstPointMs,
    lastPointMs,
    firstHitMs,
    lastHitMs,
  });
  return result;
}

function resolveHubMediaUrl(value: string): string {
  if (/^https?:\/\//i.test(value)) return value;
  const trimmed = value.startsWith("/") ? value.slice(1) : value;
  return `${API_BASE_URL.replace(/\/$/, "")}/${trimmed}`;
}

export async function fetchAdminOverview(days: number): Promise<AdminOverviewResponse> {
  const response = await fetch(`${API_BASE_URL}/admin/overview?days=${encodeURIComponent(String(days))}`, {
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not load admin overview.");
  }
  const payload = (await response.json()) as Partial<AdminOverviewResponse> | null;
  return {
    totalRuns: payload?.totalRuns ?? 0,
    totalPlayers: payload?.totalPlayers ?? 0,
    totalScenarios: payload?.totalScenarios ?? 0,
    unknownTypeRuns: payload?.unknownTypeRuns ?? 0,
    missingSummaryRuns: payload?.missingSummaryRuns ?? 0,
    missingFeatureRuns: payload?.missingFeatureRuns ?? 0,
    missingTimelineRuns: payload?.missingTimelineRuns ?? 0,
    missingContextRuns: payload?.missingContextRuns ?? 0,
    zeroScoreRuns: payload?.zeroScoreRuns ?? 0,
    missingSourceSessionRuns: payload?.missingSourceSessionRuns ?? 0,
    appVersions: Array.isArray(payload?.appVersions) ? payload.appVersions : [],
    schemaVersions: Array.isArray(payload?.schemaVersions) ? payload.schemaVersions : [],
    topUnknownScenarios: Array.isArray(payload?.topUnknownScenarios) ? payload.topUnknownScenarios : [],
    recentIngests: Array.isArray(payload?.recentIngests) ? payload.recentIngests : [],
    userSyncHealth: Array.isArray(payload?.userSyncHealth) ? payload.userSyncHealth : [],
    recentFailures: Array.isArray(payload?.recentFailures) ? payload.recentFailures : [],
  };
}

export async function runAdminReclassify(): Promise<{ ok: boolean; updated: number }> {
  const response = await fetch(`${API_BASE_URL}/admin/actions/reclassify`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not run scenario reclassify.");
  }
  return response.json() as Promise<{ ok: boolean; updated: number }>;
}

export async function runAdminRepairMetrics(): Promise<{ ok: boolean; updated: number }> {
  const response = await fetch(`${API_BASE_URL}/admin/actions/repair-metrics`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not repair run metrics.");
  }
  return response.json() as Promise<{ ok: boolean; updated: number }>;
}

export async function clearAdminFailures(): Promise<{ ok: boolean; cleared: number }> {
  const response = await fetch(`${API_BASE_URL}/admin/actions/clear-failures`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not clear ingest failures.");
  }
  return response.json() as Promise<{ ok: boolean; cleared: number }>;
}

export async function fetchAdminUserDetail(handle: string, days: number): Promise<AdminUserDetailResponse> {
  const response = await fetch(`${API_BASE_URL}/admin/user?handle=${encodeURIComponent(handle)}&days=${encodeURIComponent(String(days))}`, {
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not load user admin detail.");
  }
  const payload = (await response.json()) as Partial<AdminUserDetailResponse> | null;
  return {
    userHandle: payload?.userHandle ?? handle,
    userDisplayName: payload?.userDisplayName ?? handle,
    aimmodUserId: payload?.aimmodUserId ?? "",
    legacyExternalId: payload?.legacyExternalId ?? "",
    profileHandle: payload?.profileHandle ?? "",
    avatarUrl: payload?.avatarUrl ?? "",
    isVerified: Boolean(payload?.isVerified),
    runCount: payload?.runCount ?? 0,
    scenarioCount: payload?.scenarioCount ?? 0,
    unknownTypeRuns: payload?.unknownTypeRuns ?? 0,
    missingTimelineRuns: payload?.missingTimelineRuns ?? 0,
    missingContextRuns: payload?.missingContextRuns ?? 0,
    zeroScoreRuns: payload?.zeroScoreRuns ?? 0,
    lastPlayedAt: payload?.lastPlayedAt ?? "",
    lastIngestedAt: payload?.lastIngestedAt ?? "",
    lastAppVersion: payload?.lastAppVersion ?? "",
    lastSchemaVersion: payload?.lastSchemaVersion ?? 0,
    appVersions: Array.isArray(payload?.appVersions) ? payload.appVersions : [],
    schemaVersions: Array.isArray(payload?.schemaVersions) ? payload.schemaVersions : [],
    linkedAccounts: Array.isArray(payload?.linkedAccounts)
      ? payload.linkedAccounts.map((account) => ({
          provider: account?.provider ?? "",
          providerAccountId: account?.providerAccountId ?? "",
          username: account?.username ?? "",
          displayName: account?.displayName ?? "",
          avatarUrl: account?.avatarUrl ?? "",
          verified: Boolean(account?.verified),
          createdAt: account?.createdAt ?? "",
          updatedAt: account?.updatedAt ?? "",
        }))
      : [],
    topUnknownScenarios: Array.isArray(payload?.topUnknownScenarios) ? payload.topUnknownScenarios : [],
    recentFailures: Array.isArray(payload?.recentFailures) ? payload.recentFailures : [],
    recentRuns: Array.isArray(payload?.recentRuns) ? payload.recentRuns : [],
  };
}

// ---- Player search (fuzzy, across AimMod + KovaaK's) ----

export type PlayerSearchResult = {
  /** "aimmod" = has an AimMod Hub profile; "kovaaks" = KovaaK's-only player */
  type: "aimmod" | "kovaaks";
  steamId: string;
  /** AimMod profile handle — only set for type "aimmod" */
  handle: string;
  displayName: string;
  username: string;
  avatarUrl: string;
  country: string;
  runCount: number;
};

export async function searchPlayers(query: string): Promise<PlayerSearchResult[]> {
  const response = await fetch(
    `${API_BASE_URL}/api/lookup/search?q=${encodeURIComponent(query.trim())}`,
  );
  if (!response.ok) return [];
  const data = (await response.json()) as { players?: PlayerSearchResult[] };
  return data.players ?? [];
}

// ---- External (Steam ID) lookup ----

export type ExternalBenchmarkSummary = {
  benchmarkId: number;
  benchmarkName: string;
  benchmarkIconUrl: string;
  benchmarkAuthor: string;
  benchmarkType: string;
  overallRankName: string;
  overallRankIcon: string;
  overallRankColor: string;
};

export type ExternalProfileResponse = {
  /** Canonical Steam64 ID — may be empty for KovaaK's-username-only lookups. */
  resolvedSteamId: string;
  kovaaksUsername: string;
  isAimmodUser: boolean;
  aimmodHandle: string;
  aimmodDisplayName: string;
  benchmarks: ExternalBenchmarkSummary[];
};

export type ExternalRankVisual = {
  rankIndex: number;
  rankName: string;
  iconUrl: string;
  color: string;
  frameUrl: string;
};

export type ExternalThreshold = {
  rankIndex: number;
  rankName: string;
  iconUrl: string;
  color: string;
  score: number;
};

export type ExternalScenarioPage = {
  scenarioName: string;
  score: number;
  leaderboardRank: number;
  leaderboardId: number;
  rankIndex: number;
  rankName: string;
  rankIconUrl: string;
  rankColor: string;
  thresholds: ExternalThreshold[];
};

export type ExternalCategoryPage = {
  categoryName: string;
  categoryRank: number;
  scenarios: ExternalScenarioPage[];
};

export type ExternalBenchmarkPageResponse = {
  steamId: string;
  kovaaksUsername: string;
  isAimmodUser: boolean;
  aimmodHandle: string;
  benchmarkId: number;
  benchmarkName: string;
  benchmarkIconUrl: string;
  overallRankIndex: number;
  overallRankName: string;
  overallRankIcon: string;
  overallRankColor: string;
  ranks: ExternalRankVisual[];
  categories: ExternalCategoryPage[];
};

export async function lookupExternalUser(query: string): Promise<ExternalProfileResponse> {
  const response = await fetch(
    `${API_BASE_URL}/api/lookup?q=${encodeURIComponent(query.trim())}`,
  );
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `Lookup failed (${response.status})`);
  }
  return response.json() as Promise<ExternalProfileResponse>;
}

export async function fetchExternalBenchmarkPage(
  steamId: string,
  benchmarkId: number,
): Promise<ExternalBenchmarkPageResponse> {
  const response = await fetch(
    `${API_BASE_URL}/api/lookup/benchmark?q=${encodeURIComponent(steamId.trim())}&benchmarkId=${benchmarkId}`,
  );
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `Benchmark lookup failed (${response.status})`);
  }
  return response.json() as Promise<ExternalBenchmarkPageResponse>;
}

export async function runAdminReclassifyUser(handle: string): Promise<{ ok: boolean; updated: number }> {
  const response = await fetch(`${API_BASE_URL}/admin/actions/reclassify-user?handle=${encodeURIComponent(handle)}`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not repair this player's scenario types.");
  }
  return response.json() as Promise<{ ok: boolean; updated: number }>;
}

export async function runAdminRepairUserMetrics(handle: string): Promise<{ ok: boolean; updated: number }> {
  const response = await fetch(`${API_BASE_URL}/admin/actions/repair-user-metrics?handle=${encodeURIComponent(handle)}`, {
    method: "POST",
    credentials: "include",
  });
  if (!response.ok) {
    throw new Error(await response.text() || "Could not repair this player's run metrics.");
  }
  return response.json() as Promise<{ ok: boolean; updated: number }>;
}
