import { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, useParams } from "react-router-dom";
import type { BenchmarkCategoryPage, BenchmarkScenarioEntry, BenchmarkThreshold, GetBenchmarkPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { Card } from "../components/ui/Card";
import { fetchBenchmarkPage, slugifyScenarioName } from "../lib/api";

// ─── helpers ──────────────────────────────────────────────────────────────────

function hasRank(rankName?: string | null) {
  const n = rankName?.trim().toLowerCase();
  return Boolean(n && n !== "no rank");
}

type BenchmarkCategoryViewModel = {
  categoryName: string;
  categoryRank: number;
  scenarios: BenchmarkScenarioEntry[];
};

function visibleCategories(categories: BenchmarkCategoryPage[]): BenchmarkCategoryViewModel[] {
  return categories
    .map((c) => ({
      categoryName: c.categoryName,
      categoryRank: c.categoryRank,
      scenarios: c.scenarios.filter(
        (s) => s.scenarioRank && hasRank(s.scenarioRank.rankName),
      ),
    }))
    .filter((c) => c.scenarios.length > 0);
}

function tierColor(color: string | undefined): string {
  if (color?.trim()) return color.trim();
  return "#a7c2b3";
}

// "VT Bronze Novice S5" → "Bronze"
function shortName(name: string | null | undefined): string {
  if (!name) return "";
  return name
    .replace(/^VT\s+/i, "")
    .replace(/\s+S\d+$/i, "")
    .replace(/\s+(Novice|Intermediate|Advanced|Expert)$/i, "")
    .trim();
}

// ─── tier column derivation ───────────────────────────────────────────────────
//
// Group thresholds by color so benchmarks with sub-tiers (Bronze I/II/III)
// collapse into one column per tier family. The column label and color come
// from the LOWEST sub-tier in the group. The bar for each column shows the
// HIGHEST sub-tier reached (or progress toward the lowest if none reached).

type TierColumn = {
  key: string;               // unique key (color or rankIndex fallback)
  label: string;             // e.g. "Bronze"
  color: string;             // CSS color
  iconUrl: string;           // highest sub-tier icon
  thresholds: BenchmarkThreshold[]; // sorted ascending by score
};

function fmtScore(n: number): string {
  if (n >= 100_000) return `${Math.round(n / 1000)}k`;
  if (n >= 10_000)  return `${Math.round(n / 1000)}k`;
  if (n >= 1_000)   return `${(n / 1000).toFixed(1)}k`;
  return String(Math.round(n));
}

// Group thresholds by color so Bronze I/II/III collapse into one "Bronze" column.
function deriveTierColumns(thresholds: BenchmarkThreshold[]): TierColumn[] {
  if (thresholds.length === 0) return [];
  const byColor = new Map<string, BenchmarkThreshold[]>();
  for (const t of thresholds) {
    const key = t.color?.trim() || `__idx_${t.rankIndex}`;
    if (!byColor.has(key)) byColor.set(key, []);
    byColor.get(key)!.push(t);
  }
  const columns: TierColumn[] = [...byColor.entries()].map(([key, ts]) => {
    const sorted = [...ts].sort((a, b) => a.rankIndex - b.rankIndex);
    return {
      key,
      label: shortName(sorted[0].rankName),
      color: tierColor(sorted[0].color),
      iconUrl: sorted[sorted.length - 1].iconUrl ?? "",
      thresholds: sorted,
    };
  });
  columns.sort((a, b) => a.thresholds[0].rankIndex - b.thresholds[0].rankIndex);
  return columns;
}

// ─── tier bar cell ────────────────────────────────────────────────────────────
//
// Segmented bar — one segment per sub-tier (e.g. Bronze I / II / III).
// Segments are equal-width, separated by a hairline divider.
// A segment is fully filled when its threshold is met; partially filled while
// in progress; empty when not yet reached.

function TierBar({ col, score, startScore = 0 }: { col: TierColumn; score: number; startScore?: number }) {
  const ts = col.thresholds; // sorted ascending
  const color = col.color;

  return (
    <div className="flex h-5 min-w-16 overflow-hidden rounded-sm bg-black/20">
      {ts.map((t, i) => {
        // For the first segment, use startScore (the previous column's max) as the
        // baseline so players don't show "in progress" on tiers they haven't reached.
        const prevScore = i === 0 ? startScore : ts[i - 1].score;
        const met = score >= t.score;
        const inProgress = !met && score > prevScore;
        const pct = inProgress
          ? Math.min(100, ((score - prevScore) / ((t.score - prevScore) || 1)) * 100)
          : met ? 100 : 0;
        const textColor = met ? "rgba(0,0,0,0.7)" : `${color}bb`;

        return (
          <div
            key={t.rankIndex}
            className="relative flex-1 overflow-hidden"
            style={{ borderLeft: i > 0 ? "1px solid rgba(0,0,0,0.35)" : undefined }}
          >
            <div
              className="absolute inset-y-0 left-0"
              style={{ width: `${pct}%`, background: met ? color : `${color}55` }}
            />
            <div
              className="absolute inset-0 flex items-center justify-center text-[9px] font-medium tabular-nums z-10 leading-none"
              style={{ color: textColor, mixBlendMode: met ? "multiply" : "normal" }}
            >
              {fmtScore(t.score)}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ─── category color palette ───────────────────────────────────────────────────

const PALETTE = ["#b8ffe1", "#ffd956", "#c2a9ff", "#79c997", "#ff8787", "#50c8ff"];

function buildCatColorMap(categories: BenchmarkCategoryViewModel[]): Map<string, string> {
  const map = new Map<string, string>();
  categories.forEach((c, i) => map.set(c.categoryName, PALETTE[i % PALETTE.length]));
  return map;
}

// ─── category rows ────────────────────────────────────────────────────────────

function CategoryRows({
  category,
  userHandle,
  tierCols,
  catColors,
}: {
  category: BenchmarkCategoryViewModel;
  userHandle: string;
  tierCols: TierColumn[];
  catColors: Map<string, string>;
}) {
  const accent = catColors.get(category.categoryName) ?? PALETTE[0];
  const count = category.scenarios.length;

  return (
    <>
      {category.scenarios.map((scenario, i) => {
        // Build a map from color-key → thresholds for THIS scenario
        const scenarioByColor = new Map<string, BenchmarkThreshold[]>();
        for (const t of scenario.thresholds) {
          const key = t.color?.trim() || `__idx_${t.rankIndex}`;
          if (!scenarioByColor.has(key)) scenarioByColor.set(key, []);
          scenarioByColor.get(key)!.push(t);
        }
        const colThresholds = tierCols.map((col) => {
          const ts = scenarioByColor.get(col.key);
          return ts ? [...ts].sort((a, b) => a.rankIndex - b.rankIndex) : null;
        });
        // startScore for each column = max threshold of the previous column in this scenario.
        // If the previous column has no data, use Infinity so the first segment never
        // shows "in progress" for tiers the player hasn't earned their way into.
        const colStartScores = tierCols.map((_, ci) => {
          if (ci === 0) return 0;
          const prevTs = colThresholds[ci - 1];
          return prevTs ? prevTs[prevTs.length - 1].score : Infinity;
        });

        // % of gold (last tier's highest threshold)
        const maxTierThreshold = tierCols.length > 0
          ? colThresholds[tierCols.length - 1]
          : null;
        const maxScore = maxTierThreshold?.[maxTierThreshold.length - 1]?.score ?? 0;
        const pctOfMax = maxScore > 0 ? Math.min(100, Math.round((scenario.score / maxScore) * 100)) : null;

        return (
          <tr
            key={`${category.categoryName}:${scenario.scenarioSlug || scenario.scenarioName}`}
            className="border-b border-white/4 last:border-0 hover:bg-white/[0.018] transition-colors"
          >
            {/* Category label — rowspan */}
            {i === 0 && (
              <td
                rowSpan={count}
                className="py-0 align-middle text-center"
                style={{ borderLeft: `2px solid ${accent}55`, width: 28, minWidth: 28 }}
              >
                <span
                  className="text-[8px] uppercase font-medium whitespace-nowrap inline-block"
                  style={{
                    color: `${accent}cc`,
                    writingMode: "vertical-rl",
                    transform: "rotate(180deg)",
                    letterSpacing: "0.14em",
                  }}
                >
                  {category.categoryName}
                </span>
              </td>
            )}

            {/* Scenario name + rank badge */}
            <td className="px-3 py-2">
              <div className="flex items-center gap-2 min-w-0">
                {/* rank icon */}
                {scenario.scenarioRank?.iconUrl ? (
                  <img
                    src={scenario.scenarioRank.iconUrl}
                    alt={scenario.scenarioRank.rankName ?? ""}
                    title={scenario.scenarioRank.rankName ?? undefined}
                    className="shrink-0 h-5 w-5 rounded-md border border-white/10 object-cover"
                  />
                ) : (
                  <span
                    className="shrink-0 text-[10px] font-medium"
                    style={{ color: "rgba(167,194,179,0.6)" }}
                  >
                    {shortName(scenario.scenarioRank?.rankName) || "—"}
                  </span>
                )}
                <Link
                  to={`/profiles/${userHandle}/scenarios/${scenario.scenarioSlug || slugifyScenarioName(scenario.scenarioName)}`}
                  className="text-[11.5px] font-medium text-text/85 hover:text-cyan transition-colors truncate"
                >
                  {scenario.scenarioName}
                </Link>
                {scenario.leaderboardRank > 0 && (
                  <span className="shrink-0 text-[9px] text-muted/50 tabular-nums">
                    #{scenario.leaderboardRank.toLocaleString()}
                  </span>
                )}
              </div>
            </td>

            {/* Score + % of max */}
            <td className="px-3 py-2 text-right whitespace-nowrap">
              <div className="text-[12px] font-medium text-text tabular-nums">
                {Math.round(scenario.score).toLocaleString()}
              </div>
              {pctOfMax !== null && (
                <div className="text-[9px] text-muted/50 tabular-nums">{pctOfMax}%</div>
              )}
            </td>

            {/* One cell per tier column */}
            {tierCols.map((col, ci) => {
              const ts = colThresholds[ci];
              if (!ts || ts.length === 0) {
                return <td key={col.key} className="px-1.5 py-2" />;
              }
              return (
                <td key={col.key} className="px-1.5 py-2">
                  <TierBar col={{ ...col, thresholds: ts }} score={scenario.score} startScore={colStartScores[ci]} />
                </td>
              );
            })}

            {/* Energy — rowspan */}
            {i === 0 && (
              <td
                rowSpan={count}
                className="px-3 py-2 text-right align-middle whitespace-nowrap"
              >
                {category.categoryRank > 0 ? (
                  <div className="flex flex-col items-end gap-0.5">
                    <span
                      className="text-[13px] font-medium tabular-nums"
                      style={{ color: accent }}
                    >
                      {category.categoryRank.toLocaleString()}
                    </span>
                    <span className="text-[8px] uppercase tracking-widest text-muted/40">nrg</span>
                  </div>
                ) : null}
              </td>
            )}
          </tr>
        );
      })}
    </>
  );
}

// ─── page ─────────────────────────────────────────────────────────────────────

export function BenchmarkPage() {
  const { handle = "", benchmarkId = "" } = useParams();
  const [page, setPage] = useState<GetBenchmarkPageResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setError(null);
    const parsedId = Number(benchmarkId);
    if (!Number.isFinite(parsedId) || parsedId <= 0) {
      setError("This benchmark page is not available.");
      return () => { cancelled = true; };
    }
    void fetchBenchmarkPage(handle, parsedId)
      .then((next) => { if (!cancelled) setPage(next); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this benchmark."); });
    return () => { cancelled = true; };
  }, [benchmarkId, handle]);

  const categories = useMemo(() => visibleCategories(page?.categories ?? []), [page]);

  const catColors = useMemo(() => buildCatColorMap(categories), [categories]);

  // Derive tier columns from the first scenario that has thresholds
  const tierCols = useMemo(() => {
    for (const cat of categories) {
      for (const sc of cat.scenarios) {
        if (sc.thresholds.length > 0) return deriveTierColumns(sc.thresholds);
      }
    }
    return [];
  }, [categories]);

  if (error) {
    return (
      <PageStack>
        <Card className="p-4.5">
          <EmptyState title="Benchmark not found" body={error} />
        </Card>
      </PageStack>
    );
  }

  if (!page) {
    return (
      <PageStack>
        <Card className="p-4.5">
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="mb-3 h-8 w-64" />
          <Skeleton className="h-4 w-48" />
        </Card>
        <Card className="p-0 overflow-hidden">
          <div className="flex flex-col gap-px">
            {Array.from({ length: 12 }).map((_, i) => (
              <Skeleton key={i} className="h-9 rounded-none" />
            ))}
          </div>
        </Card>
      </PageStack>
    );
  }

  const displayName = page.userDisplayName || page.userHandle;

  return (
    <PageStack>
      <Helmet>
        <title>{page.benchmarkName} · {displayName} · AimMod Hub</title>
      </Helmet>

      {/* header */}
      <Card className="p-3.5 md:p-4.5">
        <div className="flex items-center justify-between gap-4 flex-wrap">
          <div className="flex items-start gap-3 min-w-0">
            {page.benchmarkIconUrl && (
              <img
                src={page.benchmarkIconUrl}
                alt=""
                className="mt-0.5 h-10 w-10 shrink-0 rounded-[10px] border border-white/10 object-cover"
              />
            )}
            <div className="min-w-0">
              <Breadcrumb
                crumbs={[
                  { label: displayName, to: `/profiles/${page.userHandle}` },
                  { label: page.benchmarkName },
                ]}
              />
              <h1 className="mt-2 text-base font-medium text-text leading-tight">
                {page.benchmarkName}
              </h1>
              <p className="mt-0.5 text-[11px] text-muted/70">
                {page.benchmarkAuthor ? `by ${page.benchmarkAuthor}` : "Community benchmark"}
                {page.benchmarkType ? ` · ${page.benchmarkType}` : ""}
              </p>
            </div>
          </div>
          {page.overallRank && hasRank(page.overallRank.rankName) && (
            <div className="flex items-center gap-2 rounded-xl border border-white/10 bg-white/3 px-3 py-2 shrink-0">
              {page.overallRank.iconUrl && (
                <img
                  src={page.overallRank.iconUrl}
                  alt=""
                  className="h-7 w-7 rounded-lg border border-white/10 object-cover"
                />
              )}
              <div>
                <div className="text-[12px] font-medium text-text">{page.overallRank.rankName}</div>
                <div className="text-[9px] text-muted/60 uppercase tracking-widest">Overall</div>
              </div>
            </div>
          )}
        </div>
      </Card>

      {/* table */}
      <Card className="p-0 overflow-hidden">
        <div className="overflow-x-auto hub-scroll">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-line bg-white/2">
                <th className="py-2 w-7 font-normal" />
                <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal">
                  Scenario
                </th>
                <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-right">
                  Score
                </th>
                {tierCols.map((col) => (
                  <th
                    key={col.key}
                    className="px-1.5 py-1.5 font-normal text-center"
                    style={{ color: `${col.color}cc` }}
                  >
                    <div className="flex flex-col items-center gap-0.5">
                      {col.iconUrl && (
                        <img
                          src={col.iconUrl}
                          alt=""
                          className="h-4 w-4 rounded-sm border border-white/10 object-cover"
                        />
                      )}
                      <span className="text-[8px] uppercase tracking-widest leading-none">{col.label}</span>
                    </div>
                  </th>
                ))}
                <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-right">
                  Nrg
                </th>
              </tr>
            </thead>
            <tbody>
              {categories.map((category) => (
                <CategoryRows
                  key={category.categoryName}
                  category={category}
                  userHandle={page.userHandle}
                  tierCols={tierCols}
                  catColors={catColors}
                />
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </PageStack>
  );
}
