import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { ComparePlayersResponse, CompareScenario } from "../gen/aimmod/hub/v1/hub_pb";
import { CompareLauncher } from "../components/PlayerScenarioStats";
import { StatCard } from "../components/StatCard";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { comparePlayers } from "../lib/api";
import { cn } from "../lib/cn";
import { formatScore } from "../lib/kovaaksStats";
import { Helmet } from "../lib/helmet";
import { useUrlState } from "../lib/urlState";

export const compareSorts = ["played", "lead", "gap", "name"] as const;

/** Percent difference of a against b; positive when a is ahead. */
export function compareDelta(a: number, b: number) {
  if (b <= 0) return a > 0 ? 100 : 0;
  return ((a - b) / b) * 100;
}

export function sortCompared(rows: readonly CompareScenario[], sort: string, query = ""): CompareScenario[] {
  const q = query.trim().toLowerCase();
  const list = rows.filter((r) => !q || r.scenarioName.toLowerCase().includes(q));
  const d = (r: CompareScenario) => compareDelta(r.score, r.otherScore);
  const cmp: Record<string, (a: CompareScenario, b: CompareScenario) => number> = {
    played: (a, b) => (b.runCount + b.otherRunCount) - (a.runCount + a.otherRunCount),
    lead: (a, b) => d(b) - d(a),
    gap: (a, b) => d(a) - d(b),
    name: (a, b) => a.scenarioName.localeCompare(b.scenarioName),
  };
  return [...list].sort((a, b) => (cmp[sort] ?? cmp.played)(a, b) || a.scenarioName.localeCompare(b.scenarioName));
}

export function ComparePage() {
  const { handle = "", other = "" } = useParams();
  const [data, setData] = useState<ComparePlayersResponse | null>(null);
  const [error, setError] = useState(false);
  const [state, setState] = useUrlState({ sort: "played", q: "" }, { sort: compareSorts });

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setError(false);
    void comparePlayers(handle, other).then((next) => { if (!cancelled) setData(next); }).catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [handle, other]);

  const rows = useMemo(() => sortCompared(data?.shared ?? [], state.sort, state.q), [data, state.sort, state.q]);

  if (error) {
    return <PageStack><EmptyState title="One of these players could not be found."><CompareLauncher handle={handle} /></EmptyState></PageStack>;
  }
  if (!data || !data.player || !data.other) return <PageStack><PageSkeleton label="Comparing players" /></PageStack>;
  const a = data.player;
  const b = data.other;
  const aName = a.displayName || a.userHandle;
  const bName = b.displayName || b.userHandle;

  return (
    <PageStack>
      <Helmet><title>{`${aName} vs ${bName} · AimMod Hub`}</title></Helmet>
      <PageHeader
        before={<Breadcrumb crumbs={[{ label: aName, to: `/profiles/${a.userHandle}` }, { label: "Compare" }]} />}
        title={<span>{aName} <span className="text-muted">vs</span> {bName}</span>}
        meta={`${data.shared.length} scenarios both have played · best scores from AimMod uploads`}
        actions={<>
          <Link to={`/profiles/${b.userHandle}/compare/${a.userHandle}`} className="text-sm text-cyan hover:underline">Swap</Link>
          <CompareLauncher handle={a.userHandle} />
        </>}
      />

      <div className="grid grid-cols-3 gap-3">
        <StatCard label={`${aName} ahead`} value={data.wins.toLocaleString()} accent="mint" />
        <StatCard label="Level" value={data.ties.toLocaleString()} />
        <StatCard label={`${bName} ahead`} value={data.losses.toLocaleString()} accent="gold" />
      </div>

      {rows.length === 0 && !state.q ? (
        <EmptyState title="No shared scenarios yet." body="Comparisons use scenarios both players have uploaded through AimMod." />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <input type="search" value={state.q} onChange={(e) => setState({ q: e.target.value })} placeholder="Filter scenarios" aria-label="Filter scenarios"
              className="min-h-9 w-full rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted sm:w-56" />
            <label className="flex items-center gap-1.5 text-sm text-muted">Sort
              <select value={state.sort} onChange={(e) => setState({ sort: e.target.value })} className="min-h-9 rounded-md border border-line bg-panel px-2 text-sm text-text">
                <option value="played">Most played</option>
                <option value="lead">{aName}'s biggest leads</option>
                <option value="gap">{aName}'s biggest gaps</option>
                <option value="name">Name</option>
              </select>
            </label>
          </div>
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="w-full text-left text-sm">
              <caption className="sr-only">Best scores on shared scenarios</caption>
              <thead className="border-b border-line text-xs text-muted">
                <tr>
                  <th scope="col" className="px-3 py-2 font-medium">Scenario</th>
                  <th scope="col" className="px-3 py-2 text-right font-medium">{aName}</th>
                  <th scope="col" className="w-40 px-3 py-2 text-center font-medium max-sm:hidden">Difference</th>
                  <th scope="col" className="px-3 py-2 font-medium">{bName}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => {
                  const delta = compareDelta(r.score, r.otherScore);
                  const width = Math.min(50, Math.abs(delta) * 2);
                  return (
                    <tr key={r.scenarioName} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                      <td className="max-w-[160px] truncate px-3 py-2 sm:max-w-[300px]"><Link className="hover:text-cyan" to={`/scenarios/${r.scenarioSlug}`}>{r.scenarioName}</Link></td>
                      <td className={cn("px-3 py-2 text-right tabular-nums", delta > 0 && "font-semibold text-mint")}>
                        <Link className="hover:text-cyan" to={`/profiles/${a.userHandle}/scenarios/${r.scenarioSlug}`}>{formatScore(r.score)}</Link>
                        <div className="text-[11px] font-normal text-muted-2">{r.runCount} runs</div>
                      </td>
                      <td className="px-3 py-2 max-sm:hidden">
                        <div className="relative h-2 rounded-full bg-white/5" aria-label={`${delta >= 0 ? "+" : ""}${delta.toFixed(1)}%`}>
                          <span className="absolute inset-y-0 left-1/2 w-px bg-white/30" />
                          <span className={cn("absolute inset-y-0 rounded-full", delta >= 0 ? "bg-mint/70" : "bg-gold/70")}
                            style={delta >= 0 ? { right: "50%", width: `${width}%` } : { left: "50%", width: `${width}%` }} />
                        </div>
                        <div className="mt-1 text-center text-[11px] tabular-nums text-muted">{delta >= 0 ? "+" : ""}{delta.toFixed(1)}%</div>
                      </td>
                      <td className={cn("px-3 py-2 tabular-nums", delta < 0 && "font-semibold text-gold")}>
                        <Link className="hover:text-cyan" to={`/profiles/${b.userHandle}/scenarios/${r.scenarioSlug}`}>{formatScore(r.otherScore)}</Link>
                        <div className="text-[11px] font-normal text-muted-2">{r.otherRunCount} runs</div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      )}
    </PageStack>
  );
}
