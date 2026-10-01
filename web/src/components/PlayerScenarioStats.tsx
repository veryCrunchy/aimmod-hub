import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { ActivityDay, GetPlayerScenarioStatsResponse, PlayerScenarioStat } from "../gen/aimmod/hub/v1/hub_pb";
import { fetchPlayerScenarioStats, formatRelativeTime, quickSearch } from "../lib/api";
import { cn } from "../lib/cn";
import { formatAccuracy, formatScore } from "../lib/kovaaksStats";
import { useUrlState } from "../lib/urlState";
import { ScenarioTypeBadge } from "./ScenarioTypeBadge";
import { EmptyState } from "./ui/EmptyState";
import { Section } from "./ui/Section";
import { TableSkeleton } from "./ui/Skeleton";

export const statSorts = ["recent", "runs", "best", "percentile", "trend", "consistency", "name"] as const;
export type StatSort = (typeof statSorts)[number];

/** Orders scenario stats for the table. */
export function sortStats(stats: readonly PlayerScenarioStat[], sort: StatSort, query = ""): PlayerScenarioStat[] {
  const q = query.trim().toLowerCase();
  const list = stats.filter((s) => !q || s.scenarioName.toLowerCase().includes(q) || s.scenarioType.toLowerCase().includes(q));
  const by: Record<StatSort, (a: PlayerScenarioStat, b: PlayerScenarioStat) => number> = {
    recent: (a, b) => b.lastPlayedAtIso.localeCompare(a.lastPlayedAtIso),
    runs: (a, b) => b.runCount - a.runCount,
    best: (a, b) => b.bestScore - a.bestScore,
    percentile: (a, b) => b.percentile - a.percentile || a.hubRank - b.hubRank,
    trend: (a, b) => b.trendPct - a.trendPct,
    consistency: (a, b) => b.consistency - a.consistency,
    name: (a, b) => a.scenarioName.localeCompare(b.scenarioName),
  };
  return [...list].sort((a, b) => by[sort](a, b) || a.scenarioName.localeCompare(b.scenarioName));
}

/** Tiny line of recent scores; the last point is highlighted. */
export function Sparkline({ values, className }: { values: readonly number[]; className?: string }) {
  if (values.length < 2) return <span className="text-xs text-muted-2">–</span>;
  const w = 72;
  const h = 20;
  const lo = Math.min(...values);
  const hi = Math.max(...values);
  const span = hi - lo || 1;
  const pts = values.map((v, i) => [(i / (values.length - 1)) * w, h - 2 - ((v - lo) / span) * (h - 4)] as const);
  const last = pts[pts.length - 1];
  return (
    <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} className={cn("overflow-visible", className)} aria-hidden>
      <polyline points={pts.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(" ")} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" className="text-cyan/70" />
      <circle cx={last[0]} cy={last[1]} r="2" className="fill-mint" />
    </svg>
  );
}

function trendText(pct: number) {
  if (!pct) return <span className="text-muted-2">–</span>;
  return <span className={pct > 0 ? "text-mint" : "text-gold"}>{pct > 0 ? "+" : ""}{pct.toFixed(1)}%</span>;
}

/** Last 26 weeks of runs as a calendar strip. */
export function ActivityStrip({ days, weeks = 26 }: { days: readonly ActivityDay[]; weeks?: number }) {
  const counts = useMemo(() => new Map(days.map((d) => [d.dateIso, d.runCount])), [days]);
  const max = Math.max(1, ...days.map((d) => d.runCount));
  const today = new Date();
  const end = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth(), today.getUTCDate()));
  const start = new Date(end);
  start.setUTCDate(start.getUTCDate() - (weeks * 7 - 1) - end.getUTCDay());
  const cells: { iso: string; count: number }[] = [];
  for (let d = new Date(start); d <= end; d.setUTCDate(d.getUTCDate() + 1)) {
    const iso = d.toISOString().slice(0, 10);
    cells.push({ iso, count: counts.get(iso) ?? 0 });
  }
  const active = days.filter((d) => d.runCount > 0).length;
  return (
    <div>
      <div className="hub-scroll overflow-x-auto pb-1">
        <div className="grid w-max grid-flow-col grid-rows-7 gap-[3px]" role="img" aria-label={`${active} active days in the last ${weeks} weeks`}>
          {cells.map((c) => (
            <span key={c.iso} title={`${c.iso}: ${c.count} ${c.count === 1 ? "run" : "runs"}`} className="h-2.5 w-2.5 rounded-[2px]"
              style={{ background: c.count ? `rgba(121, 201, 151, ${0.25 + 0.75 * (c.count / max)})` : "rgba(255,255,255,0.05)" }} />
          ))}
        </div>
      </div>
      <p className="mt-1.5 text-xs text-muted-2">{active} active {active === 1 ? "day" : "days"} in the last {weeks} weeks</p>
    </div>
  );
}

