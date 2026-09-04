import { useCallback, useEffect, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, NavLink } from "react-router-dom";
import { PlayerLookup } from "../components/PlayerLookup";
import type { GetOverviewResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { StatCard } from "../components/StatCard";
import { VerificationBadge } from "../components/VerificationBadge";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Skeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useCountUp } from "../hooks/useCountUp";
import { fetchOverview, formatDurationMs, formatRelativeTime, slugifyScenarioName } from "../lib/api";

const PAGE_SIZE = 15;

function AnimatedStatCard({
  label,
  target,
  detail,
  accent,
  suffix = "",
}: {
  label: string;
  target: number;
  detail: string;
  accent?: "cyan" | "gold" | "violet";
  suffix?: string;
}) {
  const value = useCountUp(target);
  return (
    <StatCard
      label={label}
      value={target ? `${value.toLocaleString()}${suffix}` : "—"}
      detail={detail}
      accent={accent}
    />
  );
}


export function HomePage() {
  const [overview, setOverview] = useState<GetOverviewResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [runsVisible, setRunsVisible] = useState(PAGE_SIZE);

  useEffect(() => {
    let cancelled = false;
    void fetchOverview()
      .then((next) => {
        if (!cancelled) {
          setOverview(next);
          setError(null);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load hub overview.");
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const doRefresh = useCallback(() => {
    setError(null);
    void fetchOverview()
      .then((next) => {
        setOverview(next);
        setError(null);
      })
      .catch(() => setError("Community activity is temporarily unavailable."));
  }, []);
  useAutoRefresh(doRefresh, 30_000);

  const visibleRuns = overview?.recentRuns.slice(0, runsVisible) ?? [];
  const hasMoreRuns = (overview?.recentRuns.length ?? 0) > runsVisible;

  if (!overview) {
    return <PageStack>
      <Helmet><title>Overview · AimMod Hub</title></Helmet>
      <PageSection>
        <h1 className="text-3xl font-semibold">Overview</h1>
        <p className="mt-2 text-sm text-muted">Recent plays, players, and practice history.</p>
        <div className="mt-5 flex flex-wrap gap-2"><Button to="/osu/community">Browse osu! plays</Button><Button to="/community">KovaaK's community</Button></div>
      </PageSection>
      {error ? <EmptyState title="Community activity is unavailable" body="Please try again in a moment."><Button onClick={doRefresh}>Try again</Button></EmptyState>
        : <div role="status" aria-label="Loading community activity"><p className="mb-4 text-muted">Loading community activity...</p><Skeleton className="h-48" /></div>}
      <Link to="/app" className="block max-w-lg"><img src="/images/aimmod-brand-card.png" width="512" height="256" alt="AimMod" className="w-full" /><span className="block py-3 text-sm text-cyan">Download AimMod</span></Link>
    </PageStack>;
  }

  return (
    <PageStack>
      <Helmet>
        <title>AimMod Hub · Shared practice intelligence</title>
        <meta name="description" content="AimMod analysis and coaching for osu! and KovaaK's, plus shared KovaaK's practice data." />
      </Helmet>
      <PageSection>
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div><p className="text-sm text-muted">Community activity</p><h1 className="mt-1 text-3xl font-semibold">Overview</h1><p className="mt-2 text-sm text-muted">Recent plays, players, and practice history.</p></div>
          <Button to="/osu/community">Browse osu! plays</Button>
        </div>
      </PageSection>

      <Grid className="grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
        <AnimatedStatCard
          label="Runs"
          target={overview ? Number(overview.totalRuns) : 0}
          detail={error ? "Overview is unavailable right now" : "Sessions available to study"}
        />
        <AnimatedStatCard
          label="Scenarios"
          target={overview ? Number(overview.totalScenarios) : 0}
          detail="Scenarios with practice history"
          accent="cyan"
        />
        <AnimatedStatCard
          label="Players"
          target={overview ? Number(overview.totalPlayers) : 0}
          detail="Profiles with saved history"
          accent="gold"
        />
      </Grid>

      <PageSection className="overflow-visible">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-4">
          <div className="shrink-0">
            <div className="text-[11px] uppercase tracking-normal text-cyan">KovaaK's lookup</div>
            <div className="mt-0.5 text-[13px] text-text">Find any player</div>
          </div>
          <div className="flex-1">
            <PlayerLookup />
          </div>
        </div>
      </PageSection>

      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_280px]">
        <nav aria-label="Browse community" className="grid content-start divide-y divide-line border-y border-line">
          <Link to="/replays" className="flex items-center justify-between py-4"><span><strong className="block">Replay library</strong><span className="text-sm text-muted">Mouse paths and recorded runs</span></span><span aria-hidden="true">→</span></Link>
          <Link to="/community" className="flex items-center justify-between py-4"><span><strong className="block">Players & scenarios</strong><span className="text-sm text-muted">Public profiles and practice history</span></span><span aria-hidden="true">→</span></Link>
          <Link to="/leaderboard" className="flex items-center justify-between py-4"><span><strong className="block">Leaderboard</strong><span className="text-sm text-muted">Records and top scores</span></span><span aria-hidden="true">→</span></Link>
        </nav>
        <Link to="/app" className="block self-start overflow-hidden rounded-md border border-line bg-panel">
          <img src="/images/aimmod-brand-card.png" alt="AimMod" width="512" height="256" className="block w-full" />
          <span className="flex justify-between p-3 text-sm"><strong>Get AimMod</strong><span aria-hidden="true">→</span></span>
        </Link>
      </div>

      <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Top scenarios"
            title="Popular scenarios"
            body="These scenarios already have enough history to be useful comparison pages."
            aside={<NavLink to="/community" className="text-cyan transition-colors hover:underline">Browse all →</NavLink>}
          />
          {overview?.topScenarios.length ? (
            <ScrollArea className="max-h-[min(54vh,720px)] pr-2">
              <div className="grid gap-3">
                {overview.topScenarios.slice(0, 6).map((scenario) => (
                  <Link
                    key={scenario.scenarioSlug}
                    to={`/scenarios/${scenario.scenarioSlug}`}
                    className="rounded-md border border-line bg-white/2 p-4 transition-colors hover:border-cyan/25 hover:bg-white/3"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div>
                        <strong className="block text-text">{scenario.scenarioName}</strong>
                        <div className="mt-1.5"><ScenarioTypeBadge type={scenario.scenarioType} /></div>
                      </div>
                      <div className="text-right shrink-0">
                        <span className="block tabular-nums text-sm text-mint">{scenario.runCount.toLocaleString()}</span>
                        <span className="text-[10px] text-muted-2 uppercase tracking-wider">runs</span>
                      </div>
                    </div>
                  </Link>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState
              title="No scenario history yet"
              body={error || "Scenario pages will appear here once there is enough history to compare."}
            />
          )}
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Active players"
            title="Profiles with real history"
            body="These players have the most practice history right now, which makes them the best starting point for study and comparison."
            aside={<NavLink to="/community" className="text-cyan transition-colors hover:underline">Browse all →</NavLink>}
          />
          {overview?.activeProfiles.length ? (
            <ScrollArea className="max-h-[min(54vh,720px)] pr-2">
              <div className="grid gap-3">
                {overview.activeProfiles.slice(0, 6).map((profile) => (
                  <Link
                    key={profile.userHandle}
                    to={`/profiles/${profile.userHandle}`}
                    className="rounded-md border border-line bg-white/2 p-4 transition-colors hover:border-cyan/25 hover:bg-white/3"
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <strong className="block text-text truncate">
                            {profile.userDisplayName || profile.userHandle}
                          </strong>
                          <VerificationBadge verified={Boolean(profile.isVerified)} />
                        </div>
                        <p className="mt-1 text-sm text-muted">@{profile.userHandle}</p>
                      </div>
                      <div className="text-right shrink-0">
                        <span className="block tabular-nums text-sm text-cyan">{profile.runCount.toLocaleString()}</span>
                        <span className="text-[10px] text-muted-2 uppercase tracking-wider">runs</span>
                      </div>
                    </div>
                    <div className="mt-3 flex items-center gap-2">
                      <span className="text-[12px] text-muted">{profile.scenarioCount.toLocaleString()} scenarios</span>
                      <ScenarioTypeBadge type={profile.primaryScenarioType} />
                    </div>
                  </Link>
                ))}
              </div>
            </ScrollArea>
          ) : (
            <EmptyState
              title="No player history yet"
              body={error || "Player profiles will appear here once there is saved practice history to show."}
            />
          )}
        </PageSection>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Recent completed runs"
          title="Recent runs"
          body="The latest runs players have completed."
          aside={<NavLink to="/leaderboard" className="text-cyan transition-colors hover:underline">View leaderboard →</NavLink>}
        />
        {overview?.recentRuns.length ? (
          <>
            <ScrollArea className="max-h-[min(56vh,740px)] overflow-auto rounded-md border border-line bg-white/2">
              <table className="min-w-[760px] w-full text-left text-sm">
                <thead className="sticky top-0 z-10 border-b border-line bg-panel-strong text-[11px] uppercase tracking-normal text-muted">
                  <tr>
                    <th className="px-4 py-3">Scenario</th>
                    <th className="px-4 py-3">Player</th>
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
                      <td className="px-3 py-2.5 text-text max-w-[200px] truncate md:px-4 md:py-3">
                        <Link
                          className="hover:text-cyan transition-colors"
                          to={`/scenarios/${slugifyScenarioName(run.scenarioName)}`}
                        >
                          {run.scenarioName}
                        </Link>
                      </td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/profiles/${run.userHandle || run.userDisplayName}`}
                        >
                          {run.userDisplayName || run.userHandle}
                        </Link>
                      </td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">{Math.round(run.score).toLocaleString()}</td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">{run.accuracy.toFixed(1)}%</td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">{formatDurationMs(run.durationMs)}</td>
                      <td className="px-3 py-2.5 text-muted md:px-4 md:py-3">{formatRelativeTime(run.playedAtIso)}</td>
                      <td className="px-3 py-2.5 text-text md:px-4 md:py-3">
                        <Link
                          className="text-cyan underline underline-offset-3"
                          to={`/runs/${run.runId || run.sessionId}`}
                        >
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
                className="mt-3 w-full rounded-md border border-line py-2.5 text-sm text-muted transition-colors hover:border-cyan/30 hover:text-text"
              >
                Load {Math.min(PAGE_SIZE, (overview?.recentRuns.length ?? 0) - runsVisible)} more runs
              </button>
            )}
          </>
        ) : (
          <EmptyState
            title="No runs yet"
            body={error || "Recent runs will appear here once there is practice history to show."}
          />
        )}
      </PageSection>

      <PageSection className="relative overflow-hidden border-mint/20 bg-[radial-gradient(circle_at_60%_0%,rgba(121,201,151,0.1),transparent_40%),linear-gradient(180deg,rgba(6,18,12,0.98),rgba(4,12,9,0.97))]">
        <div className="flex flex-col gap-5 md:flex-row md:items-center md:justify-between">
          <div>
            <div className="mb-2 text-[11px] uppercase tracking-normal text-mint">AimMod desktop</div>
            <h2 className="mb-2 max-w-[22ch] text-[clamp(18px,2.8vw,28px)] font-medium leading-[1.1] tracking-normal">
              This is everyone else's data.<br />Want to see yours?
            </h2>
            <p className="max-w-125 text-[13px] leading-relaxed text-muted">
              Choose the native osu! client for beatmaps, PP targets, replays, and practice maps, or the KovaaK's companion for live runs and mouse-path analysis.
            </p>
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            <Button to="/app" variant="primary">Choose a product</Button>
            <Button to="/app/osu">Download for osu!</Button>
          </div>
        </div>
      </PageSection>
    </PageStack>
  );
}
