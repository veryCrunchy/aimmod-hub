import { Fragment, useMemo } from "react";
import { Link } from "react-router-dom";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { cn } from "../lib/cn";
import {
  compactScore,
  groupCategories,
  hasRankName,
  rankColor,
  rankCounts,
  rankProgress,
  sheetColumns,
  sortScenarios,
  splitCategory,
  type SheetCategory,
  type SheetRank,
  type SheetScenario,
  type SheetSort,
  type SheetThreshold,
} from "../lib/benchmarkSheet";
import { RankBadge } from "./RankBadge";
import { Tabs } from "./ui/Tabs";

export const sheetSorts = ["sheet", "closest", "weakest"] as const;

type SheetProps = {
  categories: readonly SheetCategory[];
  sort: SheetSort;
  onSortChange: (sort: SheetSort) => void;
  /** Link for a scenario name; omit to render plain text. */
  scenarioHref?: (scenario: SheetScenario) => string | undefined;
};

function scoreText(score: number) {
  return score > 0 ? compactScore(Math.round(score * 10) / 10) : "–";
}

/** Bar from the current rank to the next, in the next rank's colour. */
function NextRankBar({ scenario }: { scenario: SheetScenario }) {
  const progress = rankProgress(scenario.score, scenario.thresholds);
  if (!scenario.thresholds.length) return <span className="text-xs text-muted-2">No thresholds</span>;
  if (progress.maxed) {
    return <span className="text-xs text-mint">Top rank{progress.pctOverTop > 0 ? ` · +${progress.pctOverTop}%` : ""}</span>;
  }
  const next = progress.next!;
  const color = rankColor(next.color, next.rankIndex);
  return (
    <div className="min-w-[132px]">
      <div className="flex items-baseline justify-between gap-2 text-xs">
        <span style={{ color }} className="truncate font-medium">{next.rankName}</span>
        <span className="tabular-nums text-muted">+{compactScore(Math.ceil(progress.pointsToNext))}</span>
      </div>
      <div className="mt-1 h-1.5 overflow-hidden rounded-full bg-white/8" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress.pctToNext} aria-label={`${progress.pctToNext}% of the way to ${next.rankName}`}>
        <div className="h-full rounded-full" style={{ width: `${progress.pctToNext}%`, background: color }} />
      </div>
    </div>
  );
}

/** One threshold cell: filled when met, partly filled while it is the next rank. */
function ThresholdCell({ threshold, score, previous }: { threshold?: SheetThreshold; score: number; previous: number }) {
  if (!threshold) return <td className="px-0.5 py-1.5" />;
  const color = rankColor(threshold.color, threshold.rankIndex);
  const met = score >= threshold.score;
  const partial = !met && score > previous ? Math.min(100, ((score - previous) / Math.max(1, threshold.score - previous)) * 100) : 0;
  return (
    <td className="px-0.5 py-1.5">
      <div className="relative h-6 min-w-12 overflow-hidden rounded-sm bg-white/[0.04]" title={`${threshold.rankName}: ${threshold.score.toLocaleString()}`}>
        <div className="absolute inset-y-0 left-0" style={{ width: met ? "100%" : `${partial}%`, background: met ? color : `${color}55` }} />
        <span className={cn("relative z-10 flex h-full items-center justify-center text-[11px] tabular-nums", met ? "font-semibold text-black/75" : "text-muted")}>
          {compactScore(threshold.score)}
        </span>
      </div>
    </td>
  );
}

function ScenarioName({ scenario, href }: { scenario: SheetScenario; href?: string }) {
  return href ? (
    <Link to={href} className="truncate text-text hover:text-cyan">{scenario.scenarioName}</Link>
  ) : <span className="truncate text-text">{scenario.scenarioName}</span>;
}

function ScoreCell({ scenario }: { scenario: SheetScenario }) {
  return (
    <div className="text-right">
      <div className="font-medium tabular-nums">{scoreText(scenario.score)}</div>
      <div className="text-[11px] text-muted-2">
        {scenario.scoreSource === "aimmod" ? <span className="text-cyan" title="Your AimMod upload beat the KovaaK's score">AimMod</span> : scenario.leaderboardRank ? `#${scenario.leaderboardRank.toLocaleString()}` : null}
      </div>
    </div>
  );
}

function currentRankOf(scenario: SheetScenario): SheetRank | null {
  const { current } = rankProgress(scenario.score, scenario.thresholds);
  return current;
}

