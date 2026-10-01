import type { LiveHubActivity } from "./api";

// Live activity comes from two clients with different health checks:
// - companion app: UE4SS runtime loaded / bridge DLL connected
// - in-game mod: AimModCore loaded in KovaaK's / live game data reaching the AimMod service
// Older Hub responses and companion payloads have no `client`; treat them as the companion.

export function liveClient(activity: LiveHubActivity): "companion" | "in-game" {
  return activity.client === "in-game" ? "in-game" : "companion";
}

export function isLiveHealthy(activity: LiveHubActivity): boolean {
  if (typeof activity.healthy === "boolean") return activity.healthy;
  return activity.runtimeLoaded !== false && activity.bridgeConnected !== false;
}

/** One status line for an unhealthy connection, or null when both checks pass. */
export function liveHealthDetail(activity: LiveHubActivity): string | null {
  if (isLiveHealthy(activity)) return null;
  if (liveClient(activity) === "in-game") {
    if (activity.runtimeLoaded === false) return "AimMod is starting in KovaaK's";
    return "AimMod is reconnecting to KovaaK's";
  }
  return `Bridge status: ${activity.runtimeLoaded ? "runtime loaded" : "runtime not loaded"} · ${
    activity.bridgeConnected ? "bridge connected" : "bridge reconnecting"
  }`;
}

/** Session progress, e.g. "3 runs this session", or null when the client does not send it. */
export function liveSessionDetail(activity: LiveHubActivity): string | null {
  const runs = activity.sessionRunCount;
  if (typeof runs !== "number" || !Number.isFinite(runs) || runs < 0) return null;
  return runs === 1 ? "1 run this session" : `${Math.round(runs).toLocaleString()} runs this session`;
}
