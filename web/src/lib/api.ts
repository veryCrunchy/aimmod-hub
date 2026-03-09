import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  GetOverviewRequest,
  GetProfileRequest,
  GetRunRequest,
  GetScenarioPageRequest,
  SessionSummaryValue,
} from "../gen/aimmod/hub/v1/hub_pb";
import { HubService } from "../gen/aimmod/hub/v1/hub_connect";

const transport = createConnectTransport({
  baseUrl: import.meta.env.VITE_API_BASE_URL || "http://localhost:8080"
});

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "http://localhost:8080";

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

export function displayScenarioType(value?: string | null) {
  const normalized = value?.trim();
  if (!normalized || normalized === "Unknown") {
    return null;
  }
  return normalized;
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
  scenarioName: string;
  scenarioType: string;
  playedAt: string;
  score: number;
  accuracy: number;
  durationMS: number;
  userHandle: string;
  userDisplayName: string;
};

export type HubSearchResponse = {
  query: string;
  scenarios: HubSearchScenario[];
  profiles: HubSearchProfile[];
  runs: HubSearchRun[];
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
  };
}
