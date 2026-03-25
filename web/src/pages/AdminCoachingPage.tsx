import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { Card } from "../components/ui/Card";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useAuth } from "../lib/AuthContext";
import {
  fetchCoachingAnalytics,
  formatRelativeTime,
  type CoachingAnalytics,
  type CoachingQueryRow,
} from "../lib/api";

const DAY_OPTIONS = [7, 14, 30, 60, 90] as const;

function pct(value: number) {
  return value.toFixed(1) + "%";
}

function HitMissBar({ hitRatePct }: { hitRatePct: number }) {
  const miss = Math.max(0, 100 - hitRatePct);
  return (
    <div className="flex h-3 w-full overflow-hidden rounded-full bg-white/5">
      <div
        className="h-full bg-mint/60 transition-all duration-500"
        style={{ width: `${hitRatePct.toFixed(1)}%` }}
      />
      <div
        className="h-full bg-amber-500/40 transition-all duration-500"
        style={{ width: `${miss.toFixed(1)}%` }}
      />
    </div>
  );
}

function TermPill({ term, count, max }: { term: string; count: number; max: number }) {
  const intensity = Math.max(0.15, count / max);
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full border border-amber-500/20 bg-amber-500/8 px-2.5 py-1 text-[11px]"
      style={{ opacity: 0.5 + intensity * 0.5 }}
    >
      <span className="font-medium text-amber-300">{term}</span>
      <span className="tabular-nums text-muted">{count}</span>
    </span>
  );
}

function EntryPill({ entryId, count, max }: { entryId: string; count: number; max: number }) {
  const intensity = Math.max(0.15, count / max);
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full border border-mint/20 bg-mint/8 px-2.5 py-1 text-[11px]"
      style={{ opacity: 0.5 + intensity * 0.5 }}
    >
      <span className="font-medium text-mint">{entryId}</span>
      <span className="tabular-nums text-muted">{count}×</span>
    </span>
  );
}

