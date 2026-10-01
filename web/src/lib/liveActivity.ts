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
    title: activity.scenarioName?.trim() || activity.gameState?.trim() || "In game",
    scenarioType: activity.scenarioType && activity.scenarioType !== "Unknown" ? activity.scenarioType : undefined,
    timer: phase === "idle" ? null : liveTimer(activity, nowMs),
    score: phase === "idle" ? null : finite(activity.score),
    accuracy: phase === "idle" ? null : finite(activity.accuracyPct),
    scorePerMinute: phase === "idle" ? null : finite(activity.scorePerMinute),
    kills: phase === "idle" ? null : finite(activity.kills),
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

/**
 * A short note for the signed-in player about their own session when the game
 * is not reporting everything. Other players never see this.
 */
export function ownSessionNote(activity: LiveHubActivity): string | null {
  // Only a loaded runtime with a dropped connection is a real problem. Senders
  // that do not report these fields leave both false, which says nothing.
  if (activity.runtimeLoaded === true && activity.bridgeConnected === false) return "Reconnecting to your game. Live scores will resume shortly.";
  return null;
}

export function phaseLabel(phase: LivePhase): string {
  return phase === "playing" ? "Playing" : phase === "paused" ? "Paused" : "In menus";
}
