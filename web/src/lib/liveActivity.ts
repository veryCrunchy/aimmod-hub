import type { LiveHubActivity } from "./api";

export type LivePhase = "playing" | "paused" | "idle";

export type LiveView = {
  name: string;
  handle: string;
  phase: LivePhase;
  /** Scenario name, or the game state when no scenario is loaded. */
  title: string;
  scenarioType?: string;
  /** "0:34 left", "1:20 in" or null when the game reports no timer. */
  timer: string | null;
  score: number | null;
  accuracy: number | null;
  scorePerMinute: number | null;
  kills: number | null;
  /** "Playing", "Paused", or what the player is doing outside a scenario. */
  status: string;
  /** "3 runs this session", or null when the client does not send it. */
  session: string | null;
};

function finite(value: number | null | undefined): number | null {
  return value != null && Number.isFinite(value) ? value : null;
}

function clock(seconds: number): string {
  const whole = Math.max(0, Math.round(seconds));
  return `${Math.floor(whole / 60)}:${(whole % 60).toString().padStart(2, "0")}`;
}

export function livePhase(activity: LiveHubActivity): LivePhase {
  if (!activity.scenarioName?.trim()) return "idle";
  return activity.paused ? "paused" : "playing";
}

/** Seconds since the last update, used to keep timers moving between updates. */
function sinceUpdate(activity: LiveHubActivity, nowMs: number): number {
  const updated = activity.updatedAt ? new Date(activity.updatedAt).getTime() : NaN;
  return Number.isFinite(updated) && updated > 0 ? Math.max(0, (nowMs - updated) / 1000) : 0;
}

export function liveTimer(activity: LiveHubActivity, nowMs: number): string | null {
  const drift = activity.paused ? 0 : sinceUpdate(activity, nowMs);
  const remaining = finite(activity.timeRemainingSecs) ?? finite(activity.queueTimeRemainingSecs);
  if (remaining != null) return `${clock(Math.max(0, remaining - drift))} left`;
  const elapsed = finite(activity.elapsedSecs);
  if (elapsed != null) return `${clock(elapsed + drift)} in`;
  return null;
}

export function liveView(activity: LiveHubActivity, nowMs: number): LiveView {
  const handle = activity.userHandle?.trim() ?? "";
  const phase = livePhase(activity);
  return {
    name: activity.userDisplayName?.trim() || handle || "Player",
    handle,
    phase,
    title: activity.scenarioName?.trim() || activity.gameState?.trim() || activityLabel(activity) || "In game",
    scenarioType: activity.scenarioType && activity.scenarioType !== "Unknown" ? activity.scenarioType : undefined,
    timer: phase === "idle" ? null : liveTimer(activity, nowMs),
    score: phase === "idle" ? null : finite(activity.score),
    accuracy: phase === "idle" ? null : finite(activity.accuracyPct),
    scorePerMinute: phase === "idle" ? null : finite(activity.scorePerMinute),
    kills: phase === "idle" ? null : finite(activity.kills),
    status: phase === "idle" ? activityLabel(activity) ?? phaseLabel(phase) : phaseLabel(phase),
    session: liveSessionDetail(activity),
  };
}

const phaseOrder: Record<LivePhase, number> = { playing: 0, paused: 1, idle: 2 };

/** Players in a scenario first, then paused, then in menus; newest update first within each. */
export function sortLive(items: readonly LiveHubActivity[]): LiveHubActivity[] {
  return [...items].sort((a, b) => {
    const phase = phaseOrder[livePhase(a)] - phaseOrder[livePhase(b)];
    if (phase !== 0) return phase;
    return (new Date(b.updatedAt ?? 0).getTime() || 0) - (new Date(a.updatedAt ?? 0).getTime() || 0);
  });
}

// Live activity comes from two clients with different health checks:
// - companion app: game runtime loaded / connection to the game
// - in-game mod: AimMod loaded in KovaaK's / live game data reaching AimMod
// Older Hub responses and companion payloads have no `client`; treat them as the companion.

export function liveClient(activity: LiveHubActivity): "companion" | "in-game" {
  return activity.client === "in-game" ? "in-game" : "companion";
}

export function isLiveHealthy(activity: LiveHubActivity): boolean {
  if (typeof activity.healthy === "boolean") return activity.healthy;
  return activity.runtimeLoaded !== false && activity.bridgeConnected !== false;
}

/** A short connection problem for this client, or null when nothing is wrong. */
export function liveHealthDetail(activity: LiveHubActivity): string | null {
  if (liveClient(activity) === "in-game") {
    if (isLiveHealthy(activity)) return null;
    if (activity.runtimeLoaded === false) return "AimMod is starting in KovaaK's";
    return "AimMod is reconnecting to KovaaK's";
  }
  // Only a loaded runtime with a dropped connection is a real problem. Companion
  // versions that do not report these fields leave both false, which says nothing.
  if (activity.runtimeLoaded === true && activity.bridgeConnected === false) return "Reconnecting to your game";
  return null;
}

/**
 * A short note for the signed-in player about their own session when the game
 * is not reporting everything. Other players never see this.
 */
export function ownSessionNote(activity: LiveHubActivity): string | null {
  const detail = liveHealthDetail(activity);
  if (!detail) return null;
  return activity.runtimeLoaded === false
    ? `${detail}. Live scores will appear shortly.`
    : `${detail}. Live scores will resume shortly.`;
}

/** Session progress, e.g. "3 runs this session", or null when the client does not send it. */
export function liveSessionDetail(activity: LiveHubActivity): string | null {
  const runs = activity.sessionRunCount;
  if (typeof runs !== "number" || !Number.isFinite(runs) || runs < 0) return null;
  return runs === 1 ? "1 run this session" : `${Math.round(runs).toLocaleString()} runs this session`;
}

const activityLabels: Record<string, string> = {
  menu: "In menus",
  scenario: "In a scenario",
  challenge: "In a challenge",
  replay: "Watching a replay",
  lobby: "In a lobby",
  match: "In a match",
  results: "Viewing results",
};

/** A label for the client's activity kind, or null when it sent none. */
export function activityLabel(activity: LiveHubActivity): string | null {
  return activityLabels[activity.activity?.trim().toLowerCase() ?? ""] ?? null;
}

export function phaseLabel(phase: LivePhase): string {
  return phase === "playing" ? "Playing" : phase === "paused" ? "Paused" : "In menus";
}