const thSort = (sort: StatSort, current: StatSort, onSort: (s: StatSort) => void, label: string, align = "text-right") => (
  <th scope="col" className={cn("px-3 py-2 font-medium", align)} aria-sort={sort === current ? "descending" : "none"}>
    <button type="button" className={cn("hover:text-text", sort === current && "text-text underline underline-offset-4")} onClick={() => onSort(sort)}>{label}</button>
  </th>
);

/** Per-scenario bests, trend, consistency and AimMod standing for a player. */
export function PlayerScenarioStats({ handle }: { handle: string }) {
  const [data, setData] = useState<GetPlayerScenarioStatsResponse | null>(null);
  const [error, setError] = useState(false);
  const [state, setState] = useUrlState({ ssort: "recent", srange: "all", sq: "" }, { ssort: statSorts, srange: ["7d", "30d", "90d", "all"] });
  const [shown, setShown] = useState(15);

  useEffect(() => {
    let cancelled = false;
    setError(false);
    void fetchPlayerScenarioStats(handle, state.srange === "all" ? "" : state.srange)
      .then((next) => { if (!cancelled) setData(next); })
      .catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [handle, state.srange]);

  const rows = useMemo(() => sortStats(data?.scenarios ?? [], state.ssort as StatSort, state.sq), [data, state.ssort, state.sq]);
  const sort = state.ssort as StatSort;
  const onSort = (s: StatSort) => setState({ ssort: s });

  return (
    <Section title="Scenarios" aside={data ? `${data.scenarios.length} played${state.srange !== "all" ? " in this period" : ""}` : undefined}>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <input type="search" value={state.sq} onChange={(e) => setState({ sq: e.target.value })} placeholder="Filter scenarios" aria-label="Filter scenarios"
          className="min-h-9 w-full rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted sm:w-56" />
        <select aria-label="Period" value={state.srange} onChange={(e) => setState({ srange: e.target.value })} className="min-h-9 rounded-md border border-line bg-panel px-2 text-sm">
          <option value="7d">Last 7 days</option>
          <option value="30d">Last 30 days</option>
          <option value="90d">Last 90 days</option>
          <option value="all">All time</option>
        </select>
        <select aria-label="Sort" value={sort} onChange={(e) => onSort(e.target.value as StatSort)} className="min-h-9 rounded-md border border-line bg-panel px-2 text-sm sm:hidden">
          <option value="recent">Recently played</option>
          <option value="runs">Most runs</option>
          <option value="best">Best score</option>
          <option value="percentile">AimMod standing</option>
          <option value="trend">Improving</option>
          <option value="consistency">Most consistent</option>
          <option value="name">Name</option>
        </select>
      </div>
      {error ? <EmptyState title="Scenario stats could not be loaded." />
        : !data ? <TableSkeleton rows={6} />
        : rows.length === 0 ? <EmptyState title={state.sq ? "No scenario matches." : "No runs in this period."} />
        : (
          <>
            <div className="overflow-x-auto rounded-md border border-line">
              <table className="w-full text-left text-sm">
                <caption className="sr-only">Scenario statistics</caption>
                <thead className="border-b border-line text-xs text-muted">
                  <tr>
                    {thSort("name", sort, onSort, "Scenario", "text-left")}
                    {thSort("best", sort, onSort, "Best")}
                    {thSort("percentile", sort, onSort, "AimMod rank")}
                    <th scope="col" className="px-3 py-2 font-medium max-md:hidden">Last 20</th>
                    {thSort("trend", sort, onSort, "Trend")}
                    {thSort("consistency", sort, onSort, "Steadiness")}
                    {thSort("runs", sort, onSort, "Runs")}
                    {thSort("recent", sort, onSort, "Played")}
                  </tr>
                </thead>
                <tbody>
                  {rows.slice(0, shown).map((s) => (
                    <tr key={s.scenarioName} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                      <td className="max-w-[180px] px-3 py-2 sm:max-w-[280px]">
                        <Link className="block truncate hover:text-cyan" to={`/profiles/${handle}/scenarios/${s.scenarioSlug}`}>{s.scenarioName}</Link>
                        <span className="max-sm:hidden"><ScenarioTypeBadge type={s.scenarioType} /></span>
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        <div className="font-medium text-gold">{formatScore(s.bestScore)}</div>
                        <div className="text-[11px] text-muted-2">{formatAccuracy(s.bestAccuracy)}</div>
                      </td>
                      <td className="px-3 py-2 text-right tabular-nums">
                        <Link to={`/scenarios/${s.scenarioSlug}`} className="hover:text-cyan">#{s.hubRank} <span className="text-muted-2">of {s.hubPlayers}</span></Link>
                        <div className="text-[11px] text-muted-2">beats {s.percentile}%</div>
                      </td>
                      <td className="px-3 py-2 max-md:hidden"><Sparkline values={s.recentScores} /></td>
                      <td className="px-3 py-2 text-right tabular-nums">{trendText(s.trendPct)}</td>
                      <td className="px-3 py-2 text-right tabular-nums text-muted">{s.consistency ? Math.round(s.consistency) : "–"}</td>
                      <td className="px-3 py-2 text-right tabular-nums text-muted">{s.runCount.toLocaleString()}</td>
                      <td className="whitespace-nowrap px-3 py-2 text-right text-muted">{formatRelativeTime(s.lastPlayedAtIso)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {rows.length > shown ? <button type="button" className="mt-3 w-full rounded-md border border-line bg-panel py-2 text-sm hover:border-line-strong" onClick={() => setShown((n) => n + 25)}>Show more scenarios ({rows.length - shown} left)</button> : null}
            <p className="mt-2 text-xs text-muted-2">Trend compares the last five runs with the five before. Steadiness is 100 when recent runs score the same.</p>
          </>
        )}
    </Section>
  );
}

/** Picks another AimMod player to compare with. */
export function CompareLauncher({ handle }: { handle: string }) {
  const navigate = useNavigate();
  const [value, setValue] = useState("");
  const [options, setOptions] = useState<{ handle: string; title: string }[]>([]);
  useEffect(() => {
    const q = value.trim();
    if (q.length < 2) { setOptions([]); return; }
    let cancelled = false;
    const t = setTimeout(() => {
      void quickSearch(q, false, 12).then((r) => {
        if (cancelled) return;
        setOptions(r.results.filter((x) => x.kind === "player" && x.userHandle.toLowerCase() !== handle.toLowerCase()).slice(0, 6).map((x) => ({ handle: x.userHandle, title: x.title })));
      }).catch(() => {});
    }, 150);
    return () => { cancelled = true; clearTimeout(t); };
  }, [value, handle]);
  const listId = `compare-${handle}`;
  return (
    <form className="flex items-center gap-2" onSubmit={(e) => {
      e.preventDefault();
      const pick = options.find((o) => o.title.toLowerCase() === value.trim().toLowerCase() || o.handle.toLowerCase() === value.trim().toLowerCase()) ?? options[0];
      const other = pick?.handle ?? value.trim().replace(/^@/, "");
      if (other) navigate(`/profiles/${handle}/compare/${encodeURIComponent(other)}`);
    }}>
      <input list={listId} value={value} onChange={(e) => setValue(e.target.value)} placeholder="Compare with…" aria-label="Compare with another player"
        className="min-h-10 w-40 rounded-md border border-line bg-panel px-3 text-sm placeholder:text-muted" />
      <datalist id={listId}>{options.map((o) => <option key={o.handle} value={o.handle}>{o.title}</option>)}</datalist>
      <button type="submit" className="inline-flex min-h-10 items-center rounded-md border border-line bg-panel px-3 text-sm font-medium hover:border-line-strong">Compare</button>
    </form>
  );
}
