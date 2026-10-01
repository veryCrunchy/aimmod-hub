import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useSearchParams } from "react-router-dom";
import { updateFilterQuery } from "../lib/savedPageFilters";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { useAnimatedNumber } from "../hooks/useAnimatedNumber";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useNow } from "../hooks/useNow";
import { useAuth } from "../lib/AuthContext";
import { displayScenarioType, fetchLiveActivityFeed, formatRelativeTime, slugifyScenarioName, subscribeLiveActivityFeed, type LiveHubActivity } from "../lib/api";
import { liveView, ownSessionNote, sortLive, type LiveView } from "../lib/liveActivity";

function PlayerAvatar({ url, name }: { url?: string; name: string }) {
  if (!url) {
    return (
      <span aria-hidden="true" className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-line bg-bg-2 text-sm text-muted">
        {name.slice(0, 1).toUpperCase()}
      </span>
    );
  }
  return <img src={url} alt="" className="h-10 w-10 shrink-0 rounded-full border border-line object-cover" />;
}

function LiveNumber({ label, value, format }: { label: string; value: number | null; format: (value: number) => string }) {
  const animated = useAnimatedNumber(value, 650);
  return (
    <div className="min-w-0">
      <dt className="text-xs text-muted-2">{label}</dt>
      <dd className="mt-0.5 text-base font-medium tabular-nums text-text">{animated != null ? format(animated) : "—"}</dd>
    </div>
  );
}

const phaseDot = { playing: "bg-mint", paused: "bg-gold", idle: "bg-muted-2" } as const;

function LiveCard({ activity, view, own }: { activity: LiveHubActivity; view: LiveView; own: boolean }) {
  const note = own ? ownSessionNote(activity) : null;
  return (
    <li className="grid gap-3 rounded-md border border-line bg-panel p-4">
      <div className="flex items-center gap-3">
        <PlayerAvatar url={activity.avatarUrl} name={view.name} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <Link to={`/profiles/${view.handle}`} className="truncate font-semibold text-text hover:text-cyan">{view.name}</Link>
            <VerificationBadge verified={Boolean(activity.isVerified)} />
          </div>
          <div className="mt-0.5 flex items-center gap-1.5 text-xs text-muted">
            <span aria-hidden="true" className={`h-2 w-2 rounded-full ${phaseDot[view.phase]}`} />
            {view.status}
            {view.session ? <span className="text-muted-2">· {view.session}</span> : null}
            {activity.updatedAt ? <span className="text-muted-2">· updated {formatRelativeTime(activity.updatedAt)}</span> : null}
          </div>
        </div>
        {view.timer ? <span className="shrink-0 text-sm tabular-nums text-muted">{view.timer}</span> : null}
      </div>

      {view.phase === "idle" ? null : (
        <>
          <div className="flex min-w-0 items-center justify-between gap-3">
            <Link to={`/scenarios/${slugifyScenarioName(view.title)}`} className="min-w-0 truncate text-base text-text hover:text-cyan">{view.title}</Link>
            {view.scenarioType ? <ScenarioTypeBadge type={view.scenarioType} /> : null}
          </div>
          <dl className="grid grid-cols-3 gap-3 border-t border-line pt-3">
            <LiveNumber label="Score" value={view.score} format={(v) => Math.round(v).toLocaleString()} />
            <LiveNumber label="Accuracy" value={view.accuracy} format={(v) => `${v.toFixed(1)}%`} />
            {view.scorePerMinute != null
              ? <LiveNumber label="Score/min" value={view.scorePerMinute} format={(v) => Math.round(v).toLocaleString()} />
              : <LiveNumber label="Kills" value={view.kills} format={(v) => Math.round(v).toLocaleString()} />}
          </dl>
        </>
      )}
      {note ? <p className="text-xs text-gold">{note}</p> : null}
    </li>
  );
}

