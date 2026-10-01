import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Helmet } from "../lib/helmet";
import type { BenchmarkListItem } from "../gen/aimmod/hub/v1/hub_pb";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { fetchBenchmarkList } from "../lib/api";
import { browseBenchmarkGroups, groupHidden, groupKovaaksPlayers, groupPlayers, type BenchmarkSort } from "../lib/benchmarkGroups";
import { useUrlState } from "../lib/urlState";

const PAGE = 60;

function compactCount(n: number) {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
  if (n >= 10_000) return `${Math.round(n / 1000)}k`;
  if (n >= 1000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  return n.toLocaleString();
}

export function GlobalBenchmarksPage() {
  const [benchmarks, setBenchmarks] = useState<BenchmarkListItem[] | null>(null);
  const [error, setError] = useState(false);
  const [shown, setShown] = useState(PAGE);
  const [state, setState] = useUrlState({ q: "", sort: "kovaaks", ranked: "", all: "" }, { sort: ["kovaaks", "players", "name"] });

  const load = () => {
    void fetchBenchmarkList()
      .then((res) => { setBenchmarks(res.benchmarks ?? []); setError(false); })
      .catch(() => setError(true));
  };
  useEffect(load, []);

  const includeHidden = state.all === "1";
  const rankedOnly = state.ranked === "1";
  const groups = useMemo(() => browseBenchmarkGroups(benchmarks ?? [], state.q, state.sort as BenchmarkSort, rankedOnly, includeHidden), [benchmarks, state.q, state.sort, rankedOnly, includeHidden]);
  useEffect(() => setShown(PAGE), [state.q, state.sort, rankedOnly, includeHidden]);

  const head = <Helmet>
    <title>Benchmarks · AimMod Hub</title>
    <meta name="description" content="KovaaK's benchmarks with rank thresholds, KovaaK's popularity and AimMod leaderboards." />
  </Helmet>;

  if (!benchmarks) {
    return <PageStack>{head}{error
      ? <><PageHeader title="Benchmarks" /><EmptyState title="Benchmarks could not be loaded."><Button onClick={load}>Try again</Button></EmptyState></>
      : <PageSkeleton stats={0} rows={10} label="Loading benchmarks" />}</PageStack>;
  }

  const hiddenCount = benchmarks.filter((b) => b.hidden).length;
  const rankedCount = benchmarks.filter((b) => b.playerCount > 0).length;

  return (
    <PageStack>
      {head}
      <PageHeader title="Benchmarks" meta={[
        `${(benchmarks.length - hiddenCount).toLocaleString()} benchmarks`,
        rankedCount ? `${rankedCount.toLocaleString()} with ranked AimMod players` : null,
        hiddenCount && !includeHidden ? `${hiddenCount.toLocaleString()} test, empty or barely played hidden` : null,
      ].filter(Boolean).join(" · ")} />

      <div className="flex flex-wrap items-center gap-3">
        <label className="sr-only" htmlFor="benchmark-search">Search benchmarks</label>
        <input id="benchmark-search" type="search" value={state.q} onChange={(e) => setState({ q: e.target.value })} placeholder="Benchmark or author"
          className="min-h-9 w-full max-w-xs rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted-2" />
        <label className="flex items-center gap-2 text-sm text-muted">
          Sort
          <select value={state.sort} onChange={(e) => setState({ sort: e.target.value })} className="min-h-9 rounded-md border border-line bg-panel px-2 text-sm text-text">
            <option value="kovaaks">Most played on KovaaK's</option>
            <option value="players">Most AimMod players ranked</option>
            <option value="name">Name</option>
          </select>
        </label>
        <label className="flex items-center gap-2 text-sm text-muted">
          <input type="checkbox" checked={rankedOnly} onChange={(e) => setState({ ranked: e.target.checked ? "1" : "" })} />
          Only with ranked AimMod players
        </label>
        {hiddenCount ? (
          <label className="flex items-center gap-2 text-sm text-muted">
            <input type="checkbox" checked={includeHidden} onChange={(e) => setState({ all: e.target.checked ? "1" : "" })} />
            Show all ({hiddenCount.toLocaleString()} hidden)
          </label>
        ) : null}
      </div>

      {groups.length === 0 ? (
        <EmptyState title={rankedOnly ? "No AimMod players are ranked in a matching benchmark yet." : "No benchmarks match."}>
          {rankedOnly ? <Button onClick={() => setState({ ranked: "" })}>Show every benchmark</Button> : <Button onClick={() => setState({ q: "" })}>Clear search</Button>}
        </EmptyState>
      ) : (
        <>
          <ul className="divide-y divide-line overflow-hidden rounded-md border border-line">
            {groups.slice(0, shown).map((group) => {
              const players = groupPlayers(group);
              const kovaaks = groupKovaaksPlayers(group);
              const single = group.variants.length === 1 ? group.variants[0].item : null;
              const hidden = groupHidden(group);
              const scenarios = Math.max(...group.variants.map((v) => v.item.scenarioCount));
              return (
                <li key={group.base + group.iconUrl + group.variants[0].item.benchmarkId} className="flex flex-wrap items-center gap-3 px-3 py-2.5 hover:bg-white/[0.02]">
                  {group.iconUrl
                    ? <img src={group.iconUrl} alt="" className="h-8 w-8 shrink-0 rounded border border-line object-cover" loading="lazy" />
                    : <span aria-hidden="true" className="h-8 w-8 shrink-0 rounded border border-line bg-bg-2" />}
                  <div className="min-w-0 flex-1">
                    {single
                      ? <Link to={`/benchmarks/${single.benchmarkId}`} className="block truncate text-sm font-medium text-text hover:text-cyan">{single.benchmarkName}</Link>
                      : <span className="block truncate text-sm font-medium text-text">{group.base}</span>}
                    <span className="block truncate text-xs text-muted-2">
                      {[group.author && `by ${group.author}`, scenarios > 0 ? `${scenarios} scenarios` : null, hidden ? group.variants[0].item.hiddenReason : null].filter(Boolean).join(" · ")}
                    </span>
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
                  <span className="grid w-32 shrink-0 text-right text-xs tabular-nums text-muted max-sm:w-full max-sm:text-left">
                    <span>{kovaaks > 0 ? `${compactCount(kovaaks)} on KovaaK's` : "KovaaK's count pending"}</span>
                    <span className="text-muted-2">{players > 0 ? `${players.toLocaleString()} AimMod ranked` : "No AimMod ranks yet"}</span>
                  </span>
                </li>
              );
            })}
          </ul>
          {groups.length > shown && <Button onClick={() => setShown((n) => n + PAGE)}>Show more ({(groups.length - shown).toLocaleString()} left)</Button>}
          <p className="text-xs text-muted-2">KovaaK's counts are the players on each benchmark's first scenario, read slowly from KovaaK's public leaderboards.</p>
        </>
      )}
    </PageStack>
  );
}