function QueryRow({ row }: { row: CoachingQueryRow }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div
      className={`border-b border-line/50 px-4 py-3 last:border-0 ${row.isMiss ? "bg-amber-500/3" : ""}`}
    >
      <div className="flex min-w-0 items-start gap-3">
        {/* Hit/miss indicator */}
        <div className="mt-0.5 shrink-0">
          {row.isMiss ? (
            <span className="inline-block size-2 rounded-full bg-amber-400/70" title="Miss — no KB entries returned" />
          ) : (
            <span className="inline-block size-2 rounded-full bg-mint/70" title="Hit — KB entries returned" />
          )}
        </div>

        <div className="min-w-0 flex-1">
          {/* Question */}
          <div className="text-[13px] leading-snug text-white/85">
            {row.question || <span className="text-muted italic">no question</span>}
          </div>

          {/* Meta row */}
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[11px] text-muted">
            {row.scenarioName && (
              <span className="text-cyan/80">{row.scenarioName}</span>
            )}
            {row.scenarioType && (
              <span>{row.scenarioType}</span>
            )}
            <span>{formatRelativeTime(row.createdAt)}</span>
            {row.isMiss ? (
              <span className="text-amber-400/70">0 results</span>
            ) : (
              <span className="text-mint/70">{row.itemsReturned} result{row.itemsReturned !== 1 ? "s" : ""}</span>
            )}
            {row.questionTermsMatched.length > 0 && (
              <span className="text-muted-2">
                terms: {row.questionTermsMatched.join(", ")}
              </span>
            )}
          </div>

          {/* Matched entries */}
          {!row.isMiss && row.matchedEntryIds.length > 0 && (
            <div className="mt-1.5 flex flex-wrap gap-1">
              {row.matchedEntryIds.map((id) => (
                <span
                  key={id}
                  className="rounded-full border border-mint/15 bg-mint/6 px-2 py-0.5 text-[10px] text-mint/70"
                >
                  {id}
                </span>
              ))}
            </div>
          )}

          {/* Signal keys / context tags on expand */}
          {(row.signalKeys.length > 0 || row.contextTags.length > 0) && (
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="mt-1 text-[11px] text-muted hover:text-white/70"
            >
              {expanded ? "hide context ↑" : "show context ↓"}
            </button>
          )}
          {expanded && (
            <div className="mt-1.5 flex flex-wrap gap-1">
              {row.signalKeys.map((k) => (
                <span key={k} className="rounded-full border border-cyan/15 bg-cyan/6 px-2 py-0.5 text-[10px] text-cyan/70">
                  {k}
                </span>
              ))}
              {row.contextTags.map((t) => (
                <span key={t} className="rounded-full border border-violet/15 bg-violet/6 px-2 py-0.5 text-[10px] text-violet/60">
                  {t}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export function AdminCoachingPage() {
  const auth = useAuth();
  const isAdmin = Boolean(auth.user?.isAdmin ?? auth.isAdmin);
  const [analytics, setAnalytics] = useState<CoachingAnalytics | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [days, setDays] = useState(30);
  const [queryFilter, setQueryFilter] = useState<"all" | "hits" | "misses">("all");

  const load = useCallback(() => {
    if (!isAdmin) return;
    void fetchCoachingAnalytics(days)
      .then((data) => {
        setAnalytics(data);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Could not load coaching analytics.");
      });
  }, [isAdmin, days]);

  useEffect(() => { load(); }, [load]);
  useAutoRefresh(load, 60_000);

  if (auth.loading) {
    return (
      <PageSection>
        <SectionHeader eyebrow="Admin · Coaching KB" title="Loading…" body="Checking your linked account." />
      </PageSection>
    );
  }

  if (!auth.authenticated || !isAdmin) {
    return (
      <PageSection>
        <EmptyState
          title="This page is locked"
          body="Only the configured admin account can view coaching analytics."
        />
      </PageSection>
    );
  }

  const filteredQueries = (analytics?.recentQueries ?? []).filter((q) => {
    if (queryFilter === "hits") return !q.isMiss;
    if (queryFilter === "misses") return q.isMiss;
    return true;
  });

  const topMissMax = analytics?.topMissTerms[0]?.count ?? 1;
  const topEntryMax = analytics?.topMatchedEntries[0]?.count ?? 1;

  return (
    <PageStack>
      {/* Header */}
      <PageSection className="border-amber-500/15 bg-[radial-gradient(circle_at_top_left,rgba(245,158,11,0.12),transparent_22%),linear-gradient(135deg,rgba(14,10,4,0.98),rgba(4,12,9,0.98))]">
        <SectionHeader
          eyebrow="Admin · Coaching KB"
          title="Knowledge base analytics"
          body="See what players are asking, where the knowledge base is answering well, and where gaps exist."
          aside={
            <div className="flex flex-wrap items-center gap-2">
              <Link to="/admin">
                <Button variant="secondary">← Hub health</Button>
              </Link>
              <div className="flex items-center gap-1 rounded-xl border border-line bg-panel p-1">
                {DAY_OPTIONS.map((d) => (
                  <button
                    key={d}
                    type="button"
                    onClick={() => setDays(d)}
                    className={`rounded-lg px-3 py-1 text-[12px] transition-colors ${
                      days === d
                        ? "bg-white/10 text-white"
                        : "text-muted hover:text-white/70"
                    }`}
                  >
                    {d}d
                  </button>
                ))}
              </div>
              <Button variant="secondary" onClick={load}>Refresh</Button>
            </div>
          }
        />

        {error && (
          <div className="mt-2 rounded-xl border border-amber-500/20 bg-amber-500/5 px-4 py-3 text-[13px] text-amber-300">
            {error}
          </div>
        )}

        {/* Stats */}
        <Grid className="mt-4 grid-cols-2 md:grid-cols-4">
          <StatCard
            label="Total queries"
            value={analytics ? analytics.totalQueries.toLocaleString() : "—"}
            detail={`Last ${days} days`}
            accent="mint"
          />
          <StatCard
            label="Hit rate"
            value={analytics ? pct(analytics.hitRatePct) : "—"}
            detail={`${analytics?.hitCount.toLocaleString() ?? "—"} hits · ${analytics?.missCount.toLocaleString() ?? "—"} misses`}
            accent={analytics && analytics.hitRatePct >= 60 ? "mint" : "gold"}
          />
          <StatCard
            label="Avg items / hit"
            value={analytics ? analytics.avgItemsPerHit.toFixed(1) : "—"}
            detail="Across queries with ≥1 result"
            accent="cyan"
          />
          <StatCard
            label="Zero-result queries"
            value={analytics ? analytics.missCount.toLocaleString() : "—"}
            detail="Questions with no KB match"
            accent={analytics && analytics.missCount > 0 ? "gold" : "mint"}
          />
        </Grid>

        {/* Hit/miss bar */}
        {analytics && (
          <div className="mt-4">
            <div className="mb-1.5 flex items-center justify-between text-[11px] text-muted">
              <span className="flex items-center gap-1.5">
                <span className="inline-block size-2 rounded-full bg-mint/60" /> hits ({pct(analytics.hitRatePct)})
              </span>
              <span className="flex items-center gap-1.5">
                misses ({pct(100 - analytics.hitRatePct)})
                <span className="inline-block size-2 rounded-full bg-amber-400/60" />
              </span>
            </div>
            <HitMissBar hitRatePct={analytics.hitRatePct} />
          </div>
        )}
      </PageSection>

      {/* Main content */}
      {analytics && (
        <>
          <PageSection>
            <div className="grid gap-5 md:grid-cols-2">

              {/* KB gaps — unmatched question terms */}
              <Card className="p-5">
                <div className="mb-3 text-[11px] uppercase tracking-[0.1em] text-amber-400/80">
                  KB gaps — top terms from zero-result queries
                </div>
                {analytics.topMissTerms.length === 0 ? (
                  <p className="text-[13px] text-muted">No miss queries in this period.</p>
                ) : (
                  <div className="flex flex-wrap gap-1.5">
                    {analytics.topMissTerms.map((t) => (
                      <TermPill key={t.term} term={t.term} count={t.count} max={topMissMax} />
                    ))}
                  </div>
                )}
                {analytics.topMissTerms.length > 0 && (
                  <p className="mt-3 text-[11px] text-muted">
                    These words appeared in questions that got no KB results. Consider adding entries covering these topics.
                  </p>
                )}
              </Card>

              {/* Top queried scenarios */}
              <Card className="p-5">
                <div className="mb-3 text-[11px] uppercase tracking-[0.1em] text-cyan/80">
                  Top queried scenarios
                </div>
                {analytics.topScenarios.length === 0 ? (
                  <p className="text-[13px] text-muted">No queries in this period.</p>
                ) : (
                  <div className="flex flex-col gap-1.5">
                    {analytics.topScenarios.map((s) => {
                      const maxCount = analytics.topScenarios[0]?.count ?? 1;
                      const barW = Math.max(4, (s.count / maxCount) * 100);
                      return (
                        <div key={s.scenarioName} className="flex items-center gap-2 text-[12px]">
                          <div className="w-[140px] shrink-0 truncate text-white/75" title={s.scenarioName}>
                            {s.scenarioName || <span className="text-muted italic">unknown</span>}
                          </div>
                          <div className="flex flex-1 items-center gap-2">
                            <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-white/5">
                              <div
                                className="h-full rounded-full bg-cyan/50"
                                style={{ width: `${barW}%` }}
                              />
                            </div>
                            <span className="w-8 shrink-0 text-right tabular-nums text-muted">{s.count}</span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </Card>

              {/* Top matched KB entries */}
              <Card className="p-5 md:col-span-2">
                <div className="mb-3 text-[11px] uppercase tracking-[0.1em] text-mint/80">
                  Most-returned KB entries
                </div>
                {analytics.topMatchedEntries.length === 0 ? (
                  <p className="text-[13px] text-muted">No KB hits in this period.</p>
                ) : (
                  <div className="flex flex-wrap gap-1.5">
                    {analytics.topMatchedEntries.map((e) => (
                      <EntryPill key={e.entryId} entryId={e.entryId} count={e.count} max={topEntryMax} />
                    ))}
                  </div>
                )}
                <p className="mt-3 text-[11px] text-muted">
                  Entries returned the most often — high counts mean these are broadly relevant and frequently matched.
                </p>
              </Card>
            </div>
          </PageSection>

          {/* Recent query log */}
          <PageSection>
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <div className="text-[11px] uppercase tracking-[0.1em] text-muted-2">
                Recent queries ({filteredQueries.length})
              </div>
              <div className="flex items-center gap-1 rounded-xl border border-line bg-panel p-1">
                {(["all", "hits", "misses"] as const).map((f) => (
                  <button
                    key={f}
                    type="button"
                    onClick={() => setQueryFilter(f)}
                    className={`rounded-lg px-3 py-1 text-[11px] capitalize transition-colors ${
                      queryFilter === f
                        ? "bg-white/10 text-white"
                        : "text-muted hover:text-white/70"
                    }`}
                  >
                    {f}
                  </button>
                ))}
              </div>
            </div>

            <Card className="overflow-hidden">
              {filteredQueries.length === 0 ? (
                <div className="px-4 py-8 text-center text-[13px] text-muted">
                  No queries to show.
                </div>
              ) : (
                <ScrollArea className="max-h-[560px]">
                  {filteredQueries.map((row) => (
                    <QueryRow key={row.id} row={row} />
                  ))}
                </ScrollArea>
              )}
            </Card>
          </PageSection>
        </>
      )}

      {!analytics && !error && (
        <PageSection>
          <div className="text-[13px] text-muted">Loading analytics…</div>
        </PageSection>
      )}
    </PageStack>
  );
}