export function LivePage() {
  const auth = useAuth();
  const [items, setItems] = useState<LiveHubActivity[] | null>(null);
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const setQuery = (q: string) => setParams(current => updateFilterQuery(current, { q }), { replace: true });
  const setScenarioTypeFilter = (scenarioType: string) => setParams(current => updateFilterQuery(current, { scenarioType: scenarioType === "all" ? null : scenarioType }), { replace: true });
  const [error, setError] = useState<string | null>(null);
  const refreshTimeoutRef = useRef<number | null>(null);
  const nowMs = useNow(1000);
  const ownHandle = auth.user?.profileHandle?.toLowerCase() ?? "";

  const load = useCallback(() => {
    void fetchLiveActivityFeed(250)
      .then((response) => {
        setItems(response.items.filter((item) => item.active && item.userHandle?.trim()));
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Could not load live players.");
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
    const unsubscribe = subscribeLiveActivityFeed({ onUpdate: scheduleRefresh });
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
    for (const item of items ?? []) {
      if (item.scenarioType?.trim() && item.scenarioType !== "Unknown") values.add(item.scenarioType);
    }
    return [...values].sort((a, b) => a.localeCompare(b));
  }, [items]);

  const scenarioTypeFilter = scenarioTypes.includes(params.get("scenarioType") ?? "") ? params.get("scenarioType")! : "all";

  const filteredItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return sortLive(items ?? []).filter((item) => {
      if (scenarioTypeFilter !== "all" && item.scenarioType !== scenarioTypeFilter) return false;
      if (!normalizedQuery) return true;
      return [item.userHandle, item.userDisplayName, item.scenarioName, item.scenarioType]
        .filter(Boolean).join(" ").toLowerCase().includes(normalizedQuery);
    });
  }, [items, query, scenarioTypeFilter]);

  const playingCount = (items ?? []).filter((item) => Boolean(item.scenarioName?.trim())).length;
  const total = items?.length ?? 0;
  const filtering = Boolean(query.trim()) || scenarioTypeFilter !== "all";

  return (
    <PageStack>
      <Helmet>
        <title>Live now · AimMod Hub</title>
        <meta name="description" content="KovaaK's players in a scenario right now, with live score and accuracy." />
      </Helmet>

      <PageHeader
        title="Live now"
        meta={items == null ? "Checking who is playing…" : total === 0 ? "Nobody is playing right now" : `${total.toLocaleString()} ${total === 1 ? "player" : "players"} online · ${playingCount.toLocaleString()} in a scenario`}
      />

      {total > 1 && (
        <div className="flex flex-col gap-2 sm:flex-row">
          <label className="sr-only" htmlFor="live-search">Search live players</label>
          <input
            id="live-search"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Player or scenario"
            className="min-h-10 min-w-0 flex-1 rounded-md border border-line bg-panel px-3 text-sm text-text placeholder:text-muted-2 focus:border-line-strong"
          />
          <label className="sr-only" htmlFor="live-type">Scenario type</label>
          <select
            id="live-type"
            value={scenarioTypeFilter}
            onChange={(event) => setScenarioTypeFilter(event.target.value)}
            className="min-h-10 rounded-md border border-line bg-panel px-3 text-sm text-text"
          >
            <option value="all">All types</option>
            {scenarioTypes.map((type) => <option key={type} value={type}>{displayScenarioType(type) ?? type}</option>)}
          </select>
        </div>
      )}

      {items == null && !error ? (
        <div role="status" aria-label="Loading live players" className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {[0, 1, 2].map((i) => <Skeleton key={i} className="h-44" />)}
        </div>
      ) : error && total === 0 ? (
        <EmptyState title="Live players could not be loaded.">
          <Button onClick={load}>Try again</Button>
        </EmptyState>
      ) : filteredItems.length > 0 ? (
        <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filteredItems.map((item) => (
            <LiveCard key={item.userHandle} activity={item} view={liveView(item, nowMs)} own={Boolean(ownHandle) && item.userHandle?.toLowerCase() === ownHandle} />
          ))}
        </ul>
      ) : filtering ? (
        <EmptyState title="No live players match.">
          <Button onClick={() => setParams(current => updateFilterQuery(current, { q: null, scenarioType: null }), { replace: true })}>Clear filters</Button>
        </EmptyState>
      ) : (
        <EmptyState title="Nobody is playing right now.">
          <Button to="/kovaaks">See recent runs</Button>
        </EmptyState>
      )}
    </PageStack>
  );
}
