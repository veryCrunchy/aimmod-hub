import { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, useParams } from "react-router-dom";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import {
  fetchExternalBenchmarkPage,
  slugifyScenarioName,
  type ExternalBenchmarkPageResponse,
  type ExternalCategoryPage,
  type ExternalScenarioPage,
  type ExternalThreshold,
} from "../lib/api";

// ─── helpers ──────────────────────────────────────────────────────────────────

function hasRank(rankName?: string | null) {
  const n = rankName?.trim().toLowerCase();
  return Boolean(n && n !== "no rank");
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

// ─── category grouping ────────────────────────────────────────────────────────

function parseCatName(name: string): { parent: string; sub: string | null } {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return { parent: parts[0], sub: null };
  return { parent: parts[parts.length - 1], sub: parts.slice(0, -1).join(" ") };
}

type CategoryViewModel = {
  categoryName: string;
  categoryRank: number;
  scenarios: ExternalScenarioPage[];
};

type CategoryGroup = {
  parent: string;
  cats: CategoryViewModel[];
  totalRows: number;
};

function groupCategories(categories: CategoryViewModel[]): CategoryGroup[] {
  const groups = new Map<string, CategoryViewModel[]>();
  for (const cat of categories) {
    const { parent } = parseCatName(cat.categoryName);
    if (!groups.has(parent)) groups.set(parent, []);
    groups.get(parent)!.push(cat);
  }
  return [...groups.entries()].map(([parent, cats]) => ({
    parent,
    cats,
    totalRows: cats.reduce((s, c) => s + c.scenarios.length, 0),
  }));
}

// ─── tier colors ──────────────────────────────────────────────────────────────

const RANK_NAME_COLORS: Record<string, string> = {
  "iron":        "#6b8c8c",
  "bronze":      "#b07840",
  "silver":      "#9098a0",
  "gold":        "#c0a030",
  "platinum":    "#6eaec0",
  "diamond":     "#38c8c0",
  "jade":        "#38c868",
  "master":      "#a840c8",
  "grandmaster": "#e04040",
  "nova":        "#ff8820",
};

const RANK_PALETTE = ["#c8956c", "#b0b8b0", "#e8c84a", "#7ec8e3", "#c084fc", "#60e0a0", "#f87171"];

function tierColor(apiColor: string | undefined, rankName?: string, paletteIdx?: number): string {
  if (rankName) {
    const known = RANK_NAME_COLORS[rankName.trim().toLowerCase()];
    if (known) return known;
  }
  const c = apiColor?.trim();
  if (c && c !== "#ffffff" && c !== "#fff") return c;
  return RANK_PALETTE[(paletteIdx ?? 0) % RANK_PALETTE.length];
}

// ─── tier column key ──────────────────────────────────────────────────────────

function tierKey(t: { iconUrl?: string | null; color?: string | null; rankIndex: number }): string {
  return t.iconUrl?.trim() || t.color?.trim() || `__idx_${t.rankIndex}`;
}

// ─── tier column derivation ───────────────────────────────────────────────────

type TierColumn = {
  key: string;
  label: string;
  color: string;
  iconUrl: string;
  thresholds: ExternalThreshold[];
};

function fmtScore(n: number): string {
  if (n >= 10_000) return `${Math.round(n / 1000)}k`;
  if (n >= 1_000)  return `${(n / 1000).toFixed(1)}k`;
  return String(Math.round(n));
}

function deriveTierColumns(thresholds: ExternalThreshold[]): TierColumn[] {
  if (thresholds.length === 0) return [];
  const byKey = new Map<string, ExternalThreshold[]>();
  for (const t of thresholds) {
    const key = tierKey(t);
    if (!byKey.has(key)) byKey.set(key, []);
    byKey.get(key)!.push(t);
  }
  const columns: TierColumn[] = [...byKey.entries()].map(([key, ts]) => {
    const sorted = [...ts].sort((a, b) => a.rankIndex - b.rankIndex);
    return {
      key,
      label: shortName(sorted[0].rankName),
      color: "#__pending__",
      iconUrl: sorted[sorted.length - 1].iconUrl ?? "",
      thresholds: sorted,
    };
  });
  columns.sort((a, b) => a.thresholds[0].rankIndex - b.thresholds[0].rankIndex);
  columns.forEach((col, i) => {
    col.color = tierColor(col.thresholds[0].color, col.thresholds[0].rankName, i);
  });
  return columns;
}

// ─── tier bar ─────────────────────────────────────────────────────────────────

function TierBar({ col, score, startScore = 0 }: { col: TierColumn; score: number; startScore?: number }) {
  const ts = col.thresholds;
  const color = col.color;

  return (
    <div className="flex h-5 min-w-16 overflow-hidden rounded-sm bg-black/20">
      {ts.map((t, i) => {
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

function buildCatColorMap(categories: CategoryViewModel[]): Map<string, string> {
  const map = new Map<string, string>();
  categories.forEach((c, i) => map.set(c.categoryName, PALETTE[i % PALETTE.length]));
  return map;
}

// ─── category rows ────────────────────────────────────────────────────────────

function CategoryRows({
  category,
  tierCols,
  aimmodHandle,
  catColors,
  parentLabel,
  parentRowSpan,
}: {
  category: CategoryViewModel;
  tierCols: TierColumn[];
  aimmodHandle?: string;
  catColors: Map<string, string>;
  parentLabel: string | null;
  parentRowSpan: number;
}) {
  const accent = catColors.get(category.categoryName) ?? PALETTE[0];
  const count = category.scenarios.length;
  const { sub } = parseCatName(category.categoryName);

  return (
    <>
      {category.scenarios.map((scenario, i) => {
        const scenarioByKey = new Map<string, ExternalThreshold[]>();
        for (const t of scenario.thresholds) {
          const key = tierKey(t);
          if (!scenarioByKey.has(key)) scenarioByKey.set(key, []);
          scenarioByKey.get(key)!.push(t);
        }
        const colThresholds = tierCols.map((col) => {
          const ts = scenarioByKey.get(col.key);
          return ts ? [...ts].sort((a, b) => a.rankIndex - b.rankIndex) : null;
        });
        const colStartScores = tierCols.map((_, ci) => {
          if (ci === 0) return 0;
          const prevTs = colThresholds[ci - 1];
          return prevTs ? prevTs[prevTs.length - 1].score : Infinity;
        });
        const maxTierThreshold = tierCols.length > 0 ? colThresholds[tierCols.length - 1] : null;
        const maxScore = maxTierThreshold?.[maxTierThreshold.length - 1]?.score ?? 0;
        const pctOfMax = maxScore > 0 ? Math.min(100, Math.round((scenario.score / maxScore) * 100)) : null;
        const scenarioSlug = slugifyScenarioName(scenario.scenarioName);

        return (
          <tr
            key={`${category.categoryName}:${scenario.scenarioName}`}
            className="border-b border-white/4 last:border-0 hover:bg-white/[0.018] transition-colors"
          >
            {/* Parent group label — first row of first sub-cat only */}
            {i === 0 && parentLabel !== null && (
              <td
                rowSpan={parentRowSpan}
                className="py-0 align-middle text-center"
                style={{ width: 20, minWidth: 20 }}
              >
                <span
                  className="text-[7px] uppercase font-medium whitespace-nowrap inline-block"
                  style={{
                    color: "rgba(167,194,179,0.35)",
                    writingMode: "vertical-rl",
                    transform: "rotate(180deg)",
                    letterSpacing: "0.14em",
                  }}
                >
                  {parentLabel}
                </span>
              </td>
            )}

            {/* Sub-category label — first row of this sub-cat */}
            {i === 0 && (
              <td
                rowSpan={count}
                className="py-0 align-middle text-center"
                style={{ borderLeft: `2px solid ${accent}55`, width: 24, minWidth: 24 }}
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
                  {sub ?? category.categoryName}
                </span>
              </td>
            )}

            <td className="px-3 py-2">
              <div className="flex items-center gap-2 min-w-0">
                {scenario.rankIconUrl ? (
                  <img
                    src={scenario.rankIconUrl}
                    alt={scenario.rankName ?? ""}
                    title={scenario.rankName ?? undefined}
                    className="shrink-0 h-5 w-5 rounded-md border border-white/10 object-cover"
                  />
                ) : (
                  <span className="shrink-0 text-[10px] font-medium" style={{ color: "rgba(167,194,179,0.45)" }}>
                    {scenario.rankIndex > 0 ? shortName(scenario.rankName) : "—"}
                  </span>
                )}
                {aimmodHandle ? (
                  <Link
                    to={`/profiles/${aimmodHandle}/scenarios/${scenarioSlug}`}
                    className="text-[11.5px] font-medium text-text/85 hover:text-cyan transition-colors truncate"
                  >
                    {scenario.scenarioName}
                  </Link>
                ) : (
                  <span className="text-[11.5px] font-medium text-text/85 truncate">
                    {scenario.scenarioName}
                  </span>
                )}
                {scenario.leaderboardRank > 0 && (
                  <span className="shrink-0 text-[9px] text-muted/50 tabular-nums">
                    #{scenario.leaderboardRank.toLocaleString()}
                  </span>
                )}
              </div>
            </td>

            <td className="px-3 py-2 text-right whitespace-nowrap">
              <div className="text-[12px] font-medium text-text tabular-nums">
                {scenario.score > 0 ? Math.round(scenario.score).toLocaleString() : "—"}
              </div>
              {pctOfMax !== null && scenario.score > 0 && (
                <div className="text-[9px] text-muted/50 tabular-nums">{pctOfMax}%</div>
              )}
            </td>

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

            {i === 0 && (
              <td rowSpan={count} className="px-3 py-2 text-right align-middle whitespace-nowrap">
                {category.categoryRank > 0 && (
                  <div className="flex flex-col items-end gap-0.5">
                    <span className="text-[13px] font-medium tabular-nums" style={{ color: accent }}>
                      {category.categoryRank.toLocaleString()}
                    </span>
                    <span className="text-[8px] uppercase tracking-widest text-muted/40">nrg</span>
                  </div>
                )}
              </td>
            )}
          </tr>
        );
      })}
    </>
  );
}

// ─── view model ───────────────────────────────────────────────────────────────

function buildCategoryViewModels(
  categories: ExternalCategoryPage[],
  showUnranked: boolean,
): CategoryViewModel[] {
  return categories
    .map((c) => ({
      categoryName: c.categoryName,
      categoryRank: c.categoryRank,
      scenarios: showUnranked
        ? c.scenarios
        : c.scenarios.filter((s) => s.rankIndex > 0 && hasRank(s.rankName)),
    }))
    .filter((c) => c.scenarios.length > 0);
}

// ─── page ─────────────────────────────────────────────────────────────────────

export function ExternalBenchmarkPage() {
  const { steamId = "", benchmarkId = "" } = useParams();
  const [page, setPage] = useState<ExternalBenchmarkPageResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showUnranked, setShowUnranked] = useState(false);

  useEffect(() => {
    if (!steamId || !benchmarkId) return;
    const parsedId = Number(benchmarkId);
    if (!Number.isFinite(parsedId) || parsedId <= 0) {
      setError("Invalid benchmark ID.");
      return;
    }
    let cancelled = false;
    setPage(null);
    setError(null);
    void fetchExternalBenchmarkPage(steamId, parsedId)
      .then((next) => { if (!cancelled) setPage(next); })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this benchmark.");
      });
    return () => { cancelled = true; };
  }, [steamId, benchmarkId]);

  const categories = useMemo(
    () => buildCategoryViewModels(page?.categories ?? [], showUnranked),
    [page, showUnranked],
  );

  const catColors = useMemo(() => buildCatColorMap(categories), [categories]);
  const categoryGroups = useMemo(() => groupCategories(categories), [categories]);

  const tierCols = useMemo(() => {
    for (const cat of categories) {
      for (const sc of cat.scenarios) {
        if (sc.thresholds.length > 0) return deriveTierColumns(sc.thresholds);
      }
    }
    return [];
  }, [categories]);

  const displayName = page?.kovaaksUsername || steamId;

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

  return (
    <PageStack>
      <Helmet>
        <title>{page.benchmarkName} · {displayName} · AimMod Hub</title>
      </Helmet>

      <Card className="p-3.5 md:p-4.5">
        <div className="flex items-start justify-between gap-4 flex-wrap">
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
                  { label: displayName, to: `/u/${encodeURIComponent(steamId)}` },
                  { label: page.benchmarkName },
                ]}
              />
              <h1 className="mt-2 text-base font-medium text-text leading-tight">
                {page.benchmarkName}
              </h1>
              <div className="mt-1 flex flex-wrap items-center gap-2">
                {page.isAimmodUser && page.aimmodHandle ? (
                  <Link
                    to={`/profiles/${page.aimmodHandle}`}
                    className="text-[11px] text-cyan hover:underline underline-offset-3"
                  >
                    @{page.aimmodHandle} · AimMod profile →
                  </Link>
                ) : (
                  <span className="text-[11px] text-muted/60">KovaaK's only user</span>
                )}
              </div>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-3">
            {page.overallRankName && hasRank(page.overallRankName) && (
              <div className="flex items-center gap-2 rounded-xl border border-white/10 bg-white/3 px-3 py-2">
                {page.overallRankIcon && (
                  <img src={page.overallRankIcon} alt="" className="h-7 w-7 rounded-lg border border-white/10 object-cover" />
                )}
                <div>
                  <div className="text-[12px] font-medium" style={{ color: page.overallRankColor || "#a7c2b3" }}>
                    {page.overallRankName}
                  </div>
                  <div className="text-[9px] text-muted/60 uppercase tracking-widest">Overall</div>
                </div>
              </div>
            )}
          </div>
        </div>

        <div className="mt-3 flex items-center gap-2">
          <button
            onClick={() => setShowUnranked((v) => !v)}
            className={[
              "rounded-full border px-3 py-1 text-[10px] uppercase tracking-[0.08em] transition-colors",
              showUnranked
                ? "border-gold/30 bg-gold/8 text-gold"
                : "border-line text-muted hover:border-line/60 hover:text-text",
            ].join(" ")}
          >
            {showUnranked ? "Ranked only" : "Show all scenarios"}
          </button>
        </div>
      </Card>

      <Card className="p-0 overflow-hidden">
        {categories.length > 0 ? (
          <div className="overflow-x-auto hub-scroll">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-line bg-white/2">
                  <th className="w-5 py-2 font-normal" />
                  <th className="w-6 py-2 font-normal" />
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
                          <img src={col.iconUrl} alt="" className="h-4 w-4 rounded-sm border border-white/10 object-cover" />
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
                {categoryGroups.map((group) =>
                  group.cats.map((category, catIdx) => (
                    <CategoryRows
                      key={category.categoryName}
                      category={category}
                      tierCols={tierCols}
                      aimmodHandle={page.isAimmodUser ? page.aimmodHandle : undefined}
                      catColors={catColors}
                      parentLabel={catIdx === 0 ? group.parent : null}
                      parentRowSpan={group.totalRows}
                    />
                  ))
                )}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="p-4.5">
            <EmptyState
              title="No ranked scenarios"
              body={`${displayName} hasn't reached a rank on any scenario in this benchmark yet.`}
            />
          </div>
        )}
      </Card>
    </PageStack>
  );
}
