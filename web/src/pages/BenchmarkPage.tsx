import { useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Radar, RadarChart, PolarGrid, PolarAngleAxis, PolarRadiusAxis,
  PieChart, Pie, Cell,
  BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer,
} from "recharts";
import type { BenchmarkCategoryPage, BenchmarkScenarioEntry, BenchmarkSummary, BenchmarkThreshold, GetBenchmarkPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { Card } from "../components/ui/Card";
import { fetchBenchmarkPage, fetchProfile, slugifyScenarioName } from "../lib/api";
import { groupBenchmarks, extractDifficulty } from "../lib/benchmarkGroups";

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
      scenarios: c.scenarios,
    }))
    .filter((c) => c.scenarios.length > 0);
}

// ─── category grouping ────────────────────────────────────────────────────────
//
// "Control Tracking" → { parent: "Tracking", sub: "Control" }
// "Speed"            → { parent: "Speed",    sub: null }

function parseCatName(name: string): { parent: string; sub: string | null } {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) return { parent: parts[0], sub: null };
  return { parent: parts[parts.length - 1], sub: parts.slice(0, -1).join(" ") };
}

type CategoryGroup = {
  parent: string;
  cats: BenchmarkCategoryViewModel[];
  totalRows: number;
};

function groupCategories(categories: BenchmarkCategoryViewModel[]): CategoryGroup[] {
  const groups = new Map<string, BenchmarkCategoryViewModel[]>();
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

// Known Voltaic rank colors. Used as primary lookup when the API returns white.
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

// Palette fallback for unknown rank names where the API also returns no color.
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
//
// Prefer icon URL (distinct per tier family), fall back to color, then index.
// Must be identical in deriveTierColumns and CategoryRows.

function tierKey(t: { iconUrl?: string | null; color?: string | null; rankIndex: number }): string {
  return t.iconUrl?.trim() || t.color?.trim() || `__idx_${t.rankIndex}`;
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

type TierColumn = {
  key: string;
  label: string;
  color: string;
  iconUrl: string;
  thresholds: BenchmarkThreshold[];
};

function fmtScore(n: number): string {
  if (n >= 10_000) return `${Math.round(n / 1000)}k`;
  if (n >= 1_000)  return `${(n / 1000).toFixed(1)}k`;
  return String(Math.round(n));
}

function deriveTierColumns(thresholds: BenchmarkThreshold[]): TierColumn[] {
  if (thresholds.length === 0) return [];
  const byKey = new Map<string, BenchmarkThreshold[]>();
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
  parentLabel,
  parentRowSpan,
}: {
  category: BenchmarkCategoryViewModel;
  userHandle: string;
  tierCols: TierColumn[];
  catColors: Map<string, string>;
  /** Non-null only for the first sub-category in a parent group. */
  parentLabel: string | null;
  /** Total scenario rows across all sub-categories in the parent group. */
  parentRowSpan: number;
}) {
  const accent = catColors.get(category.categoryName) ?? PALETTE[0];
  const count = category.scenarios.length;
  const { sub } = parseCatName(category.categoryName);

  return (
    <>
      {category.scenarios.map((scenario, i) => {
        const scenarioByKey = new Map<string, BenchmarkThreshold[]>();
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

        return (
          <tr
            key={`${category.categoryName}:${scenario.scenarioSlug || scenario.scenarioName}`}
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

            {/* Scenario name + rank badge */}
            <td className="px-3 py-2">
              <div className="flex items-center gap-2 min-w-0">
                {scenario.scenarioRank?.iconUrl ? (
                  <img
                    src={scenario.scenarioRank.iconUrl}
                    alt={scenario.scenarioRank.rankName ?? ""}
                    title={scenario.scenarioRank.rankName ?? undefined}
                    className="shrink-0 h-5 w-5 rounded-md border border-white/10 object-cover"
                  />
                ) : (
                  <span className="shrink-0 text-[10px] font-medium" style={{ color: "rgba(167,194,179,0.6)" }}>
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
              <td rowSpan={count} className="px-3 py-2 text-right align-middle whitespace-nowrap">
                {category.categoryRank > 0 ? (
                  <div className="flex flex-col items-end gap-0.5">
                    <span className="text-[13px] font-medium tabular-nums" style={{ color: accent }}>
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

// ─── chart helpers ────────────────────────────────────────────────────────────

function maxRankIndexFromCols(tierCols: TierColumn[]): number {
  if (tierCols.length === 0) return 1;
  return Math.max(...tierCols.flatMap((col) => col.thresholds.map((t) => t.rankIndex)));
}

function scenarioTierCol(scenario: BenchmarkScenarioEntry, tierCols: TierColumn[]): TierColumn | undefined {
  return tierCols.find((col) =>
    col.thresholds.some((t) => t.rankIndex === scenario.scenarioRank?.rankIndex),
  );
}

function buildRadarData(
  categories: BenchmarkCategoryViewModel[],
  tierCols: TierColumn[],
): { subject: string; value: number; fullMark: number }[] {
  const maxIdx = maxRankIndexFromCols(tierCols);
  return categories.map((cat) => {
    const { sub, parent } = parseCatName(cat.categoryName);
    const ranked = cat.scenarios.filter(
      (s) => hasRank(s.scenarioRank?.rankName) && (s.scenarioRank?.rankIndex ?? 0) > 0,
    );
    const value =
      ranked.length === 0
        ? 0
        : Math.round(
            (ranked.reduce((sum, s) => sum + (s.scenarioRank?.rankIndex ?? 0), 0) /
              ranked.length /
              maxIdx) *
              100,
          );
    return { subject: sub ?? parent, value, fullMark: 100 };
  });
}

function buildRankDistData(
  categories: BenchmarkCategoryViewModel[],
  tierCols: TierColumn[],
): { name: string; value: number; color: string; rankIndex: number }[] {
  const counts = new Map<string, { count: number; color: string; rankIndex: number }>();
  for (const cat of categories) {
    for (const s of cat.scenarios) {
      if (!hasRank(s.scenarioRank?.rankName) || (s.scenarioRank?.rankIndex ?? 0) === 0) continue;
      const col = scenarioTierCol(s, tierCols);
      const label = col?.label ?? shortName(s.scenarioRank?.rankName) ?? "?";
      if (!counts.has(label)) {
        counts.set(label, { count: 0, color: col?.color ?? "#808080", rankIndex: col?.thresholds[0].rankIndex ?? 0 });
      }
      counts.get(label)!.count++;
    }
  }
  return [...counts.entries()]
    .map(([name, { count, color, rankIndex }]) => ({ name, value: count, color, rankIndex }))
    .sort((a, b) => a.rankIndex - b.rankIndex);
}

function buildScenarioPerfData(
  categories: BenchmarkCategoryViewModel[],
  tierCols: TierColumn[],
): { name: string; value: number; color: string }[] {
  const pts: { name: string; value: number; color: string }[] = [];
  for (const cat of categories) {
    for (const s of cat.scenarios) {
      if (!hasRank(s.scenarioRank?.rankName) || (s.scenarioRank?.rankIndex ?? 0) === 0) continue;
      const col = scenarioTierCol(s, tierCols);
      const name = s.scenarioName
        .replace(/^VT\s+/i, "")
        .replace(/\s+S\d+$/i, "")
        .replace(/\s+(Novice|Intermediate|Advanced|Expert)$/i, "")
        .trim();
      pts.push({ name, value: s.scenarioRank?.rankIndex ?? 0, color: col?.color ?? "#808080" });
    }
  }
  return pts;
}

// ─── chart components ─────────────────────────────────────────────────────────

const CHART_TOOLTIP_STYLE = {
  background: "#151e1b",
  border: "1px solid rgba(255,255,255,0.08)",
  borderRadius: 8,
  fontSize: 11,
  color: "#a7c2b3",
};

function BenchmarkCharts({
  categories,
  tierCols,
}: {
  categories: BenchmarkCategoryViewModel[];
  tierCols: TierColumn[];
}) {
  const radarData = useMemo(() => buildRadarData(categories, tierCols), [categories, tierCols]);
  const rankDistData = useMemo(() => buildRankDistData(categories, tierCols), [categories, tierCols]);
  const scenarioPerfData = useMemo(() => buildScenarioPerfData(categories, tierCols), [categories, tierCols]);
  const maxIdx = useMemo(() => maxRankIndexFromCols(tierCols), [tierCols]);
  const totalRanked = rankDistData.reduce((sum, d) => sum + d.value, 0);

  if (tierCols.length === 0 || categories.length === 0) return null;

  return (
    <div className="flex flex-col gap-4">
      {/* Radar chart */}
      <Card className="p-4">
        <p className="mb-1 text-[10px] uppercase tracking-widest text-muted/50">Radar Chart</p>
        <p className="mb-4 text-[11px] text-muted/40">Average rank index per subcategory</p>
        <ResponsiveContainer width="100%" height={300}>
          <RadarChart data={radarData} margin={{ top: 10, right: 40, bottom: 10, left: 40 }}>
            <PolarGrid stroke="rgba(255,255,255,0.07)" />
            <PolarAngleAxis
              dataKey="subject"
              tick={{ fill: "rgba(167,194,179,0.65)", fontSize: 10 }}
            />
            <PolarRadiusAxis domain={[0, 100]} tick={false} axisLine={false} />
            <Radar
              dataKey="value"
              stroke="#38c8c0"
              fill="#38c8c0"
              fillOpacity={0.15}
              strokeWidth={1.5}
            />
            <Tooltip
              contentStyle={CHART_TOOLTIP_STYLE}
              formatter={(v: unknown) => [`${v}%`, "Performance"]}
            />
          </RadarChart>
        </ResponsiveContainer>
      </Card>

      {/* Rank distribution + scenario performance */}
      <div className="grid grid-cols-1 md:grid-cols-[220px_1fr] gap-4">
        {/* Donut */}
        <Card className="p-4 flex flex-col">
          <p className="mb-1 text-[10px] uppercase tracking-widest text-muted/50">Rank Distribution</p>
          <p className="mb-3 text-[11px] text-muted/40">Scenarios per tier</p>
          <div className="relative mx-auto">
            <PieChart width={180} height={180}>
              <Pie
                data={rankDistData}
                cx={90}
                cy={90}
                innerRadius={52}
                outerRadius={76}
                paddingAngle={rankDistData.length > 1 ? 2 : 0}
                dataKey="value"
                strokeWidth={0}
              >
                {rankDistData.map((entry, i) => (
                  <Cell key={i} fill={entry.color} />
                ))}
              </Pie>
              <Tooltip
                contentStyle={CHART_TOOLTIP_STYLE}
                formatter={(v: unknown, name: unknown) => {
                  const count = Number(v);
                  return [`${count} scenario${count !== 1 ? "s" : ""}`, String(name)];
                }}
              />
            </PieChart>
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <span className="text-xl font-medium text-text tabular-nums">{totalRanked}</span>
              <span className="text-[9px] uppercase tracking-widest text-muted/40">ranked</span>
            </div>
          </div>
          <div className="mt-3 flex flex-col gap-1.5">
            {rankDistData.map((d) => (
              <div key={d.name} className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-1.5">
                  <div className="h-2 w-2 shrink-0 rounded-sm" style={{ background: d.color }} />
                  <span className="text-[11px] text-text/70">{d.name}</span>
                </div>
                <span className="text-[11px] text-muted/50 tabular-nums">{d.value}</span>
              </div>
            ))}
          </div>
        </Card>

        {/* Horizontal bar — scenario performance */}
        <Card className="p-4">
          <p className="mb-1 text-[10px] uppercase tracking-widest text-muted/50">Rank Performance</p>
          <p className="mb-4 text-[11px] text-muted/40">Per-scenario rank index</p>
          <ResponsiveContainer width="100%" height={Math.max(200, scenarioPerfData.length * 28)}>
            <BarChart
              data={scenarioPerfData}
              layout="vertical"
              margin={{ top: 0, right: 12, bottom: 0, left: 4 }}
              barSize={14}
            >
              <XAxis type="number" domain={[0, maxIdx]} tick={false} axisLine={false} tickLine={false} />
              <YAxis
                type="category"
                dataKey="name"
                width={130}
                tick={{ fill: "rgba(167,194,179,0.6)", fontSize: 10 }}
                axisLine={false}
                tickLine={false}
              />
              <Tooltip
                contentStyle={CHART_TOOLTIP_STYLE}
                formatter={(v: unknown, name: unknown) => [`Rank index: ${v}`, String(name)]}
                cursor={{ fill: "rgba(255,255,255,0.03)" }}
              />
              <Bar dataKey="value" radius={[0, 3, 3, 0]} background={{ fill: "rgba(255,255,255,0.03)", radius: 3 }}>
                {scenarioPerfData.map((entry, i) => (
                  <Cell key={i} fill={entry.color} fillOpacity={0.85} />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </Card>
      </div>
    </div>
  );
}

// ─── page ─────────────────────────────────────────────────────────────────────

export function BenchmarkPage() {
  const { handle = "", benchmarkId = "" } = useParams();
  const navigate = useNavigate();
  const [page, setPage] = useState<GetBenchmarkPageResponse | null>(null);
  const [profileBenchmarks, setProfileBenchmarks] = useState<BenchmarkSummary[] | null>(null);
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
    // Fetch profile benchmarks for sibling tabs (best-effort)
    void fetchProfile(handle)
      .then((p) => { if (!cancelled) setProfileBenchmarks(p.benchmarks ?? []); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [benchmarkId, handle]);

  const categories = useMemo(() => visibleCategories(page?.categories ?? []), [page]);
  const catColors = useMemo(() => buildCatColorMap(categories), [categories]);
  const categoryGroups = useMemo(() => groupCategories(categories), [categories]);

  // Find sibling benchmarks in the same series (Novice / Intermediate / Advanced…)
  const siblings = useMemo(() => {
    if (!profileBenchmarks || !page) return null;
    const groups = groupBenchmarks(profileBenchmarks);
    const group = groups.find((g) =>
      g.variants.some((v) => v.item.benchmarkId === page.benchmarkId)
    );
    return group && group.variants.length > 1 ? group : null;
  }, [profileBenchmarks, page]);

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
                  { label: siblings ? siblings.base : page.benchmarkName },
                ]}
              />
              <h1 className="mt-2 text-base font-medium text-text leading-tight">
                {siblings ? siblings.base : page.benchmarkName}
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

        {/* Difficulty tabs */}
        {siblings && (
          <div className="mt-4 flex flex-wrap gap-2">
            {siblings.variants.map(({ item, difficulty }) => {
              const isActive = item.benchmarkId === page.benchmarkId;
              const label = difficulty ?? item.benchmarkName;
              const rank = item.overallRank;
              const hasR = hasRank(rank?.rankName);
              return (
                <button
                  key={item.benchmarkId}
                  onClick={() => navigate(`/profiles/${page.userHandle}/benchmarks/${item.benchmarkId}`)}
                  className={[
                    "flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[11px] font-medium uppercase tracking-[0.06em] transition-colors",
                    isActive
                      ? "border-cyan/40 bg-cyan/10 text-cyan"
                      : "border-line text-muted/60 hover:border-line/80 hover:text-text",
                  ].join(" ")}
                >
                  {rank?.iconUrl && (
                    <img src={rank.iconUrl} alt="" className="h-3.5 w-3.5 rounded-sm border border-white/10 object-cover" />
                  )}
                  {label}
                  {hasR && rank?.rankName && (
                    <span className={`text-[9px] ${isActive ? "text-cyan/70" : "text-muted/40"}`}>
                      · {rank.rankName}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        )}

      </Card>

      {/* table */}
      <Card className="p-0 overflow-hidden">
        <div className="overflow-x-auto hub-scroll">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-line bg-white/2">
                {/* parent group column */}
                <th className="w-5 py-2 font-normal" />
                {/* sub-category column */}
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
              {categoryGroups.map((group) =>
                group.cats.map((category, catIdx) => (
                  <CategoryRows
                    key={category.categoryName}
                    category={category}
                    userHandle={page.userHandle}
                    tierCols={tierCols}
                    catColors={catColors}
                    parentLabel={catIdx === 0 ? group.parent : null}
                    parentRowSpan={group.totalRows}
                  />
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>

      {/* charts */}
      <BenchmarkCharts categories={categories} tierCols={tierCols} />
    </PageStack>
  );
}
