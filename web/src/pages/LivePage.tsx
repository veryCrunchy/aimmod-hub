import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link } from "react-router-dom";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { VerificationBadge } from "../components/VerificationBadge";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAnimatedNumber } from "../hooks/useAnimatedNumber";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useNow } from "../hooks/useNow";
import { fetchLiveActivityFeed, formatRelativeTime, subscribeLiveActivityFeed, type LiveHubActivity } from "../lib/api";

function PlayerAvatar({ url, name }: { url?: string; name: string }) {
  if (!url) {
    return (
      <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border border-white/10 bg-white/6 text-sm text-muted">
        {name.slice(0, 1).toUpperCase()}
      </span>
    );
  }
  return <img src={url} alt="" className="h-11 w-11 shrink-0 rounded-full border border-white/10 object-cover" />;
}

function activityStatusLabel(activity: LiveHubActivity): string {
  const base = activity.gameState?.trim() || "Practicing";
  if (activity.scenarioSubtype?.trim()) {
    return `${base} · ${activity.scenarioSubtype}`;
  }
  return base;
}

function resolveLiveTimerSeconds(activity: LiveHubActivity, nowMs: number): number | null {
  const updatedAtMs = activity.updatedAt ? new Date(activity.updatedAt).getTime() : NaN;
  const elapsedSinceUpdateSecs =
    Number.isFinite(updatedAtMs) && updatedAtMs > 0 ? Math.max(0, (nowMs - updatedAtMs) / 1000) : 0;
  const isPaused = activity.paused === true;

  if (activity.timeRemainingSecs != null && Number.isFinite(activity.timeRemainingSecs)) {
    return Math.max(0, activity.timeRemainingSecs - (isPaused ? 0 : elapsedSinceUpdateSecs));
  }
  if (activity.queueTimeRemainingSecs != null && Number.isFinite(activity.queueTimeRemainingSecs)) {
    return Math.max(0, activity.queueTimeRemainingSecs - (isPaused ? 0 : elapsedSinceUpdateSecs));
  }
  if (activity.elapsedSecs != null && Number.isFinite(activity.elapsedSecs)) {
    return Math.max(0, activity.elapsedSecs + (isPaused ? 0 : elapsedSinceUpdateSecs));
  }
  return null;
}

function AnimatedMetric({
  label,
  value,
  format,
}: {
  label: string;
  value: number | null | undefined;
  format: (value: number) => string;
}) {
  const animatedValue = useAnimatedNumber(value, 650);
  return (
    <div className="rounded-[14px] border border-white/8 bg-white/[0.03] px-3 py-2">
      <div className="text-[11px] uppercase tracking-[0.08em] text-muted">{label}</div>
      <div className="mt-1 text-[18px] tabular-nums transition-[opacity,transform] duration-300 ease-out">
        {animatedValue != null ? format(animatedValue) : "—"}
      </div>
    </div>
  );
}