export function BenchmarkSheet({ categories, sort, onSortChange, scenarioHref }: SheetProps) {
  const columns = useMemo(() => sheetColumns(categories), [categories]);
  const groups = useMemo(() => groupCategories(categories), [categories]);
  const ordered = useMemo(() => sortScenarios(categories, sort), [categories, sort]);
  const dense = columns.length > 9;

  const thresholdsFor = (scenario: SheetScenario) => {
    const byIndex = new Map(scenario.thresholds.map((t) => [t.rankIndex, t]));
    return columns.map((column) => byIndex.get(column.rankIndex));
  };

  const row = (scenario: SheetScenario, lead: React.ReactNode) => {
    const cells = thresholdsFor(scenario);
    const current = currentRankOf(scenario);
    return (
      <tr key={`${scenario.categoryName}:${scenario.scenarioName}`} className="border-b border-line/50 last:border-b-0 hover:bg-white/[0.02]">
        {lead}
        <td className="max-w-[280px] px-3 py-1.5">
          <div className="flex min-w-0 items-center gap-2">
            <ScenarioName scenario={scenario} href={scenarioHref?.(scenario)} />
          </div>
          {sort !== "sheet" ? <div className="text-[11px] text-muted-2">{scenario.categoryName}</div> : null}
        </td>
        <td className="px-2 py-1.5"><RankBadge rankName={current?.rankName} iconUrl={current?.iconUrl} color={current?.color} rankIndex={current?.rankIndex} emptyLabel="–" /></td>
        <td className="px-3 py-1.5"><ScoreCell scenario={scenario} /></td>
        <td className="px-3 py-1.5"><NextRankBar scenario={scenario} /></td>
        {!dense && cells.map((threshold, i) => {
          const previous = i === 0 ? 0 : cells[i - 1]?.score ?? 0;
          return <ThresholdCell key={columns[i].rankIndex} threshold={threshold} score={scenario.score} previous={previous} />;
        })}
      </tr>
    );
  };

  return (
    <div className="grid gap-3">
      <Tabs label="Sort scenarios" value={sort} onChange={onSortChange}
        tabs={[["sheet", "Sheet order"], ["closest", "Closest to next rank"], ["weakest", "Weakest first"]]} />

      {/* Desktop and tablet: the sheet as a table. */}
      <div className="hub-scroll overflow-x-auto rounded-md border border-line max-md:hidden">
        <table className="w-full text-left text-sm">
          <caption className="sr-only">Scenario scores against rank thresholds</caption>
          <thead className="border-b border-line bg-white/[0.02] text-xs text-muted">
            <tr>
              {sort === "sheet" ? <th scope="col" className="w-24 px-3 py-2 font-medium">Category</th> : null}
              <th scope="col" className="px-3 py-2 font-medium">Scenario</th>
              <th scope="col" className="px-2 py-2 font-medium">Rank</th>
              <th scope="col" className="px-3 py-2 text-right font-medium">Score</th>
              <th scope="col" className="px-3 py-2 font-medium">Next rank</th>
              {!dense && columns.map((column) => (
                <th key={column.rankIndex} scope="col" className="px-0.5 py-2 text-center font-medium">
                  <span className="inline-flex items-center gap-1" style={{ color: rankColor(column.color, column.rankIndex) }}>
                    {column.iconUrl ? <img src={column.iconUrl} alt="" className="h-3.5 w-3.5 object-contain" /> : null}
                    <span className="max-w-16 truncate">{column.rankName}</span>
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sort === "sheet"
              ? groups.map((group) => group.categories.map((category) => (
                <Fragment key={category.categoryName}>
                  {category.scenarios.map((scenario, i) => row(scenario, i === 0 ? (
                    <td rowSpan={category.scenarios.length} className="border-r border-line/50 px-3 py-1.5 align-middle text-xs">
                      <div className="font-medium text-text">{splitCategory(category.categoryName).sub ?? category.categoryName}</div>
                      <div className="text-muted-2">{splitCategory(category.categoryName).sub ? group.parent : null}</div>
                    </td>
                  ) : null))}
                </Fragment>
              )))
              : ordered.map((scenario) => row(scenario, null))}
          </tbody>
        </table>
        {dense ? <p className="border-t border-line px-3 py-2 text-xs text-muted-2">This benchmark has {columns.length} ranks; the next-rank column shows what is left for each scenario.</p> : null}
      </div>

      {/* Phones: one card per scenario with a segmented ladder. */}
      <ol className="grid gap-2 md:hidden">
        {ordered.map((scenario) => {
          const current = currentRankOf(scenario);
          const sorted = [...scenario.thresholds].sort((a, b) => a.rankIndex - b.rankIndex);
          return (
            <li key={`${scenario.categoryName}:${scenario.scenarioName}`} className="rounded-md border border-line bg-panel px-3 py-2.5">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex min-w-0 text-sm"><ScenarioName scenario={scenario} href={scenarioHref?.(scenario)} /></div>
                  <div className="text-[11px] text-muted-2">{scenario.categoryName}</div>
                </div>
                <ScoreCell scenario={scenario} />
              </div>
              <div className="mt-2 flex items-center gap-2">
                <RankBadge rankName={current?.rankName} iconUrl={current?.iconUrl} color={current?.color} rankIndex={current?.rankIndex} emptyLabel="Unranked" />
                <div className="min-w-0 flex-1"><NextRankBar scenario={scenario} /></div>
              </div>
              {sorted.length ? (
                <div className="mt-2 flex h-1.5 gap-px overflow-hidden rounded-full" aria-hidden>
                  {sorted.map((t) => (
                    <span key={t.rankIndex} className="flex-1" style={{ background: scenario.score >= t.score ? rankColor(t.color, t.rankIndex) : "rgba(255,255,255,0.08)" }} />
                  ))}
                </div>
              ) : null}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

type SummaryProps = {
  categories: readonly SheetCategory[];
  overallRank?: SheetRank | null;
  benchmarkProgress?: number;
};

/** Overall rank, how many scenarios sit at each rank and KovaaK's progress. */
export function BenchmarkSummary({ categories, overallRank, benchmarkProgress }: SummaryProps) {
  const { counts, unranked, total } = useMemo(() => rankCounts(categories), [categories]);
  const fromAimmod = categories.flatMap((c) => c.scenarios).filter((s) => s.scoreSource === "aimmod").length;
  return (
    <div className="grid gap-3 rounded-md border border-line bg-panel p-3 sm:grid-cols-[auto_minmax(0,1fr)] sm:items-center sm:gap-5 sm:p-4">
      <div className="flex flex-col gap-1">
        <span className="text-xs text-muted">Overall rank</span>
        <RankBadge size="lg" rankName={overallRank?.rankName} iconUrl={overallRank?.iconUrl} color={overallRank?.color} rankIndex={overallRank?.rankIndex} emptyLabel="Not ranked yet" />
        <span className="text-[11px] text-muted-2">Set by the lowest scenario rank.</span>
      </div>
      <div className="grid gap-2">
        <div className="flex flex-wrap gap-x-5 gap-y-1 text-sm">
          <span><strong className="tabular-nums">{(total - unranked).toLocaleString()}</strong> <span className="text-muted">of {total.toLocaleString()} scenarios ranked</span></span>
          {benchmarkProgress ? <span><strong className="tabular-nums">{Math.round(benchmarkProgress).toLocaleString()}</strong> <span className="text-muted">KovaaK's progress</span></span> : null}
          {fromAimmod ? <span className="text-cyan">{fromAimmod} {fromAimmod === 1 ? "score" : "scores"} from AimMod uploads</span> : null}
        </div>
        <ul className="flex flex-wrap gap-1.5" aria-label="Scenarios per rank">
          {counts.map((count) => (
            <li key={count.rankIndex} className="inline-flex items-center gap-1.5 rounded-md border border-line px-2 py-1 text-xs">
              <span aria-hidden className="h-2 w-2 rounded-full" style={{ background: rankColor(count.color, count.rankIndex) }} />
              <span>{count.rankName}</span>
              <span className="tabular-nums text-muted">{count.count}</span>
            </li>
          ))}
          {unranked ? (
            <li className="inline-flex items-center gap-1.5 rounded-md border border-line px-2 py-1 text-xs text-muted">
              Unranked <span className="tabular-nums">{unranked}</span>
            </li>
          ) : null}
        </ul>
      </div>
    </div>
  );
}

type HistoryPoint = { dateIso: string; overallRankIndex: number; averageRankIndex: number; rankedScenarios: number };

/** Rank over time, replayed from the player's own AimMod uploads. */
export function RankHistoryChart({ points, ranks }: { points: readonly HistoryPoint[]; ranks: readonly SheetRank[] }) {
  const data = useMemo(() => points.map((p) => ({ date: p.dateIso, average: p.averageRankIndex, overall: p.overallRankIndex, ranked: p.rankedScenarios })), [points]);
  const top = Math.max(1, ranks.length - 1);
  const name = (index: number) => {
    const rank = ranks[Math.round(index)];
    return rank && hasRankName(rank.rankName) ? rank.rankName : "Unranked";
  };
  if (data.length < 2) return null;
  return (
    <div className="h-56 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
          <CartesianGrid stroke="rgba(255,255,255,0.06)" vertical={false} />
          <XAxis dataKey="date" tick={{ fill: "rgba(200,210,205,0.6)", fontSize: 11 }} tickLine={false} axisLine={false} minTickGap={24} />
          <YAxis domain={[0, top]} allowDecimals={false} tickFormatter={name} width={72} tick={{ fill: "rgba(200,210,205,0.6)", fontSize: 11 }} tickLine={false} axisLine={false} />
          <Tooltip
            contentStyle={{ background: "#121815", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, fontSize: 12 }}
            formatter={(value: unknown, key: unknown) => key === "average" ? [`${Number(value).toFixed(2)} (${name(Number(value))})`, "Average rank"] : [name(Number(value)), "Overall rank"]}
          />
          <Area type="stepAfter" dataKey="average" stroke="#5ec8d8" fill="#5ec8d8" fillOpacity={0.12} strokeWidth={1.5} />
          <Area type="stepAfter" dataKey="overall" stroke="#e0b840" fill="transparent" strokeWidth={1.5} strokeDasharray="4 3" />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
