import { useCallback, useEffect, useMemo, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, NavLink } from "react-router-dom";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { SortableTh } from "../components/ui/SortableTh";
import { TypeFilterBar } from "../components/ui/TypeFilterBar";
import { Grid, PageStack } from "../components/ui/Stack";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { VerificationBadge } from "../components/VerificationBadge";
import { fetchOverview, formatDurationMs, formatRelativeTime, slugifyScenarioName } from "../lib/api";
import { useAutoRefresh } from "../hooks/useAutoRefresh";

const PAGE_SIZE = 15;
type ScenarioSortField = "runCount" | "name";

export function CommunityPage() {
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [scenarioTypeFilter, setScenarioTypeFilter] = useState<string | null>(null);
  const [scenarioSortField, setScenarioSortField] = useState<ScenarioSortField>("runCount");
  const [scenarioSortDir, setScenarioSortDir] = useState<"asc" | "desc">("desc");
  const [runsVisible, setRunsVisible] = useState(PAGE_SIZE);

  const load = useCallback((reset: boolean) => {
    void fetchOverview()
      .then((next) => {
        if (reset) setRunsVisible(PAGE_SIZE);
        setOverview(next);
        setError(null);
      })
      .catch((err) => {
        if (reset) setError(err instanceof Error ? err.message : "Could not load community data.");
      });
  }, []);

  useEffect(() => {
    load(true);
  }, [load]);

  useAutoRefresh(() => load(false), 30_000);

  function handleScenarioSort(field: string) {
    const f = field as ScenarioSortField;
    if (f === scenarioSortField) {
      setScenarioSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setScenarioSortField(f);
      setScenarioSortDir(f === "name" ? "asc" : "desc");
    }
  }

  const scenarioTypes = useMemo(() => {
    if (!overview) return [];
    const types = new Set<string>();
    for (const s of overview.topScenarios) {
      if (s.scenarioType?.trim() && s.scenarioType !== "Unknown") types.add(s.scenarioType);
    }
    return [...types];
  }, [overview]);

  const filteredSortedScenarios = useMemo(() => {
    if (!overview) return [];
    const filtered = scenarioTypeFilter
      ? overview.topScenarios.filter((s) => s.scenarioType === scenarioTypeFilter)
      : overview.topScenarios;
    return [...filtered].sort((a, b) => {
      let diff = 0;
      if (scenarioSortField === "runCount") diff = Number(a.runCount) - Number(b.runCount);
      else diff = a.scenarioName.localeCompare(b.scenarioName);
      return scenarioSortDir === "asc" ? diff : -diff;
    });
  }, [overview, scenarioTypeFilter, scenarioSortField, scenarioSortDir]);

  const visibleRuns = overview?.recentRuns.slice(0, runsVisible) ?? [];
  const hasMoreRuns = (overview?.recentRuns.length ?? 0) > runsVisible;

  return (
    <PageStack>
      <Helmet>
        <title>Community · AimMod Hub</title>
        <meta name="description" content="Explore the AimMod Hub community — top scenarios, active players, and recent runs." />
      </Helmet>
      <PageSection>
        <SectionHeader
          eyebrow="Community"
          title="Where the strongest comparisons already are"
          body="This is where you can find the scenarios, players, and recent runs that are already worth studying."
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
          <StatCard label="Runs" value={overview ? overview.totalRuns.toLocaleString() : "—"} detail="Runs available to compare" />
          <StatCard label="Scenarios" value={overview ? overview.totalScenarios.toLocaleString() : "—"} detail="Scenario pages with saved history" accent="cyan" />
          <StatCard label="Players" value={overview ? overview.totalPlayers.toLocaleString() : "—"} detail="Players with public practice history" accent="gold" />
        </Grid>
      </PageSection>

      <Grid className="grid-cols-[1.15fr_0.85fr] items-start max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Scenario watchlist"
            title="Best pages to open next"
            body="These scenarios already have enough history to start showing useful score bands and run patterns."
            aside={<NavLink to="/leaderboard" className="text-cyan transition-colors hover:underline">View leaderboard →</NavLink>}
          />
          {overview?.topScenarios.length ? (
            <>
              {scenarioTypes.length > 1 && (
                <TypeFilterBar types={scenarioTypes} active={scenarioTypeFilter} onChange={setScenarioTypeFilter} />
              )}
              <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
                <table className="min-w-full text-left text-sm">
                  <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                    <tr>
                      <SortableTh label="Scenario" field="name" sortField={scenarioSortField} sortDir={scenarioSortDir} onSort={handleScenarioSort} />
                      <th className="px-4 py-3">Type</th>
                      <SortableTh label="Runs" field="runCount" sortField={scenarioSortField} sortDir={scenarioSortDir} onSort={handleScenarioSort} />
                      <th className="px-4 py-3">Open</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredSortedScenarios.map((scenario) => (
                      <tr key={scenario.scenarioSlug} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                        <td className="px-4 py-3 text-text">
                          <Link className="hover:text-cyan transition-colors" to={`/scenarios/${scenario.scenarioSlug}`}>
                            {scenario.scenarioName}
                          </Link>
                        </td>
                        <td className="px-4 py-3"><ScenarioTypeBadge type={scenario.scenarioType} /></td>
                        <td className="px-4 py-3 text-text">{scenario.runCount.toLocaleString()}</td>
                        <td className="px-4 py-3 text-text">
                          <Link className="text-cyan underline underline-offset-3" to={`/scenarios/${scenario.scenarioSlug}`}>
                            Open
                          </Link>
                        </td>
                      </tr>
                    ))}
                    {filteredSortedScenarios.length === 0 && (
                      <tr>
                        <td colSpan={4} className="px-4 py-6 text-center text-sm text-muted">No scenarios match this filter.</td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </ScrollArea>
            </>
          ) : (
            <EmptyState title="No scenario pages yet" body={error || "Scenario watchlists will appear here once there is enough history to compare."} />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Player watchlist"
            title="Most active uploaders"
            body="These are the best profiles to inspect first when you want examples and consistent practice history."
          />
          {overview?.activeProfiles.length ? (
            <ScrollArea className="max-h-[min(64vh,820px)] pr-2">
              <div className="grid gap-3">
              {overview.activeProfiles.map((profile) => (
                <Link
                  key={profile.userHandle}
                  to={`/profiles/${profile.userHandle}`}
                    className="rounded-[18px] border border-line bg-white/2 p-[18px] transition-colors hover:border-cyan/30 hover:bg-white/3"
                >
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <strong className="block text-text truncate">{profile.userDisplayName || profile.userHandle}</strong>
                        <VerificationBadge verified={Boolean(profile.isVerified)} />
                      </div>
                      <p className="mt-1 text-sm text-muted">@{profile.userHandle}</p>
                    </div>
                    <span className="text-sm text-mint shrink-0">{profile.runCount.toLocaleString()} runs</span>
                  </div>
                  <div className="mt-3 flex items-center gap-2">
                    <span className="text-sm text-muted">{profile.scenarioCount.toLocaleString()} scenarios</span>
                    <ScenarioTypeBadge type={profile.primaryScenarioType} />
                  </div>
                </Link>
              ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState title="No active profiles yet" body={error || "Active profiles will appear here once there is enough practice history to show."} />
          )}
        </PageSection>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Recent completed runs"
          title="What players have actually just finished"
          body="The latest runs players have completed."
          aside={<NavLink to="/replays" className="text-cyan transition-colors hover:underline">Browse replays →</NavLink>}
        />
        {overview?.recentRuns.length ? (
          <>
            <ScrollArea className="max-h-[min(62vh,780px)] overflow-auto rounded-[18px] border border-line bg-white/2">
              <table className="min-w-full text-left text-sm">
                <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                  <tr>
                    <th className="px-4 py-3">Player</th>
                    <th className="px-4 py-3">Scenario</th>
                    <th className="px-4 py-3">Score</th>
                    <th className="px-4 py-3">Acc</th>
                    <th className="px-4 py-3">Duration</th>
                    <th className="px-4 py-3">When</th>
                    <th className="px-4 py-3">Run</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleRuns.map((run) => (
                    <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                      <td className="px-4 py-3 text-text">
                        <Link className="text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle || run.userDisplayName}`}>
                          {run.userDisplayName || run.userHandle}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-text">
                        <Link className="hover:text-cyan transition-colors" to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}>
                          {run.scenarioName}
                        </Link>
                      </td>
                      <td className="px-4 py-3 text-text">{Math.round(run.score).toLocaleString()}</td>
                      <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-text">{formatDurationMs(run.durationMs)}</td>
                      <td className="px-4 py-3 text-muted" title={new Date(run.playedAtIso).toLocaleString()}>{formatRelativeTime(run.playedAtIso)}</td>
                      <td className="px-4 py-3 text-text">
                        <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.runId || run.sessionId}`}>
                          Open
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </ScrollArea>
            {hasMoreRuns && (
              <button
                onClick={() => setRunsVisible((n) => n + PAGE_SIZE)}
                className="mt-3 w-full rounded-[14px] border border-line py-2.5 text-sm text-muted transition-colors hover:border-cyan/30 hover:text-text"
              >
                Load {Math.min(PAGE_SIZE, (overview?.recentRuns.length ?? 0) - runsVisible)} more runs
              </button>
            )}
          </>
        ) : (
          <EmptyState title="No recent runs yet" body={error || "Recent runs will appear here once there is practice history to show."} />
        )}
      </PageSection>
    </PageStack>
  );
}