export function LivePage() {
  const [items, setItems] = useState<LiveHubActivity[]>([]);
  const [query, setQuery] = useState("");
  const [scenarioTypeFilter, setScenarioTypeFilter] = useState<string>("all");
  const [error, setError] = useState<string | null>(null);
  const refreshTimeoutRef = useRef<number | null>(null);
  const nowMs = useNow(1000);

  const load = useCallback(() => {
    void fetchLiveActivityFeed(250)
      .then((response) => {
        setItems(response.items.filter((item) => item.active && item.userHandle?.trim()));
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Could not load live activity.");
      });
  }, []);

  const scheduleRefresh = useCallback(() => {
    if (refreshTimeoutRef.current != null) return;
    refreshTimeoutRef.current = window.setTimeout(() => {
      refreshTimeoutRef.current = null;
      load();
    }, 750);
  }, [load]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    const unsubscribe = subscribeLiveActivityFeed({
      onUpdate: () => {
        scheduleRefresh();
      },
    });
    return () => {
      unsubscribe();
      if (refreshTimeoutRef.current != null) {
        window.clearTimeout(refreshTimeoutRef.current);
        refreshTimeoutRef.current = null;
      }
    };
  }, [scheduleRefresh]);

  useAutoRefresh(load, 30_000);

  const scenarioTypes = useMemo(() => {
    const values = new Set<string>();
    for (const item of items) {
      if (item.scenarioType?.trim() && item.scenarioType !== "Unknown") {
        values.add(item.scenarioType);
      }
    }
    return [...values].sort((a, b) => a.localeCompare(b));
  }, [items]);

  const filteredItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return items.filter((item) => {
      if (scenarioTypeFilter !== "all" && item.scenarioType !== scenarioTypeFilter) return false;
      if (!normalizedQuery) return true;
      const haystack = [
        item.userHandle,
        item.userDisplayName,
        item.scenarioName,
        item.scenarioType,
        item.scenarioSubtype,
        item.gameState,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase();
      return haystack.includes(normalizedQuery);
    });
  }, [items, query, scenarioTypeFilter]);

  const inScenarioCount = useMemo(
    () => items.filter((item) => Boolean(item.scenarioName?.trim())).length,
    [items],
  );

  const bridgeHealthyCount = useMemo(
    () => items.filter((item) => item.runtimeLoaded !== false && item.bridgeConnected !== false).length,
    [items],
  );

  return (
    <PageStack>
      <Helmet>
        <title>Live · AimMod Hub</title>
        <meta name="description" content="See which AimMod users are currently practicing live, including scenario, score, and bridge status." />
      </Helmet>

      <PageSection className="relative overflow-hidden border-cyan/18 bg-[radial-gradient(circle_at_top_left,rgba(57,208,255,0.18),transparent_24%),radial-gradient(circle_at_78%_18%,rgba(121,201,151,0.12),transparent_20%),linear-gradient(135deg,rgba(8,18,15,0.98),rgba(5,12,10,0.96)_54%,rgba(3,8,6,0.98))] shadow-[0_24px_80px_rgba(0,0,0,0.42)]">
        <div className="absolute inset-y-0 right-[8%] w-[28%] rounded-full bg-[radial-gradient(circle,rgba(57,208,255,0.14),transparent_68%)] blur-3xl" />
        <div className="relative text-[11px] uppercase tracking-[0.1em] text-cyan">Live Activity</div>
        <h1 className="my-2.5 max-w-[15ch] break-words text-[clamp(28px,5vw,58px)] leading-[0.94] tracking-[-0.05em]">
          Who is grinding right now.
        </h1>
        <p className="max-w-[760px] text-[14px] leading-6 text-[#cbe4d7] md:text-[16px] md:leading-7">
          This page updates automatically and shows active AimMod users, what they are playing, and how their session is progressing.
        </p>
        <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:items-center">
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search player, scenario, or state..."
            className="min-w-0 flex-1 rounded-xl border border-white/10 bg-white/6 px-3.5 py-2.5 text-[13px] text-text placeholder:text-muted/60 outline-none transition-colors focus:border-cyan/40 focus:bg-white/[0.08]"
          />
          <select
            value={scenarioTypeFilter}
            onChange={(event) => setScenarioTypeFilter(event.target.value)}
            className="rounded-xl border border-white/10 bg-white/6 px-3.5 py-2.5 text-[13px] text-text outline-none transition-colors focus:border-cyan/40"
          >
            <option value="all">All types</option>
            {scenarioTypes.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(180px,1fr))]">
        <StatCard label="Live now" value={items.length.toLocaleString()} detail="Users currently sending live activity" />
        <StatCard label="In scenario" value={inScenarioCount.toLocaleString()} detail="Users with a scenario currently identified" accent="cyan" />
        <StatCard label="Bridge healthy" value={bridgeHealthyCount.toLocaleString()} detail="Users with runtime and bridge both reporting healthy" accent="gold" />
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Active sessions"
          title={`${filteredItems.length.toLocaleString()} live ${filteredItems.length === 1 ? "player" : "players"}`}
          body="Sorted by the most recently updated session first."
        />

        {filteredItems.length > 0 ? (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {filteredItems.map((item) => {
              const name = item.userDisplayName?.trim() || item.userHandle?.trim() || "Player";
              const scenarioTitle = item.scenarioName?.trim() || item.gameState?.trim() || "Practicing";
              return (
                <Link
                  key={`${item.userHandle}:${item.updatedAt ?? "now"}`}
                  to={`/profiles/${item.userHandle}`}
                  className="group rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                >
                  <div className="flex items-start gap-3">
                    <PlayerAvatar url={item.avatarUrl} name={name} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <strong className="truncate text-text">{name}</strong>
                        <VerificationBadge verified={Boolean(item.isVerified)} />
                      </div>
                      <div className="mt-1 text-sm text-muted">@{item.userHandle}</div>
                    </div>
                    <div className="shrink-0 text-right text-[11px] text-muted">
                      {item.updatedAt ? formatRelativeTime(item.updatedAt) : "live"}
                    </div>
                  </div>

                  <div className="mt-4 flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="text-[11px] uppercase tracking-[0.08em] text-mint">Current activity</div>
                      <div className="mt-1 break-words text-[20px] leading-tight tracking-[-0.03em] text-text">
                        {scenarioTitle}
                      </div>
                    </div>
                    {item.scenarioType ? <ScenarioTypeBadge type={item.scenarioType} /> : null}
                  </div>

                  <div className="mt-2 text-sm leading-6 text-muted">
                    {activityStatusLabel(item)}
                  </div>

                  <div className="mt-4 grid grid-cols-2 gap-2 text-sm text-text">
                    <AnimatedMetric
                      label="Score"
                      value={item.score}
                      format={(value) => Math.round(value).toLocaleString()}
                    />
                    <AnimatedMetric
                      label="Accuracy"
                      value={item.accuracyPct}
                      format={(value) => `${value.toFixed(1)}%`}
                    />
                    <AnimatedMetric
                      label="Kills"
                      value={item.kills ?? null}
                      format={(value) => Math.round(value).toLocaleString()}
                    />
                    <AnimatedMetric
                      label="Timer"
                      value={resolveLiveTimerSeconds(item, nowMs)}
                      format={(value) => {
                        const whole = Math.max(0, Math.round(value));
                        const minutes = Math.floor(whole / 60);
                        const seconds = whole % 60;
                        return `${minutes}:${seconds.toString().padStart(2, "0")}`;
                      }}
                    />
                  </div>

                  {item.runtimeLoaded === false || item.bridgeConnected === false ? (
                    <div className="mt-3 text-[12px] leading-6 text-muted">
                      Bridge status: {item.runtimeLoaded ? "runtime loaded" : "runtime not loaded"}
                      {" · "}
                      {item.bridgeConnected ? "bridge connected" : "bridge reconnecting"}
                    </div>
                  ) : null}

                  <div className="mt-4 text-[12px] text-cyan transition-colors group-hover:text-text">
                    Open profile →
                  </div>
                </Link>
              );
            })}
          </div>
        ) : (
          <EmptyState
            title="No live users right now"
            body={error || "Active AimMod sessions will show up here as soon as players start sending live activity."}
          />
        )}
      </PageSection>
    </PageStack>
  );
}
