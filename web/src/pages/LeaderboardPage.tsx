import { useCallback, useEffect, useMemo, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useSearchParams } from "react-router-dom";
import { filterChoice, updateFilterQuery } from "../lib/savedPageFilters";
import type { GetLeaderboardResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { runHref } from "../components/RunTable";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { PageSkeleton } from "../components/ui/Skeleton";
import { Tabs } from "../components/ui/Tabs";
import { TypeFilterBar } from "../components/ui/TypeFilterBar";
import { PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { displayScenarioType, fetchLeaderboard, formatRelativeTime, slugifyScenarioName } from "../lib/api";
import { formatAccuracy, formatScore, recordHolders } from "../lib/kovaaksStats";
import { cn } from "../lib/cn";

type Tab = "records" | "top";
const PAGE_SIZE = 100;

export function LeaderboardPage() {
  const [data, setData] = useState<GetLeaderboardResponse | null>(null);
  const [error, setError] = useState(false);
  const [pagination, setPagination] = useState({ key: "", page: 0 });
  const [params, setParams] = useSearchParams();
  const tab = filterChoice<Tab>(params.get("tab"), ["records", "top"], "records");
  const query = params.get("q") ?? "";
  const setTab = (tab: Tab) => setParams(current => updateFilterQuery(current, { tab }), { replace: true });
  const setQuery = (q: string) => setParams(current => updateFilterQuery(current, { q: q || null }), { replace: true });
  const setTypeFilter = (scenarioType: string | null) => setParams(current => updateFilterQuery(current, { scenarioType }), { replace: true });

  const load = useCallback(() => {
    void fetchLeaderboard("")
      .then((next) => { setData(next); setError(false); })
      .catch(() => setError(true));
  }, []);
  useEffect(() => { load(); }, [load]);
  useAutoRefresh(load, 60_000);

  const scenarioTypes = useMemo(() => {
    const types = new Set<string>();
    for (const r of [...(data?.records ?? []), ...(data?.topScores ?? [])]) {
      if (r.scenarioType?.trim() && r.scenarioType !== "Unknown") types.add(r.scenarioType);
    }
    return [...types].sort();
  }, [data]);
  const typeFilter = scenarioTypes.includes(params.get("scenarioType") ?? "") ? params.get("scenarioType") : null;

  const activeList = useMemo(() => {
    const list = tab === "records" ? data?.records ?? [] : data?.topScores ?? [];
    const q = query.trim().toLowerCase();
    return list.filter((r) => (!typeFilter || r.scenarioType === typeFilter)
      && (!q || `${r.scenarioName} ${r.userDisplayName} ${r.userHandle}`.toLowerCase().includes(q)));
  }, [data, tab, typeFilter, query]);

  const head = <Helmet>
    <title>Leaderboards · AimMod Hub</title>
    <meta name="description" content="KovaaK's scenario records and the highest scores shared through AimMod." />
  </Helmet>;

  if (!data) {
    return <PageStack>{head}{error
      ? <><PageHeader title="Leaderboards" /><EmptyState title="Leaderboards could not be loaded."><Button onClick={load}>Try again</Button></EmptyState></>
      : <PageSkeleton stats={3} label="Loading leaderboards" />}</PageStack>;
  }

  const paginationKey = `${tab}:${typeFilter ?? ""}:${query}`;
  const pageCount = Math.max(1, Math.ceil(activeList.length / PAGE_SIZE));
  const page = Math.min(pagination.key === paginationKey ? pagination.page : 0, pageCount - 1);
  const visibleRows = activeList.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);
  const holders = recordHolders(data.records);
  const showRank = tab === "top";

  return (
    <PageStack>
      {head}
      <PageHeader title="Leaderboards" meta="Best scores shared through AimMod. Each scenario has one record." />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
        <StatCard label="Scenario records" value={data.records.length.toLocaleString()} />
        <StatCard label="Record holders" value={holders.length.toLocaleString()} />
        {holders[0] ? <StatCard label="Most records" value={<Link to={`/profiles/${holders[0].handle}`} className="hover:text-cyan">{holders[0].name}</Link>} detail={`${holders[0].records} of ${data.records.length}`} /> : null}
      </div>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_260px]">
        <div className="min-w-0">
          <Tabs label="Leaderboard" value={tab} onChange={setTab} tabs={[["records", "Scenario records"], ["top", "Top scores"]]} className="mb-3" />
          <div className="mb-3 flex flex-wrap items-center gap-3">
            <label className="sr-only" htmlFor="leaderboard-search">Filter by scenario or player</label>
            <input id="leaderboard-search" type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Scenario or player"
              className="min-h-9 w-full max-w-xs rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted-2" />
            {scenarioTypes.length > 1 && <TypeFilterBar types={scenarioTypes} active={typeFilter} onChange={setTypeFilter} className="mb-0" />}
          </div>

          {activeList.length > 0 ? (
            <div className="overflow-x-auto rounded-md border border-line">
              <table className="w-full min-w-[560px] text-left text-sm">
                <thead className="border-b border-line text-xs text-muted">
                  <tr>
                    {showRank && <th scope="col" className="w-10 px-3 py-2 font-medium">#</th>}
                    <th scope="col" className="px-3 py-2 font-medium">Scenario</th>
                    <th scope="col" className="px-3 py-2 font-medium">{tab === "records" ? "Record holder" : "Player"}</th>
                    <th scope="col" className="px-3 py-2 text-right font-medium">Score</th>
                    <th scope="col" className="px-3 py-2 text-right font-medium">Accuracy</th>
                    <th scope="col" className="px-3 py-2 text-right font-medium">Set</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleRows.map((entry, idx) => {
                    const rank = page * PAGE_SIZE + idx + 1;
                    const slug = slugifyScenarioName(entry.scenarioName);
                    return (
                      <tr key={`${entry.runId || entry.sessionId}-${idx}`} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                        {showRank && <td className={cn("px-3 py-2 tabular-nums", rank <= 3 ? "text-gold" : "text-muted-2")}>{rank}</td>}
                        <td className="max-w-[280px] px-3 py-2">
                          <Link className="block truncate text-text hover:text-cyan" to={`/scenarios/${slug}`}>{entry.scenarioName}</Link>
                          <span className="text-xs text-muted-2">{displayScenarioType(entry.scenarioType) ?? "Other"}</span>
                        </td>
                        <td className="max-w-[200px] truncate px-3 py-2">
                          <Link className="text-text hover:text-cyan" to={`/profiles/${entry.userHandle}/scenarios/${slug}`} title="Player's history on this scenario">
                            {entry.userDisplayName || entry.userHandle}
                          </Link>
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          <Link className="font-medium text-text hover:text-cyan" to={runHref(entry)} title="Open run">{formatScore(entry.score)}</Link>
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums text-muted">{formatAccuracy(entry.accuracy)}</td>
                        <td className="whitespace-nowrap px-3 py-2 text-right text-muted" title={new Date(entry.playedAtIso).toLocaleString()}>{formatRelativeTime(entry.playedAtIso)}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState title="No scores match.">
              <Button onClick={() => setParams(current => updateFilterQuery(current, { q: null, scenarioType: null }), { replace: true })}>Clear filters</Button>
            </EmptyState>
          )}
          {pageCount > 1 && <nav aria-label="Leaderboard pages" className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm">
            <span className="text-muted" aria-live="polite">Page {page + 1} of {pageCount} · {activeList.length.toLocaleString()} scores</span>
            <div className="flex gap-2">
              <Button disabled={page === 0} onClick={() => setPagination({ key: paginationKey, page: page - 1 })}>Previous</Button>
              <Button disabled={page === pageCount - 1} onClick={() => setPagination({ key: paginationKey, page: page + 1 })}>Next</Button>
            </div>
          </nav>}
        </div>

        {holders.length > 0 && (
          <Section title="Records held">
            <ol className="grid gap-1">
              {holders.slice(0, 10).map((holder, i) => (
                <li key={holder.handle}>
                  <Link to={`/profiles/${holder.handle}`} className="flex items-center justify-between gap-3 rounded px-2 py-1.5 text-sm hover:bg-panel">
                    <span className="flex min-w-0 gap-2"><span className="w-4 text-muted-2 tabular-nums">{i + 1}</span><span className="truncate">{holder.name}</span></span>
                    <span className="tabular-nums text-muted">{holder.records}</span>
                  </Link>
                </li>
              ))}
            </ol>
          </Section>
        )}
      </div>
    </PageStack>
  );
}
