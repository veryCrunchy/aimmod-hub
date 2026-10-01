import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Helmet } from "../lib/helmet";
import type { BenchmarkListItem } from "../gen/aimmod/hub/v1/hub_pb";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { fetchBenchmarkList } from "../lib/api";
import { browseBenchmarkGroups, groupPlayers, type BenchmarkSort } from "../lib/benchmarkGroups";
import { filterChoice, updateFilterQuery } from "../lib/savedPageFilters";

const PAGE = 60;

export function GlobalBenchmarksPage() {
  const [benchmarks, setBenchmarks] = useState<BenchmarkListItem[] | null>(null);
  const [error, setError] = useState(false);
  const [shown, setShown] = useState(PAGE);
  const [params, setParams] = useSearchParams();
  const query = params.get("q") ?? "";
  const sort = filterChoice<BenchmarkSort>(params.get("sort"), ["players", "name"], "players");
  const rankedParam = params.get("ranked");

  const load = () => {
    void fetchBenchmarkList()
      .then((res) => { setBenchmarks(res.benchmarks ?? []); setError(false); })
      .catch(() => setError(true));
  };
  useEffect(load, []);

  const anyRanked = (benchmarks ?? []).some((b) => b.playerCount > 0);
  const rankedOnly = rankedParam == null ? anyRanked : rankedParam === "1";
  const groups = useMemo(() => browseBenchmarkGroups(benchmarks ?? [], query, sort, rankedOnly), [benchmarks, query, sort, rankedOnly]);
  useEffect(() => setShown(PAGE), [query, sort, rankedOnly]);

  const head = <Helmet>
    <title>Benchmarks · AimMod Hub</title>
    <meta name="description" content="KovaaK's benchmarks and how many Hub players are ranked in each." />
  </Helmet>;

  if (!benchmarks) {
    return <PageStack>{head}{error
      ? <><PageHeader title="Benchmarks" /><EmptyState title="Benchmarks could not be loaded."><Button onClick={load}>Try again</Button></EmptyState></>
      : <PageSkeleton stats={0} rows={10} label="Loading benchmarks" />}</PageStack>;
  }

  const rankedCount = benchmarks.filter((b) => b.playerCount > 0).length;
  const set = (patch: Record<string, string | null>) => setParams((current) => updateFilterQuery(current, patch), { replace: true });

  return (
    <PageStack>
      {head}
      <PageHeader title="Benchmarks" meta={`${benchmarks.length.toLocaleString()} benchmarks · ${rankedCount.toLocaleString()} with ranked Hub players`} />

      <div className="flex flex-wrap items-center gap-3">
        <label className="sr-only" htmlFor="benchmark-search">Search benchmarks</label>
        <input id="benchmark-search" type="search" value={query} onChange={(e) => set({ q: e.target.value || null })} placeholder="Benchmark or author"
          className="min-h-9 w-full max-w-xs rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted-2" />
        <label className="flex items-center gap-2 text-sm text-muted">
          <input type="checkbox" checked={rankedOnly} onChange={(e) => set({ ranked: e.target.checked ? "1" : "0" })} />
          Only with ranked players
        </label>
        <label className="flex items-center gap-2 text-sm text-muted">
          Sort
          <select value={sort} onChange={(e) => set({ sort: e.target.value })} className="min-h-9 rounded-md border border-line bg-panel px-2 text-sm text-text">
            <option value="players">Most players</option>
            <option value="name">Name</option>
          </select>
        </label>
      </div>

      {groups.length === 0 ? (
        <EmptyState title={rankedOnly && !query ? "No Hub players are ranked in a benchmark yet." : "No benchmarks match."}>
          {rankedOnly ? <Button onClick={() => set({ ranked: "0" })}>Show all benchmarks</Button> : <Button onClick={() => set({ q: null })}>Clear search</Button>}
        </EmptyState>
      ) : (
        <>
          <ul className="divide-y divide-line overflow-hidden rounded-md border border-line">
            {groups.slice(0, shown).map((group) => {
              const players = groupPlayers(group);
              const single = group.variants.length === 1 ? group.variants[0].item : null;
              return (
                <li key={group.base + group.iconUrl} className="flex flex-wrap items-center gap-3 px-3 py-2.5 hover:bg-white/[0.02]">
                  {group.iconUrl
                    ? <img src={group.iconUrl} alt="" className="h-8 w-8 shrink-0 rounded border border-line object-cover" loading="lazy" />
                    : <span aria-hidden="true" className="h-8 w-8 shrink-0 rounded border border-line bg-bg-2" />}
                  <div className="min-w-0 flex-1">
                    {single
                      ? <Link to={`/benchmarks/${single.benchmarkId}`} className="block truncate text-sm font-medium text-text hover:text-cyan">{single.benchmarkName}</Link>
                      : <span className="block truncate text-sm font-medium text-text">{group.base}</span>}
                    <span className="block truncate text-xs text-muted-2">{[group.author && `by ${group.author}`, group.type].filter(Boolean).join(" · ")}</span>
                  </div>
                  {!single && (
                    <div className="flex flex-wrap gap-1.5">
                      {group.variants.map(({ item, difficulty }) => (
                        <Link key={item.benchmarkId} to={`/benchmarks/${item.benchmarkId}`} className="rounded-full border border-line px-2.5 py-0.5 text-xs capitalize text-muted hover:border-line-strong hover:text-text">
                          {difficulty ?? item.benchmarkName}{item.playerCount > 0 ? <span className="ml-1 tabular-nums text-muted-2">{item.playerCount}</span> : null}
                        </Link>
                      ))}
                    </div>
                  )}
                  <span className="w-24 shrink-0 text-right text-xs tabular-nums text-muted">{players > 0 ? `${players.toLocaleString()} ranked` : "No Hub ranks"}</span>
                </li>
              );
            })}
          </ul>
          {groups.length > shown && <Button onClick={() => setShown((n) => n + PAGE)}>Show more ({(groups.length - shown).toLocaleString()} left)</Button>}
        </>
      )}
    </PageStack>
  );
}
