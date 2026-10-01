import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import type { BenchmarkListItem, BenchmarkSummary as BenchmarkSummaryMessage, GetBenchmarkPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { fetchBenchmarkList } from "../lib/api";
import { groupBenchmarks } from "../lib/benchmarkGroups";
import type { SheetScenario, SheetSort } from "../lib/benchmarkSheet";
import { cn } from "../lib/cn";
import { useUrlState } from "../lib/urlState";
import { BenchmarkSheet, BenchmarkSummary, RankHistoryChart, sheetSorts } from "./BenchmarkSheet";
import { Breadcrumb } from "./ui/Breadcrumb";
import { Section } from "./ui/Section";

type Props = {
  page: GetBenchmarkPageResponse;
  /** Where the player's crumb and difficulty links point. */
  playerHref: string;
  playerName: string;
  /** Builds the link to a sibling difficulty of this benchmark. */
  benchmarkHref: (benchmarkId: number) => string;
  scenarioHref: (scenario: SheetScenario) => string | undefined;
  /** The player's own benchmark ranks, to label difficulty chips. */
  playerBenchmarks?: readonly BenchmarkSummaryMessage[];
  /** Shown above the sheet, for example the "not on AimMod" notice. */
  notice?: React.ReactNode;
  /** Link to the player's benchmark list; null hides that crumb. */
  benchmarksHref?: string | null;
};

export function BenchmarkSheetView({ page, playerHref, playerName, benchmarkHref, scenarioHref, playerBenchmarks, notice, benchmarksHref }: Props) {
  const [catalog, setCatalog] = useState<BenchmarkListItem[]>([]);
  const [state, setState] = useUrlState({ sort: "sheet" }, { sort: sheetSorts });
  useEffect(() => {
    let cancelled = false;
    void fetchBenchmarkList().then((r) => { if (!cancelled) setCatalog(r.benchmarks); }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

  const siblings = useMemo(() => {
    const groups = groupBenchmarks(catalog);
    const group = groups.find((g) => g.variants.some((v) => v.item.benchmarkId === page.benchmarkId));
    return group && group.variants.length > 1 ? group : null;
  }, [catalog, page.benchmarkId]);
  const ranksById = useMemo(() => new Map((playerBenchmarks ?? []).map((b) => [b.benchmarkId, b.overallRank])), [playerBenchmarks]);
  const title = siblings ? siblings.base : page.benchmarkName || `Benchmark ${page.benchmarkId}`;

  return (
    <div className="grid gap-5">
      <header className="flex flex-wrap items-start justify-between gap-4 pt-1">
        <div className="flex min-w-0 items-start gap-3">
          {page.benchmarkIconUrl ? <img src={page.benchmarkIconUrl} alt="" className="mt-1 h-11 w-11 shrink-0 rounded-md border border-line object-cover" /> : null}
          <div className="min-w-0">
            <Breadcrumb crumbs={[
              { label: playerName, to: playerHref },
              ...(benchmarksHref === null ? [] : [{ label: "Benchmarks", to: benchmarksHref ?? `${playerHref}/benchmarks` }]),
              { label: title },
            ]} />
            <h1 className="mt-1 break-words text-2xl font-semibold leading-tight md:text-3xl">{title}</h1>
            <p className="mt-1 text-sm text-muted">
              {[page.benchmarkAuthor ? `by ${page.benchmarkAuthor}` : null, `${page.categories.reduce((n, c) => n + c.scenarios.length, 0)} scenarios`].filter(Boolean).join(" · ")}
              {" · "}
              <Link to={`/benchmarks/${page.benchmarkId}`} className="text-cyan hover:underline">Benchmark leaderboard</Link>
            </p>
          </div>
        </div>
      </header>

      {siblings ? (
        <nav aria-label="Difficulty" className="flex flex-wrap gap-2">
          {siblings.variants.map(({ item, difficulty }) => {
            const active = item.benchmarkId === page.benchmarkId;
            const rank = ranksById.get(item.benchmarkId);
            return (
              <Link key={item.benchmarkId} to={benchmarkHref(item.benchmarkId) + (state.sort !== "sheet" ? `?sort=${state.sort}` : "")}
                aria-current={active ? "page" : undefined}
                className={cn("inline-flex min-h-9 items-center gap-1.5 rounded-md border px-3 text-sm capitalize transition-colors",
                  active ? "border-mint/60 bg-mint/10 text-text" : "border-line text-muted hover:text-text")}>
                {difficulty ?? item.benchmarkName}
                {rank?.rankName && rank.rankName.toLowerCase() !== "no rank" ? <span className="text-xs normal-case text-muted-2">· {rank.rankName}</span> : null}
              </Link>
            );
          })}
        </nav>
      ) : null}

      {notice}

      <BenchmarkSummary categories={page.categories} overallRank={page.overallRank} benchmarkProgress={page.benchmarkProgress} />

      <BenchmarkSheet categories={page.categories} sort={state.sort as SheetSort} onSortChange={(sort) => setState({ sort })} scenarioHref={scenarioHref} />

      {page.rankHistory.length >= 2 ? (
        <Section title="Rank over time" aside="From AimMod uploads">
          <RankHistoryChart points={page.rankHistory} ranks={page.ranks} />
          <p className="mt-2 text-xs text-muted-2">Solid line: average scenario rank. Dashed line: overall rank. Runs played before AimMod was installed are not included.</p>
        </Section>
      ) : null}
    </div>
  );
}
