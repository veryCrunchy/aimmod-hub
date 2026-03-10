import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { API_BASE_URL } from "./config";
import {
  GetOverviewRequest,
  GetProfileRequest,
  GetRunRequest,
  GetScenarioPageRequest,
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

export type HubSearchResponse = {
  query: string;
  scenarios: HubSearchScenario[];
  profiles: HubSearchProfile[];
  runs: HubSearchRun[];
  replays: HubSearchRun[];
};

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
  runCount: number;
  scenarioCount: number;
  unknownTypeRuns: number;
  missingTimelineRuns: number;
  missingContextRuns: number;
  zeroScoreRuns: number;
  lastPlayedAt: string;
  lastIngestedAt: string;
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
  const response = await fetch(`${API_BASE_URL}/search?q=${encodeURIComponent(query)}`);
  if (!response.ok) {
    throw new Error("Could not search the hub.");
  }
  const payload = (await response.json()) as Partial<HubSearchResponse> | null;
  return {
    query: payload?.query ?? query,
    scenarios: Array.isArray(payload?.scenarios) ? payload.scenarios : [],
    profiles: Array.isArray(payload?.profiles) ? payload.profiles : [],
    runs: Array.isArray(payload?.runs) ? payload.runs : [],
    replays: Array.isArray(payload?.replays) ? payload.replays : [],
  };
}

export async function fetchReplayHub(params?: {
  query?: string;
  scenarioName?: string;
  handle?: string;
  limit?: number;
}): Promise<ReplayListResponse> {
  const search = new URLSearchParams();
  if (params?.query?.trim()) search.set("q", params.query.trim());
  if (params?.scenarioName?.trim()) search.set("scenarioName", params.scenarioName.trim());
  if (params?.handle?.trim()) search.set("handle", params.handle.trim());
  if (params?.limit) search.set("limit", String(params.limit));
  const response = await fetch(`${API_BASE_URL}/replays?${search.toString()}`);
  if (!response.ok) {
    throw new Error("Could not load replays.");
  }
  const payload = (await response.json()) as Partial<ReplayListResponse> | null;
  return {
    query: payload?.query ?? params?.query ?? "",
    scenarioName: payload?.scenarioName ?? params?.scenarioName ?? "",
    userHandle: payload?.userHandle ?? params?.handle ?? "",
    items: Array.isArray(payload?.items) ? payload.items : [],
  };
}

export async function fetchReplayMediaMeta(runId: string): Promise<ReplayMediaMeta> {
  const response = await fetch(`${API_BASE_URL}/media/replays/meta?runId=${encodeURIComponent(runId)}`);
  if (!response.ok) {
    throw new Error("Could not load replay media.");
  }
  const payload = (await response.json()) as Partial<ReplayMediaMeta> | null;
  return {
    available: Boolean(payload?.available),
    runId: payload?.runId,
    quality: payload?.quality,
    contentType: payload?.contentType,
    byteSize: payload?.byteSize,
    mediaUrl: payload?.mediaUrl,
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
  const response = await fetch(`${API_BASE_URL}/replays/mouse-path?runId=${encodeURIComponent(runId)}`);
  if (!response.ok) {
    throw new Error("Could not load mouse path.");
  }
  const payload = await response.json();
  return {
    available: Boolean(payload?.available),
    points: Array.isArray(payload?.points) ? payload.points : [],
  };
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
    runCount: payload?.runCount ?? 0,
    scenarioCount: payload?.scenarioCount ?? 0,
    unknownTypeRuns: payload?.unknownTypeRuns ?? 0,
    missingTimelineRuns: payload?.missingTimelineRuns ?? 0,
    missingContextRuns: payload?.missingContextRuns ?? 0,
    zeroScoreRuns: payload?.zeroScoreRuns ?? 0,
    lastPlayedAt: payload?.lastPlayedAt ?? "",
    lastIngestedAt: payload?.lastIngestedAt ?? "",
    topUnknownScenarios: Array.isArray(payload?.topUnknownScenarios) ? payload.topUnknownScenarios : [],
    recentFailures: Array.isArray(payload?.recentFailures) ? payload.recentFailures : [],
    recentRuns: Array.isArray(payload?.recentRuns) ? payload.recentRuns : [],
  };
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
